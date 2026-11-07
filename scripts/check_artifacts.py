"""Check public release names, contents and their SHA-256 manifest."""
import argparse
import hashlib
from pathlib import Path
import tarfile
import zipfile

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--directory", default="dist")
args = parser.parse_args()
root = Path(args.directory)
manifest = root / "SHA256SUMS"
for line in manifest.read_text(encoding="utf-8").splitlines():
    expected, name = line.split("  ", 1)
    path = root / name
    if not path.resolve().is_relative_to(root.resolve()) or not path.is_file():
        raise SystemExit("invalid release manifest path")
    with path.open("rb") as stream:
        digest = hashlib.file_digest(stream, "sha256").hexdigest()
    if digest != expected:
        raise SystemExit(f"checksum mismatch: {name}")

def check_member(name):
    parts = Path(name.replace("\\", "/")).parts
    if Path(name).is_absolute() or ".." in parts or set(parts) & {".local", ".git", "logs", "data"}:
        raise SystemExit(f"private or unsafe archive member: {name}")
    if Path(name).suffix.lower() in {".conf", ".db", ".log", ".key", ".env"}:
        raise SystemExit(f"runtime material in release: {name}")

for path in root.iterdir():
    if path.suffix == ".zip":
        with zipfile.ZipFile(path) as archive:
            if archive.testzip() is not None:
                raise SystemExit("corrupt ZIP")
            for info in archive.infolist():
                check_member(info.filename)
    elif path.name.endswith(".tar.gz"):
        with tarfile.open(path) as archive:
            for info in archive:
                check_member(info.name)
                if info.issym() or info.islnk():
                    check_member(info.linkname)
print("Release checksums and archive contents passed.")
