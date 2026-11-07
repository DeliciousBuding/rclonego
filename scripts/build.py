"""Build the native application with pinned MyGo tooling."""
import argparse
import os
from pathlib import Path
import subprocess

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--platform", help="MyGo target, e.g. windows/amd64 or darwin/universal")
args = parser.parse_args()
root = Path(__file__).resolve().parents[1]
env = os.environ.copy()
env["GOFLAGS"] = (env.get("GOFLAGS", "") + " -trimpath").strip()
command = ["go", "run", "github.com/egoist/mygo/cmd/mygo", "build", "-skip-installer"]
if args.platform:
    command += ["-platform", args.platform]
subprocess.run(command, cwd=root, env=env, check=True)
