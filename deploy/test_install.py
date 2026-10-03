"""install.sh writes .env and starts Compose without host Python."""
import os
from pathlib import Path
import stat
import subprocess
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parents[1]
INSTALL = ROOT / "deploy/install.sh"
CHECKSUM = "a" * 64


class InstallScriptTests(unittest.TestCase):
    def install(self, root, *args, compose_up=0, check_config=0, init=0, checksum=CHECKSUM, listed=None):
        bin_dir = root / "bin"
        bin_dir.mkdir(exist_ok=True)
        log = root / "docker.log"
        if listed is None:
            listed = {"compose.yaml": CHECKSUM, "ports.yaml": CHECKSUM}
        checksums = "".join(f"{digest}  {name}\\n" for name, digest in listed.items())
        self.write_executable(bin_dir / "docker", textwrap.dedent(f"""\
            #!/bin/sh
            printf '%s\\n' "$*" >> {log}
            if [ "$1" = compose ] && [ "$2" = version ]; then printf 'v2.29.1\\n'; exit 0; fi
            if [ "$1" = compose ] && [ "$2" = cp ]; then printf '#!/bin/sh\\n' > ./oac; exit 0; fi
            if [ "$1" = compose ] && [ "$2" = run ]; then
              case "$*" in
                *check-config*)
                  if [ {check_config} -ne 0 ]; then
                    printf '%s\\n' 'OAC_PUBLIC_URL must be a canonical HTTPS origin without path, credentials, query or fragment, such as https://core.example; plain HTTP is accepted only for a loopback host' >&2
                  fi
                  exit {check_config}
                  ;;
              esac
              exit {init}
            fi
            if [ "$1" = compose ] && [ "$2" = up ]; then exit {compose_up}; fi
            exit 0
            """))
        self.write_executable(bin_dir / "curl", textwrap.dedent(f"""\
            #!/bin/sh
            output=""
            while [ $# -gt 0 ]; do
              if [ "$1" = --output ]; then output="$2"; shift 2; continue; fi
              shift
            done
            case "$output" in
              *compose-sha256sums.txt) printf '%b' '{checksums}' > "$output" ;;
              *) printf 'fixture\\n' > "$output" ;;
            esac
            """))
        self.write_executable(bin_dir / "sha256sum", f'#!/bin/sh\ncase "$1" in --*) echo "sha256sum: unrecognized option: $1" >&2; exit 1 ;; esac\nprintf \'%s  %s\\n\' {checksum} "$1"\n')
        self.write_executable(bin_dir / "ss", "#!/bin/sh\nexit 0\n")
        env = dict(os.environ, PATH=str(bin_dir) + os.pathsep + os.environ["PATH"], HOME=str(root))
        completed = subprocess.run(["bash", str(INSTALL), "--install-dir", str(root / "oac"), *args],
                                   env=env, capture_output=True, text=True)
        return completed, log.read_text() if log.exists() else ""

    def test_a_failed_first_start_stops_and_removes_the_directory(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            completed, recorded = self.install(root, "--host", "127.0.0.1", "--web-port", "59991",
                                               "--public-url", "https://core.example", compose_up=1)
            self.assertNotEqual(completed.returncode, 0, completed.stderr)
            self.assertFalse((root / "oac").exists(), "a failed first start must remove the directory")
            self.assertIn("compose pull", recorded)
            self.assertIn("compose up -d --wait", recorded)

    def test_initialization_and_configuration_check_run_before_the_stack_starts(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            completed, recorded = self.install(root, "--public-url", "https://core.example")
            self.assertEqual(completed.returncode, 0, completed.stderr)
            lines = recorded.splitlines()
            initialize = next(index for index, line in enumerate(lines) if line == "compose run --rm -T init")
            check = next(index for index, line in enumerate(lines) if line.endswith("core check-config"))
            up = next(index for index, line in enumerate(lines) if line == "compose up -d --wait")
            self.assertLess(initialize, check, recorded)
            self.assertLess(check, up, recorded)
            self.assertIn("--no-deps", lines[check])
            self.assertIn("--entrypoint /usr/local/bin/oac-core", lines[check])

    def test_a_rejected_public_url_fails_without_reporting_success(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            completed, recorded = self.install(root, "--public-url", "http://10.0.0.5:8080", check_config=1)
            self.assertNotEqual(completed.returncode, 0)
            self.assertIn("plain HTTP is accepted only for a loopback host", completed.stderr)
            self.assertNotIn("OpenAgentCore is running.", completed.stdout)
            self.assertNotIn("compose up -d --wait", recorded)
            self.assertFalse((root / "oac").exists(), "a rejected configuration must remove the directory")

    def test_the_insecure_origin_switch_keeps_the_install_succeeding(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            completed, _ = self.install(root, "--public-url", "http://10.0.0.5:8080", "--allow-insecure-origin")
            self.assertEqual(completed.returncode, 0, completed.stderr)
            self.assertIn("OpenAgentCore is running.", completed.stdout)
            self.assertIn("Core key:", completed.stdout)

    def test_a_failed_initialization_stops_before_the_stack_starts(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            completed, recorded = self.install(root, "--public-url", "https://core.example", init=1)
            self.assertNotEqual(completed.returncode, 0)
            self.assertNotIn("OpenAgentCore is running.", completed.stdout)
            self.assertNotIn("check-config", recorded)
            self.assertNotIn("compose up -d --wait", recorded)
            self.assertFalse((root / "oac").exists())

    def test_env_holds_only_the_installation_choices(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            completed, _ = self.install(root, "--public-url", "https://core.example")
            self.assertEqual(completed.returncode, 0, completed.stderr)
            env = dict(line.split("=", 1) for line in (root / "oac/.env").read_text().splitlines())
            self.assertEqual(env["COMPOSE_FILE"], "compose.yaml:ports.yaml")
            self.assertEqual(env["OAC_PUBLIC_URL"], "https://core.example")
            self.assertEqual(sorted(env), ["COMPOSE_FILE", "COMPOSE_PROJECT_NAME", "OAC_HOST", "OAC_INSTALL_DIR", "OAC_PUBLIC_URL", "OAC_WEB_PORT"])

    def test_a_checksum_mismatch_stops_before_the_stack_starts(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            completed, recorded = self.install(root, checksum="b" * 64)
            self.assertNotEqual(completed.returncode, 0)
            self.assertIn("compose.yaml: FAILED", completed.stderr)
            self.assertNotIn("OpenAgentCore is running.", completed.stdout)
            self.assertNotIn("compose up -d --wait", recorded)
            self.assertFalse((root / "oac").exists(), "a failed checksum must remove the directory")

    def test_a_missing_checksum_entry_stops_the_install(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            completed, recorded = self.install(root, listed={"compose.yaml": CHECKSUM})
            self.assertNotEqual(completed.returncode, 0)
            self.assertIn("No valid checksum for ports.yaml", completed.stderr)
            self.assertNotIn("compose up -d --wait", recorded)
            self.assertFalse((root / "oac").exists())

    def test_checksum_entries_for_other_release_files_are_ignored(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            listed = {"compose.yaml": CHECKSUM, "ports.yaml": CHECKSUM, "https.yaml": CHECKSUM}
            completed, _ = self.install(root, listed=listed)
            self.assertEqual(completed.returncode, 0, completed.stderr)
            self.assertIn("OpenAgentCore is running.", completed.stdout)

    def test_help_does_not_need_docker(self):
        help_text = subprocess.run(["bash", str(INSTALL), "--help"], capture_output=True, text=True, check=True)
        self.assertIn("--web-port", help_text.stdout)
        self.assertIn("--allow-insecure-origin", help_text.stdout)
        self.assertNotIn("--external-proxy", help_text.stdout)

    def test_allow_insecure_origin_is_written_only_when_requested(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            completed, _ = self.install(root, "--public-url", "http://10.0.0.5:8080", "--allow-insecure-origin")
            self.assertEqual(completed.returncode, 0, completed.stderr)
            env = dict(line.split("=", 1) for line in (root / "oac/.env").read_text().splitlines())
            self.assertEqual(env["OAC_PUBLIC_URL"], "http://10.0.0.5:8080")
            self.assertEqual(env["OAC_ALLOW_INSECURE_ORIGIN"], "1")
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            completed, _ = self.install(root)
            self.assertEqual(completed.returncode, 0, completed.stderr)
            env = dict(line.split("=", 1) for line in (root / "oac/.env").read_text().splitlines())
            self.assertNotIn("OAC_ALLOW_INSECURE_ORIGIN", env)

    def write_executable(self, path, text):
        path.write_text(text)
        path.chmod(path.stat().st_mode | stat.S_IEXEC)


if __name__ == "__main__":
    unittest.main()
