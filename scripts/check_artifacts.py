"""Check public release names, contents and their SHA-256 manifest."""
import argparse
import hashlib
import json
from pathlib import Path
import plistlib
import struct
import tarfile
import zipfile
import subprocess
import sys


def check_binary(data, target):
    operating_system, architecture = target.split("/")
    if operating_system == "windows":
        if len(data) < 64 or data[:2] != b"MZ":
            raise SystemExit("Windows package is missing a PE executable")
        offset = struct.unpack_from("<I", data, 60)[0]
        if offset + 6 > len(data) or data[offset:offset + 4] != b"PE\0\0":
            raise SystemExit("invalid Windows PE header")
        machine = struct.unpack_from("<H", data, offset + 4)[0]
        expected = {"amd64": 0x8664, "arm64": 0xAA64}[architecture]
    elif operating_system == "darwin":
        if len(data) < 8 or data[:4] != b"\xcf\xfa\xed\xfe":
            raise SystemExit("macOS package is missing a 64-bit Mach-O executable")
        machine = struct.unpack_from("<I", data, 4)[0]
        expected = {"amd64": 0x01000007, "arm64": 0x0100000C}[architecture]
    else:
        if len(data) < 20 or data[:4] != b"\x7fELF" or data[4:6] != b"\x02\x01":
            raise SystemExit("Linux package is missing a little-endian 64-bit ELF executable")
        machine = struct.unpack_from("<H", data, 18)[0]
        expected = {"amd64": 62, "arm64": 183}[architecture]
    if machine != expected:
        raise SystemExit(f"executable architecture does not match {target}")


def check_platform_packages(root, config, target, require_installer, require_updates):
    operating_system, architecture = target.split("/")
    if operating_system not in {"windows", "darwin", "linux"} or architecture not in {"amd64", "arm64"}:
        raise SystemExit(f"unsupported release target: {target}")
    version, slug, name = config["version"], config["slug"], config["name"]
    prefix = f"{slug}-{version}-{operating_system}-{architecture}"

    def asset(filename):
        matches = list(root.rglob(filename))
        if not matches:
            raise SystemExit(f"missing release asset: {filename}")
        return matches[0]

    if operating_system == "windows":
        executable = asset(f"{prefix}.exe")
        with executable.open("rb") as stream:
            check_binary(stream.read(4096), target)
        with zipfile.ZipFile(asset(f"{prefix}.zip")) as archive:
            for member in [f"{name}.exe", "LICENSE", "NOTICE", "docs/third-party.md"]:
                if member not in archive.namelist() or archive.getinfo(member).file_size == 0:
                    raise SystemExit(f"Windows ZIP is missing {member}")
            with archive.open(f"{name}.exe") as stream:
                check_binary(stream.read(4096), target)
        if require_installer:
            installer = asset(f"{slug}-setup-{version}-{architecture}.exe")
            with installer.open("rb") as stream:
                if stream.read(2) != b"MZ" or installer.stat().st_size < 1024:
                    raise SystemExit("invalid Windows installer executable")
    elif operating_system == "darwin":
        with zipfile.ZipFile(asset(f"{prefix}.zip")) as archive:
            binary = f"{name}.app/Contents/MacOS/{name}"
            required = [binary, f"{name}.app/Contents/Info.plist", "LICENSE", "NOTICE", "docs/third-party.md"]
            if any(member not in archive.namelist() for member in required):
                raise SystemExit("macOS ZIP is missing application or license files")
            if not (archive.getinfo(binary).external_attr >> 16) & 0o111:
                raise SystemExit("macOS application executable lost its Unix permissions")
            with archive.open(binary) as stream:
                check_binary(stream.read(4096), target)
            info = plistlib.loads(archive.read(f"{name}.app/Contents/Info.plist"))
            if info.get("CFBundleIdentifier") != config["identifier"] or info.get("CFBundleExecutable") != name:
                raise SystemExit("macOS bundle identity differs from the application configuration")
        image = asset(f"{slug}-{version}-{architecture}.dmg")
        with image.open("rb") as stream:
            if image.stat().st_size < 512:
                raise SystemExit("invalid macOS disk image")
            stream.seek(-512, 2)
            if stream.read(4) != b"koly":
                raise SystemExit("invalid macOS disk image trailer")
    else:
        with asset(f"{prefix}.deb").open("rb") as stream:
            if stream.read(8) != b"!<arch>\n":
                raise SystemExit("invalid Debian package")
        with tarfile.open(asset(f"{slug}-{version}-linux-{architecture}.tar.gz")) as archive:
            try:
                binary = archive.getmember(slug)
            except KeyError:
                raise SystemExit("Linux archive is missing the application executable")
            if not binary.isfile() or not binary.mode & 0o111:
                raise SystemExit("Linux application executable lost its Unix permissions")
            with archive.extractfile(binary) as stream:
                check_binary(stream.read(4096), target)
    if require_updates:
        asset(f"update-{operating_system}-{architecture}.json")


parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--directory", default="dist")
parser.add_argument("--platform", help="require complete release packages for an os/architecture target")
parser.add_argument("--require-installer", action="store_true",
                    help="require the Windows NSIS installer with --platform")
parser.add_argument("--require-signed-updates", action="store_true",
                    help="require and verify official MyGo update manifests")
parser.add_argument("--config", default=str(Path(__file__).resolve().parents[1] / "mygo.json"),
                    help="public MyGo configuration whose key/version verify signed updates")
args = parser.parse_args()
if args.require_installer and (not args.platform or not args.platform.startswith("windows/")):
    parser.error("--require-installer requires a Windows --platform")
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
    if args.platform:
        check_platform_packages(root, json.loads(Path(args.config).read_text(encoding="utf-8")),
                                args.platform, args.require_installer, args.require_signed_updates)
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
actual = {path.name for path in root.iterdir() if path.is_file() and path.name != "SHA256SUMS"}
if listed != actual:
    raise SystemExit("release files and SHA256SUMS entries differ")

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
    from urllib.parse import urlparse
    config = json.loads(Path(args.config).read_text(encoding="utf-8"))
    for path in signed_manifests:
        target = path.stem.removeprefix("update-").replace("-", "/", 1)
        operating_system, _ = target.split("/")
        binary = config["name"] + ".exe" if operating_system == "windows" else config["slug"]
        if operating_system == "darwin":
            binary = f"{config['name']}.app/Contents/MacOS/{config['name']}"
        update = json.loads(path.read_text(encoding="utf-8"))
        archive_path = root / Path(urlparse(update["url"]).path).name
        with tarfile.open(archive_path) as archive:
            try:
                executable = archive.getmember(binary)
            except KeyError:
                raise SystemExit("signed update archive is missing its application executable")
            if not executable.isfile() or operating_system != "windows" and not executable.mode & 0o111:
                raise SystemExit("signed update archive lost executable permissions")
            with archive.extractfile(executable) as stream:
                check_binary(stream.read(4096), target)
    project = Path(__file__).resolve().parents[1]
    command = [
        "go", "run", "./internal/updates/cmd/verify", "--directory", str(root.resolve()),
        "--config", str(Path(args.config).resolve()),
    ]
    if args.require_signed_updates:
        command.append("--required")
    subprocess.run(command, cwd=project, check=True)
if args.platform:
    check_platform_packages(root, json.loads(Path(args.config).read_text(encoding="utf-8")),
                            args.platform, args.require_installer, args.require_signed_updates)
print("Release checksums and archive contents passed.")
