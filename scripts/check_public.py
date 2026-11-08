"""Check tracked or source files for private runtime material."""
import argparse
from pathlib import Path
import re
import subprocess

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--tracked", action="store_true")
args = parser.parse_args()
root = Path(__file__).resolve().parents[1]
if args.tracked:
    raw = subprocess.check_output(["git", "ls-files", "-z"], cwd=root)
    files = [root / name.decode() for name in raw.split(b"\0") if name]
else:
    files = [p for p in root.rglob("*") if p.is_file() and
             not set(p.relative_to(root).parts).intersection(
                 {".git", ".local", ".worktrees", "build", "bin", "dist", "artifacts", "data", "__pycache__"})]
patterns = [
    re.compile(r"(?i)[A-Z]:[\\/](?:Users|Code)[\\/]"),
    re.compile(r"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----"),  # leak-guard-allow:LG-CCF1466D
    re.compile(r"\b(?:ghp_|gho_)[A-Za-z0-9]{20,}\b"),
    re.compile(r"\bya29\.[A-Za-z0-9_-]{20,}\b"),
]
failures = []
for path in files:
    if not path.exists():
        continue
    relative = path.relative_to(root).as_posix()
    if path.suffix.lower() in {".db", ".conf", ".log", ".key", ".exe"} or relative.startswith(".local/"):
        failures.append(f"{relative}: private or generated file")
        continue
    if path.suffix.lower() in {".png", ".gif", ".ico"}:
        continue
    text = path.read_text(encoding="utf-8", errors="replace")
    if any(pattern.search(text) for pattern in patterns):
        failures.append(f"{relative}: private path or credential pattern")
if failures:
    raise SystemExit("\n".join(failures))
print(f"Public source checks passed ({len(files)} files).")
