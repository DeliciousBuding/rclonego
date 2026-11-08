"""Run the built application's CLI on its matching native release runner."""
import argparse
import json
import os
from pathlib import Path
import platform
import subprocess
import tempfile

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--platform", required=True)
parser.add_argument("--directory", default="build")
args = parser.parse_args()
root = Path(__file__).resolve().parents[1]
config = json.loads((root / "mygo.json").read_text(encoding="utf-8"))
operating_system = {"Windows": "windows", "Linux": "linux", "Darwin": "darwin"}[platform.system()]
architecture = {"amd64": "amd64", "x86_64": "amd64", "arm64": "arm64", "aarch64": "arm64"}[platform.machine().lower()]
if args.platform != f"{operating_system}/{architecture}":
    raise SystemExit("native package verification must run on the target architecture")
stage = Path(args.directory).resolve() / f"{operating_system}-{architecture}"
name = config["name"]
executable = stage / (f"{name}.exe" if operating_system == "windows" else config["slug"])
if operating_system == "darwin":
    executable = stage / f"{name}.app" / "Contents" / "MacOS" / name

with tempfile.TemporaryDirectory(prefix="rclonego-package-check-") as profile:
    env = os.environ.copy()
    env["RCLONEGO_DATA"] = profile
    options = {"creationflags": subprocess.CREATE_NO_WINDOW} if operating_system == "windows" else {}
    version = subprocess.check_output([str(executable), "--version"], env=env,
                                      text=True, encoding="utf-8", timeout=30, **options).strip()
    if version != config["version"]:
        raise SystemExit("packaged executable version differs from mygo.json")
    licenses = json.loads(subprocess.check_output([str(executable), "licenses", "--json"],
                                                  env=env, text=True, encoding="utf-8",
                                                  timeout=30, **options))
    if not licenses.get("LICENSE") or not licenses.get("NOTICE"):
        raise SystemExit("packaged executable is missing its embedded license notices")
    if not any(key.startswith("third_party/licenses/") and value for key, value in licenses.items()):
        raise SystemExit("packaged executable is missing third-party licenses")
print(f"Native packaged CLI passed ({args.platform}, {version}).")
