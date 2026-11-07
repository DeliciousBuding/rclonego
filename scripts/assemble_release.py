"""Combine platform artifacts with verified hashes into one draft release."""
import hashlib
from pathlib import Path
import shutil
import subprocess
import sys

source, destination = map(Path, sys.argv[1:3])
destination.mkdir(exist_ok=True)
seen = set()
for platform in sorted(source.iterdir()):
    if not platform.is_dir():
        continue
    subprocess.run([sys.executable, "scripts/check_artifacts.py", "--directory", str(platform)], check=True)
    for line in (platform / "SHA256SUMS").read_text(encoding="utf-8").splitlines():
        expected, name = line.split("  ", 1)
        if name in seen:
            raise SystemExit(f"duplicate release asset: {name}")
        seen.add(name)
        shutil.copy2(platform / name, destination / name)
checksums = []
for path in sorted(destination.iterdir()):
    if not path.is_file() or path.name == "SHA256SUMS":
        continue
    with path.open("rb") as stream:
        digest = hashlib.file_digest(stream, "sha256").hexdigest()
    checksums.append(f"{digest}  {path.name}\n")
(destination / "SHA256SUMS").write_text("".join(checksums), encoding="utf-8")
print(f"Assembled {len(checksums)} verified release assets.")
