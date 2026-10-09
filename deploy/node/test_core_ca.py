"""The node's Core trust: the system store plus an operator CA.

The fixture server presents the leaf certificate alone, with no intermediate,
no root and no AIA extension, exactly like the reported deployment. Each path
asserts both directions: with the operator CA it verifies, and without it the
download fails closed instead of being downgraded to a warning.
"""
import argparse
import contextlib
import hashlib
import http.server
import json
from pathlib import Path
import ssl
import sys
import tempfile
import threading
import unittest
import uuid

import ca_fixture
import distribution
import node_install as installer
import node_spec

METADATA = b"node bootstrap metadata fixture"
ARTIFACT = b"node artifact fixture"


@contextlib.contextmanager
def serve(handler, context):
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), handler)
    server.socket = context.wrap_socket(server.socket, server_side=True)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield server
    finally:
        server.shutdown()
        server.server_close()
        thread.join()


class Routes(http.server.BaseHTTPRequestHandler):
    routes = {}

    def do_GET(self):
        body = self.routes.get(self.path)
        if body is None:
            self.send_error(404)
            return
        self.send_response(200)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


def routed(routes):
    return type("Routed", (Routes,), {"routes": routes})


@unittest.skipUnless(sys.platform == "linux" and ca_fixture.OPENSSL, "openssl on Linux is required")
class CoreTrustTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        base = Path.home() / ".oac/tests"
        base.mkdir(parents=True, exist_ok=True)
        cls.temporary = tempfile.TemporaryDirectory(dir=base)
        cls.directory = Path(cls.temporary.name)
        cls.ca_cert, cls.ca_key = ca_fixture.private_ca(cls.directory)
        leaf_cert, leaf_key = ca_fixture.leaf_signed_by(cls.ca_cert, cls.ca_key, cls.directory)
        cls.tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        cls.tls.load_cert_chain(leaf_cert, leaf_key)

    @classmethod
    def tearDownClass(cls):
        cls.temporary.cleanup()

    def setUp(self):
        # Never leak a trust decision into another test.
        self.addCleanup(installer.set_core_ca, None)

    def configuration(self, installation_id, origin):
        manifest = {"source_commit": "a" * 40, "images": {"runtime": "sha256:" + "b" * 64},
                    "image_manifest_digests": {"runtime": "sha256:" + "c" * 64},
                    "runtime_ref": "oac-runtime@sha256:" + "c" * 64,
                    "microsandbox": {"runtime_sha256": "d" * 64, "firmware_sha256": "e" * 64}}
        spec = {"resources": {"cpus": 2, "memory_mib": 2048}, "runtime": node_spec.release(manifest)}
        return {"installation_id": installation_id, "provider": "docker", "core_url": origin, "generation": 1,
                "specification": spec, "max_active": 2, "max_retained": 8,
                "specification_digest": node_spec.digest("docker", spec)}

    def test_installer_reads_metadata_and_configuration_with_the_operator_ca(self):
        installation_id = str(uuid.uuid4())
        with serve(Routes, self.tls) as server:
            origin = "https://127.0.0.1:" + str(server.server_port)
            Routes.routes = {"/node-install/SHA256SUMS": METADATA,
                             "/api/v1/sandbox-node/configuration": json.dumps(self.configuration(installation_id, origin)).encode()}
            args = argparse.Namespace(core_url=origin, installation_id=installation_id, provider=None)
            installer.set_core_ca(self.ca_cert)
            self.assertEqual(installer.fetch(origin, "SHA256SUMS").read(), METADATA)
            configuration = node_spec.fetch(args, "token", None, installer.open_request, allow_enrollment=True)
            self.assertEqual(configuration["installation_id"], installation_id)
        # Without the CA the same server is an unknown authority and the installer
        # fails with its existing metadata message, never a skipped verification.
        with serve(Routes, self.tls) as server:
            origin = "https://127.0.0.1:" + str(server.server_port)
            Routes.routes = {"/node-install/SHA256SUMS": METADATA}
            installer.set_core_ca(None)
            with self.assertRaises(installer.RuntimeDownloadError) as caught:
                installer.fetch(origin, "SHA256SUMS")
            self.assertIn("Cannot download node metadata", str(caught.exception))
        Routes.routes = {}

    def test_artifact_download_uses_the_operator_ca(self):
        name, filename = "native/bin/node", "node-" + "a" * 40
        manifest = {"source_commit": "a" * 40,
                    "artifacts": {name: {"filename": filename, "size": len(ARTIFACT),
                                         "sha256": hashlib.sha256(ARTIFACT).hexdigest()}}}
        with serve(Routes, self.tls) as server:
            origin = "https://127.0.0.1:" + str(server.server_port)
            Routes.routes = {"/artifacts/" + filename: ARTIFACT}
            manifest["artifact_base_url"] = origin + "/artifacts"
            installer.set_core_ca(self.ca_cert)
            target = self.directory / "artifact-node"
            distribution.obtain_artifact(manifest, name, target)
            self.assertEqual(target.read_bytes(), ARTIFACT)
            target.unlink()
        with serve(Routes, self.tls) as server:
            manifest["artifact_base_url"] = "https://127.0.0.1:" + str(server.server_port) + "/artifacts"
            installer.set_core_ca(None)
            with self.assertRaises(distribution.ArtifactError):
                distribution.obtain_artifact(manifest, name, self.directory / "artifact-untrusted")
        Routes.routes = {}

    def test_ssl_context_appends_the_ca_to_the_system_roots(self):
        system = ssl.create_default_context().cert_store_stats()["x509"]
        self.assertGreater(system, 0)
        context = installer.core_ssl_context(self.ca_cert)
        self.assertEqual(context.cert_store_stats()["x509"], system + 1)
        # The default context still holds only the system roots.
        self.assertEqual(installer.core_ssl_context(None).cert_store_stats()["x509"], system)

    def test_read_core_ca_rejects_missing_relative_and_unparseable_files(self):
        with self.assertRaises(installer.InstallError):
            installer.read_core_ca(str(self.directory / "missing.pem"))
        with self.assertRaises(installer.InstallError):
            installer.read_core_ca("relative-ca.pem")
        garbage = self.directory / "garbage.pem"
        garbage.write_bytes(b"not a certificate\n")
        with self.assertRaises(installer.InstallError):
            installer.read_core_ca(str(garbage))
        link = self.directory / "link.pem"
        link.symlink_to(self.ca_cert)
        with self.assertRaises(installer.InstallError):
            installer.read_core_ca(str(link))
        path, data, digest = installer.read_core_ca(str(self.ca_cert))
        self.assertEqual((path, digest), (self.ca_cert, hashlib.sha256(data).hexdigest()))

    def test_materialized_ca_is_private_and_must_match_its_identity(self):
        root = self.directory / "node-root"
        root.mkdir()
        data = self.ca_cert.read_bytes()
        args = argparse.Namespace(core_ca=str(root / installer.CORE_CA_NAME), core_ca_bytes=data,
                                  core_ca_digest=hashlib.sha256(data).hexdigest())
        installer.materialize_core_ca(root, args)
        target = root / installer.CORE_CA_NAME
        self.assertEqual(target.read_bytes(), data)
        self.assertEqual(target.stat().st_mode & 0o777, 0o600)
        identity_dir = root / "state/node"
        identity_dir.mkdir(parents=True)
        (identity_dir / "identity.json").write_text(json.dumps(
            {"identity": {"installation_id": "fixture"}, "core_ca": str(target), "core_ca_sha256": args.core_ca_digest}))
        self.assertEqual(installer.retained_core_ca(root), target)
        target.write_bytes(b"different certificate")
        with self.assertRaises(installer.InstallError):
            installer.retained_core_ca(root)


if __name__ == "__main__":
    unittest.main()
