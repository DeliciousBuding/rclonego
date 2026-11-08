"""Resolve a version tag and write only a reviewable, unpublished release."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
from urllib.parse import quote


def release_tag():
    tag = os.environ.get("RELEASE_TAG", "")
    if not re.fullmatch(r"v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?", tag):
        raise SystemExit("RELEASE_TAG must be an existing version tag such as v0.1.0-alpha.3.")
    return tag


def tag_commit(tag):
    result = subprocess.run(["git", "rev-parse", "--verify", f"refs/tags/{tag}^{{commit}}"],
                            capture_output=True, text=True, encoding="utf-8")
    if result.returncode:
        raise SystemExit("The requested version tag does not exist or does not name a commit.")
    return result.stdout.strip()


def release_state(tag):
    repository = os.environ.get("GITHUB_REPOSITORY", "")
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository):
        raise SystemExit("GITHUB_REPOSITORY must name the release repository.")
    result = subprocess.run(["gh", "api", f"repos/{repository}/releases/tags/{quote(tag, safe='')}",
                             "--jq", "{draft: .draft}"],
                            capture_output=True, text=True, encoding="utf-8")
    if result.returncode:
        if "HTTP 404" in result.stderr:
            return None
        raise SystemExit("Cannot verify release state; check GitHub authentication and connectivity.")
    state = json.loads(result.stdout)
    if state.get("draft") is not True:
        raise SystemExit("Published releases are immutable; choose a new version tag.")
    return state


def resolve():
    tag = release_tag()
    sha = tag_commit(tag)
    release_state(tag)
    for name in [
        "scripts/prepare_release.py", "scripts/check_native_package.py",
        "scripts/windows_installer.py", "scripts/windows-installer.nsi",
        "scripts/release_signing.py",
    ]:
        present = subprocess.run(["git", "cat-file", "-e", f"{sha}:{name}"],
                                 stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        if present.returncode:
            raise SystemExit("The requested tag predates this release pipeline; create a tag containing its build helpers.")
    output = os.environ.get("GITHUB_OUTPUT")
    if not output:
        raise SystemExit("GITHUB_OUTPUT is required for the manual Release prepare job.")
    with Path(output).open("a", encoding="utf-8") as stream:
        stream.write(f"tag={tag}\nsha={sha}\n")
    print(f"Prepared {tag} at {sha}.")


def draft():
    tag = release_tag()
    sha = os.environ.get("RELEASE_SHA", "")
    if not re.fullmatch(r"[a-f0-9]{40}", sha) or tag_commit(tag) != sha:
        raise SystemExit("The version tag changed after the Release prepare job.")
    head = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True, encoding="utf-8").strip()
    if head != sha:
        raise SystemExit("Release assembly is not checked out at the prepared commit.")
    assets = sorted(path for path in Path("release").iterdir() if path.is_file())
    if not assets or not (Path("release") / "SHA256SUMS").is_file():
        raise SystemExit("Checked release assets are missing.")
    if release_state(tag) is None:
        command = ["gh", "release", "create", tag, "--verify-tag", "--draft",
                   "--title", f"RcloneGo {tag[1:]}", "--notes-file", "release-notes.md"]
        if "-" in tag.split("+", 1)[0]:
            command.append("--prerelease")
        subprocess.run(command, check=True)
    # Recheck immediately before the only asset write; never overwrite a
    # release that another maintainer published while the jobs were running.
    if release_state(tag) is None:
        raise SystemExit("The release draft disappeared before upload.")
    subprocess.run(["gh", "release", "upload", tag, *map(str, assets), "--clobber"], check=True)
    print(f"Updated the unpublished draft for {tag}.")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["resolve", "draft"])
    args = parser.parse_args()
    (resolve if args.action == "resolve" else draft)()
