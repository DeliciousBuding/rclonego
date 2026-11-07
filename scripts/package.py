"""Prepare public portable artifacts, checksums and dependency inventory."""
import argparse
import hashlib
import json
from pathlib import Path
import platform
import shutil
import subprocess
import zipfile

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--platform", required=True)
args = parser.parse_args()
root = Path(__file__).resolve().parents[1]
version = json.loads((root / "mygo.json").read_text(encoding="utf-8"))["version"]
operating_system, architecture = args.platform.split("/")
build = root / "build" / f"{operating_system}-{architecture}"
dist = root / "dist"
dist.mkdir(exist_ok=True)
prefix = f"rclonego-{version}-{operating_system}-{architecture}"
if operating_system == "windows":
    executable = dist / f"{prefix}.exe"
    shutil.copy2(build / "RcloneGo.exe", executable)
    with zipfile.ZipFile(dist / f"{prefix}.zip", "w", zipfile.ZIP_DEFLATED) as archive:
        archive.write(executable, "RcloneGo.exe")
        for name in ["LICENSE", "NOTICE", "docs/third-party.md"]:
            archive.write(root / name, name)
    for installer in build.glob("rclonego-setup-*.exe"):
        shutil.copy2(installer, dist / installer.name)
elif operating_system == "darwin":
    # MyGo signs and prepares bundles on a macOS release runner. A .zip can
    # still be prepared on a development host without claiming notarization.
    with zipfile.ZipFile(dist / f"{prefix}.zip", "w", zipfile.ZIP_DEFLATED) as archive:
        for path in (build / "RcloneGo.app").rglob("*"):
            if path.is_file():
                member = zipfile.ZipInfo(path.relative_to(build).as_posix())
                member.create_system = 3
                member.external_attr = (0o100755 if "MacOS" in path.parts else 0o100644) << 16
                member.compress_type = zipfile.ZIP_DEFLATED
                archive.writestr(member, path.read_bytes())
        archive.write(root / "LICENSE", "LICENSE")
        archive.write(root / "NOTICE", "NOTICE")
    for image in build.glob("*.dmg"):
        shutil.copy2(image, dist / image.name)
else:
    for pattern in ["*.deb", "*.tar.gz"]:
        for path in build.glob(pattern):
            shutil.copy2(path, dist / path.name)

modules = []
raw = subprocess.check_output(["go", "list", "-m", "-json", "all"], cwd=root, text=True)
decoder = json.JSONDecoder()
while raw.strip():
    raw = raw.lstrip()
    module, index = decoder.raw_decode(raw)
    raw = raw[index:]
    modules.append({"type": "library", "name": module["Path"],
                    **({"version": module["Version"]} if "Version" in module else {})})
inventory = {"bomFormat": "CycloneDX", "specVersion": "1.5", "version": 1,
             "metadata": {"component": {"type": "application", "name": "RcloneGo", "version": version}},
             "components": modules}
(dist / f"{prefix}.sbom.json").write_text(json.dumps(inventory, indent=2) + "\n", encoding="utf-8")
files = sorted(path for path in dist.iterdir() if path.is_file() and path.name != "SHA256SUMS")
(dist / "SHA256SUMS").write_text("".join(
    f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n" for path in files), encoding="utf-8")
print(f"Prepared {prefix}")
