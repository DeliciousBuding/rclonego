"""Run product tests with an isolated, verified rclone integration engine."""
import argparse
import hashlib
import os
from pathlib import Path
import platform
import re
import subprocess
import tempfile
import urllib.request
import zipfile

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--race", action="store_true")
args = parser.parse_args()
root = Path(__file__).resolve().parents[1]
source = (root / "internal/engine/manager.go").read_text(encoding="utf-8")
version = re.search(r'const Recommended = "([^"]+)"', source)[1]
operating_system = {"Windows": "windows", "Linux": "linux", "Darwin": "darwin"}[platform.system()]
arch = {"amd64": "amd64", "x86_64": "amd64", "arm64": "arm64", "aarch64": "arm64"}[platform.machine().lower()]
digest = re.search(rf'"{operating_system}/{arch}":\s+"([a-f0-9]+)"', source)[1]
target_os = "osx" if operating_system == "darwin" else operating_system
asset = f"rclone-{version}-{target_os}-{arch}.zip"
with tempfile.TemporaryDirectory(prefix="rclonego-test-") as directory:
    archive = Path(directory) / asset
    request = urllib.request.Request(
        f"https://github.com/rclone/rclone/releases/download/{version}/{asset}",
        headers={"User-Agent": "RcloneGo-tests"})
    with urllib.request.urlopen(request, timeout=120) as response:
        data = response.read(128 * 1024 * 1024)
    if hashlib.sha256(data).hexdigest() != digest:
        raise SystemExit("rclone release SHA-256 mismatch")
    archive.write_bytes(data)
    binary = "rclone.exe" if operating_system == "windows" else "rclone"
    with zipfile.ZipFile(archive) as members:
        entry = next(info for info in members.infolist() if Path(info.filename).name == binary)
        engine = Path(directory) / binary
        engine.write_bytes(members.read(entry))
    engine.chmod(0o700)
    env = os.environ.copy()
    env["RCLONEGO_TEST_ENGINE"] = str(engine)
    command = ["go", "test"]
    if args.race:
        command.append("-race")
    command += ["./..."]
    subprocess.run(command, cwd=root, env=env, check=True)
