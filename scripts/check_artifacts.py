"""Check public release names, contents and their SHA-256 manifest."""
import argparse
import hashlib
from pathlib import Path
import tarfile
import zipfile
import subprocess
import sys

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--directory", default="dist")
parser.add_argument("--require-signed-updates", action="store_true",
                    help="require and verify official MyGo update manifests")
parser.add_argument("--config", default=str(Path(__file__).resolve().parents[1] / "mygo.json"),
                    help="public MyGo configuration whose key/version verify signed updates")
args = parser.parse_args()
root = Path(args.directory)
if not (root/"SHA256SUMS").is_file():
    manifests=list(root.rglob("SHA256SUMS"))
    if not manifests:
        raise SystemExit("no release checksum manifests")
    for path in manifests:
        subprocess.run([sys.executable,__file__,"--directory",str(path.parent),
                        "--config",args.config],check=True)
    if args.require_signed_updates and not list(root.rglob("update-*.json")):
        raise SystemExit("signed updates were required but no update manifests were found")
    raise SystemExit(0)
manifest = root / "SHA256SUMS"
listed = set()
for line in manifest.read_text(encoding="utf-8").splitlines():
    expected, name = line.split("  ", 1)
    if name in listed:
        raise SystemExit(f"duplicate checksum entry: {name}")
    listed.add(name)
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
signed_manifests = list(root.glob("update-*.json"))
if args.require_signed_updates and not signed_manifests:
    raise SystemExit("signed updates were required but no update manifests were found")
if signed_manifests:
    if any(path.name not in listed for path in signed_manifests):
        raise SystemExit("signed update manifest is missing from SHA256SUMS")
    project = Path(__file__).resolve().parents[1]
    command = [
        "go", "run", "./internal/updates/cmd/verify", "--directory", str(root.resolve()),
        "--config", str(Path(args.config).resolve()),
    ]
    if args.require_signed_updates:
        command.append("--required")
    subprocess.run(command, cwd=project, check=True)
print("Release checksums and archive contents passed.")
