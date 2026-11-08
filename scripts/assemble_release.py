"""Combine platform artifacts with verified hashes into one draft release."""
import hashlib
from pathlib import Path
import shutil
import subprocess
import sys

source, destination = map(Path, sys.argv[1:3])
destination.mkdir(exist_ok=True)
seen = {}
for manifest in sorted(source.rglob("SHA256SUMS")):
    platform=manifest.parent
    subprocess.run([sys.executable, "scripts/check_artifacts.py", "--directory", str(platform)], check=True)
    for line in (platform / "SHA256SUMS").read_text(encoding="utf-8").splitlines():
        expected, name = line.split("  ", 1)
        if name in seen:
            if seen[name] != expected:
                raise SystemExit(f"conflicting release asset: {name}")
            # MyGo's Linux archive is both a portable package and a signed
            # update. Identical copies become one immutable release asset.
            continue
        seen[name] = expected
        shutil.copy2(platform / name, destination / name)
checksums = []
for path in sorted(destination.iterdir()):
    if not path.is_file() or path.name == "SHA256SUMS":
        continue
    with path.open("rb") as stream:
        digest = hashlib.file_digest(stream, "sha256").hexdigest()
    checksums.append(f"{digest}  {path.name}\n")
(destination / "SHA256SUMS").write_text("".join(checksums), encoding="utf-8")
subprocess.run([sys.executable, "scripts/check_artifacts.py", "--directory", str(destination)], check=True)
print(f"Assembled {len(checksums)} verified release assets.")
