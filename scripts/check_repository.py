"""Check release version consistency and local documentation links."""
import argparse
import json
from pathlib import Path
import re

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--tag", default="")
args = parser.parse_args()
root = Path(__file__).resolve().parents[1]
version = json.loads((root / "mygo.json").read_text(encoding="utf-8"))["version"]
go_version = re.search(r'const Version = "([^"]+)"',
                       (root / "internal/version/version.go").read_text(encoding="utf-8"))[1]
if version != go_version:
    raise SystemExit("mygo.json and application version differ")
if args.tag and args.tag != f"v{version}":
    raise SystemExit("release tag does not match application version")
notes = root / "release-notes.md"
if f"# RcloneGo {version}" not in notes.read_text(encoding="utf-8"):
    raise SystemExit("release notes do not match application version")
for document in [*root.glob("*.md"), *root.glob("docs/*.md"), *root.glob(".github/*.md")]:
    text = document.read_text(encoding="utf-8")
    targets = re.findall(r"!?\[[^\]]*\]\(([^ )]+)(?:[^)]*)\)", text)
    targets += re.findall(r'(?:src|href)="([^"]+)"', text)
    for target in targets:
        if target.startswith(("#", "http:", "https:", "mailto:")):
            continue
        path = (document.parent / target.split("#")[0]).resolve()
        if not path.is_relative_to(root) or not path.exists():
            raise SystemExit(f"{document.relative_to(root)}: invalid local link {target}")
print(f"Repository version and links passed ({version}).")
