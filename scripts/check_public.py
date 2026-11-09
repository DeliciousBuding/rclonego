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
    re.compile(r"(?<![:/\w])/(?:home|Users)/[A-Za-z0-9_.-]+/"),
    re.compile(r"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----"),  # leak-guard-allow:LG-CCF1466D
    re.compile(r"\b(?:gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{30,})\b"),
    re.compile(r"\bya29\.[A-Za-z0-9_-]{20,}\b"),
    re.compile(r"""refresh_token["']?\s*[:=]\s*["']?1//[A-Za-z0-9_-]{20,}"""),
]
failures = []
images = 0
private_directories = {".local", ".worktrees", "data", "build", "dist", "artifacts", "bin"}
private_suffixes = {".db", ".conf", ".log", ".key", ".pem", ".p12", ".pfx", ".exe", ".db-wal", ".db-shm"}
private_names = {"api-token", "engine-token", "instance.json", "state.json"}
for path in files:
    if not path.exists():
        continue
    relative = path.relative_to(root).as_posix()
    environment_file = path.name == ".env" or path.name.startswith(".env.") and path.name not in {".env.example", ".env.sample"}
    if path.suffix.lower() in private_suffixes or path.name in private_names or environment_file or set(Path(relative).parts) & private_directories:
        failures.append(f"{relative}: private or generated file")
        continue
    if path.suffix.lower() in {".png", ".gif", ".ico"}:
        images += 1
        continue
    text = path.read_text(encoding="utf-8", errors="replace")
    if any(pattern.search(text) for pattern in patterns):
        failures.append(f"{relative}: private path or credential pattern")
if failures:
    raise SystemExit("\n".join(failures))
print(f"Public source text and filename checks passed ({len(files)} files).")
if images:
    print(f"{images} raster assets require visual review; this check does not inspect image pixels or metadata.")
