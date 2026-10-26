"""Offline regression checks for release packaging and reusable workflow wiring."""
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("validator", ROOT / "scripts/validate-linux-release.py")
validator = importlib.util.module_from_spec(spec)
spec.loader.exec_module(validator)


class LinuxArchiveTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.directory = Path(self.tmp.name)
        self.archive = self.directory / "s-ui-linux-amd64.tar.gz"

    def package(self, extra=None, binary_type=tarfile.REGTYPE, executable=True):
        with tarfile.open(self.archive, "w:gz") as package:
            root = tarfile.TarInfo("s-ui")
            root.type = tarfile.DIRTYPE
            package.addfile(root)
            for name in ["sui", "s-ui.service", "s-ui.sh"] + ([extra] if extra else []):
                member = tarfile.TarInfo("s-ui/" + name)
                member.size = 4
                member.mode = 0o755 if executable else 0o644
                if name == "sui":
                    member.type = binary_type
                    member.linkname = "/etc/passwd" if binary_type == tarfile.SYMTYPE else ""
                if member.isfile():
                    package.addfile(member, io.BytesIO(b"test"))
                else:
                    member.size = 0
                    package.addfile(member)
        self.digest = hashlib.sha256(self.archive.read_bytes()).hexdigest()
        Path(str(self.archive) + ".sha256").write_text(f"{self.digest}  {self.archive.name}\n")
        self.manifest = dict(version="1.1.2-tt.1", channel="beta", platform="amd64",
                             filename=self.archive.name, sha256=self.digest)
        self.write_manifest()

    def write_manifest(self):
        Path(str(self.archive) + ".manifest.json").write_text(json.dumps(self.manifest))

    def validate(self):
        validator.validate("1.1.2-tt.1", "amd64", self.directory)

    def test_valid(self):
        self.package()
        self.validate()

    def test_corrupt_archive(self):
        self.package()
        with self.archive.open("ab") as stream:
            stream.write(b"corrupt")
        with self.assertRaisesRegex(ValueError, "checksum"):
            self.validate()

    def test_manifest_binding(self):
        self.package()
        for field in self.manifest:
            with self.subTest(field=field):
                original = self.manifest[field]
                self.manifest[field] = "wrong"
                self.write_manifest()
                with self.assertRaisesRegex(ValueError, "manifest"):
                    self.validate()
                self.manifest[field] = original

    def test_unexpected_path(self):
        self.package(extra="../../outside")
        with self.assertRaisesRegex(ValueError, "entries"):
            self.validate()

    def test_duplicate(self):
        self.package(extra="sui")
        with self.assertRaisesRegex(ValueError, "entries"):
            self.validate()

    def test_symlink(self):
        self.package(binary_type=tarfile.SYMTYPE)
        with self.assertRaisesRegex(ValueError, "regular"):
            self.validate()

    def test_not_executable(self):
        self.package(executable=False)
        with self.assertRaisesRegex(ValueError, "executable"):
            self.validate()


class WorkflowRegressionTest(unittest.TestCase):
    def test_reusable_frontend_artifacts_do_not_collide(self):
        release = (ROOT / ".github/workflows/release.yml").read_text()
        windows = (ROOT / ".github/workflows/windows.yml").read_text()
        self.assertIn("uses: ./.github/workflows/windows.yml", release)
        self.assertNotIn("name: frontend-dist", windows)
        self.assertEqual(windows.count("name: windows-frontend-dist"), 2)

    def test_release_source_and_binary_checks_are_wired(self):
        workflow = (ROOT / ".github/workflows/release.yml").read_text()
        self.assertIn('git merge-base --is-ancestor "$RELEASE_COMMIT" refs/remotes/origin/s-ui-x-extended', workflow)
        self.assertIn('python3 scripts/validate-linux-release.py "$RELEASE_VERSION" "$RELEASE_PLATFORM" .', workflow)
        self.assertIn('readelf -l sui', workflow)
        self.assertIn('with_trusttunnel', workflow)
        self.assertEqual((ROOT / 'go.mod').read_text().splitlines()[2], 'go 1.26.10')

    def test_security_covers_all_published_platforms(self):
        workflow = (ROOT / ".github/workflows/release.yml").read_text()
        security = workflow.split("\n  release-security:\n", 1)[1]
        self.assertIn("needs: [release-ref, build-linux, build-windows]", security)
        self.assertIn("pattern: s-ui-windows-*", security)
        for glob in ["*.tar.gz", "*.zip", "*.manifest.json", "*.sha256", "*.spdx.json"]:
            self.assertIn("release-assets/" + glob, security)
        self.assertIn("s-ui-linux-*.sha256 s-ui-windows-*.sha256", security)


if __name__ == "__main__":
    unittest.main()
