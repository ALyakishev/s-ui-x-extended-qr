"""Validate the final Linux package without extracting or executing its contents."""
import hashlib
import json
from pathlib import Path
import sys
import tarfile


def validate(version, platform, directory):
    archive = Path(directory) / f"s-ui-linux-{platform}.tar.gz"
    digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    checksum = Path(str(archive) + ".sha256").read_text().split()
    if checksum != [digest, archive.name]:
        raise ValueError("archive checksum mismatch")
    manifest = json.loads(Path(str(archive) + ".manifest.json").read_text())
    expected = dict(version=version, channel="beta" if "-" in version else "main",
                    platform=platform, filename=archive.name, sha256=digest)
    if manifest != expected:
        raise ValueError("update manifest does not bind the exact release archive")
    with tarfile.open(archive, "r:gz") as package:
        members = package.getmembers()
        names = [member.name for member in members]
        expected_names = {"s-ui", "s-ui/sui", "s-ui/s-ui.service", "s-ui/s-ui.sh"}
        if len(names) != len(set(names)) or set(names) != expected_names:
            raise ValueError("unexpected or duplicate Linux archive entries")
        for member in members:
            if member.name == "s-ui":
                if not member.isdir():
                    raise ValueError("package root is not a directory")
            elif not member.isfile() or member.size == 0:
                raise ValueError("package must contain only nonempty regular files")
            if member.name == "s-ui/sui" and not member.mode & 0o111:
                raise ValueError("sui is not executable")
    print(f"Verified Linux release package: {archive.name}")


if __name__ == "__main__":
    validate(*sys.argv[1:])
