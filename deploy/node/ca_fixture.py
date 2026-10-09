"""Private CA and leaf fixtures for the node Core-trust tests.

The server side of the reported incident sends the leaf certificate alone, with
no intermediate, no root and no AIA extension. These helpers reproduce that with
a locally generated CA and a leaf it signed, using only openssl so the tests have
no third-party dependency. The config-file form is deliberate: it works on both
the OpenSSL 1.0.2 that some hosts ship and OpenSSL 3.x.
"""
import shutil
import subprocess
from pathlib import Path

# Resolved once at import, before any test mutates the process PATH.
OPENSSL = shutil.which("openssl")

CA_CONFIG = """[req]
distinguished_name = dn
x509_extensions = v3_ca
prompt = no
[dn]
CN = OAC test CA
[v3_ca]
basicConstraints = critical,CA:TRUE
keyUsage = critical,keyCertSign,cRLSign
"""

LEAF_CONFIG = """[req]
distinguished_name = dn
prompt = no
[dn]
CN = localhost
[v3_leaf]
basicConstraints = critical,CA:FALSE
keyUsage = critical,digitalSignature,keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt
[alt]
IP.1 = 127.0.0.1
DNS.1 = localhost
"""


def _run(*arguments):
    subprocess.run([OPENSSL, *arguments], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def private_ca(directory, name="core-ca"):
    """Generate a private CA; returns (certificate path, key path)."""
    directory = Path(directory)
    directory.mkdir(parents=True, exist_ok=True)
    config = directory / (name + ".cnf")
    config.write_text(CA_CONFIG)
    key, certificate = directory / (name + ".key"), directory / (name + ".pem")
    _run("req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", str(key), "-out", str(certificate),
         "-days", "1", "-config", str(config))
    return certificate, key


def leaf_signed_by(ca_certificate, ca_key, directory, name="leaf"):
    """Generate a server leaf signed by the CA; returns (certificate path, key path)."""
    directory = Path(directory)
    config = directory / (name + ".cnf")
    config.write_text(LEAF_CONFIG)
    key, request, certificate = directory / (name + ".key"), directory / (name + ".csr"), directory / (name + ".pem")
    _run("req", "-newkey", "rsa:2048", "-nodes", "-keyout", str(key), "-out", str(request), "-config", str(config))
    _run("x509", "-req", "-in", str(request), "-CA", str(ca_certificate), "-CAkey", str(ca_key), "-CAcreateserial",
         "-out", str(certificate), "-days", "1", "-extfile", str(config), "-extensions", "v3_leaf")
    return certificate, key
