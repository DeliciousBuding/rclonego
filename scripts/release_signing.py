"""Prepare temporary OS-signing credentials on an isolated release runner."""
import argparse
import base64
import json
import os
from pathlib import Path
import platform
import secrets
import shlex
import shutil
import subprocess
import tempfile


def run(command):
    result = subprocess.run(command, capture_output=True, text=True)
    if result.returncode:
        # Tool output and command arguments can contain signing credentials.
        raise SystemExit(f"OS signing setup failed in {Path(command[0]).name}.")
    return result.stdout


def required(name):
    value = os.environ.get(name)
    if not value:
        raise SystemExit(f"OS signing requires {name}.")
    return value


def write_private(path, data):
    path.write_bytes(data)
    path.chmod(0o600)


def certificate(name):
    try:
        return base64.b64decode(required(name), validate=True)
    except ValueError:
        raise SystemExit(f"{name} must be base64-encoded certificate data.")


def export(name, value):
    if "\n" in value or "\r" in value:
        raise SystemExit("Invalid signing environment value.")
    with Path(required("GITHUB_ENV")).open("a", encoding="utf-8") as stream:
        stream.write(f"{name}={value}\n")


def prepare():
    system = platform.system()
    if system == "Windows":
        if os.environ.get("RCLONEGO_WINDOWS_SIGN_COMMAND"):
            return
        data = certificate("RCLONEGO_WINDOWS_PFX_BASE64")
        required("MYGO_WINDOWS_CERTIFICATE_PASSWORD")
    elif system == "Darwin":
        data = certificate("RCLONEGO_MACOS_P12_BASE64")
        password = required("RCLONEGO_MACOS_CERTIFICATE_PASSWORD")
        identity = required("RCLONEGO_MACOS_SIGNING_IDENTITY")
        key = required("RCLONEGO_APPLE_API_PRIVATE_KEY")
        key_id = required("RCLONEGO_APPLE_API_KEY_ID")
        issuer = required("RCLONEGO_APPLE_API_ISSUER_ID")
    else:
        return
    runner = Path(required("RUNNER_TEMP")).resolve()
    directory = Path(tempfile.mkdtemp(prefix="rclonego-signing-", dir=runner))
    export("RCLONEGO_SIGNING_DIRECTORY", str(directory))
    # Publish the cleanup path before any import that can fail.
    if system == "Windows":
        path = directory / "windows.pfx"
        write_private(path, data)
        export("RCLONEGO_WINDOWS_CERTIFICATE", str(path))
        return
    signing_certificate = directory / "developer.p12"
    api_key = directory / "notary.p8"
    keychain = directory / "signing.keychain-db"
    write_private(signing_certificate, data)
    write_private(api_key, key.encode("utf-8"))
    previous = shlex.split(run(["security", "list-keychains", "-d", "user"]))
    write_private(directory / "keychains.json", json.dumps(previous).encode("utf-8"))
    keychain_password = secrets.token_urlsafe(32)
    run(["security", "create-keychain", "-p", keychain_password, str(keychain)])
    run(["security", "set-keychain-settings", "-lut", "21600", str(keychain)])
    run(["security", "unlock-keychain", "-p", keychain_password, str(keychain)])
    run(["security", "import", str(signing_certificate), "-k", str(keychain),
         "-P", password, "-T", "/usr/bin/codesign", "-T", "/usr/bin/security"])
    run(["security", "set-key-partition-list", "-S", "apple-tool:,apple:,codesign:",
         "-s", "-k", keychain_password, str(keychain)])
    run(["security", "list-keychains", "-d", "user", "-s", *previous, str(keychain)])
    if identity not in run(["security", "find-identity", "-v", "-p", "codesigning", str(keychain)]):
        raise SystemExit("The imported keychain does not contain the requested signing identity.")
    profile = "rclonego-release"
    run(["xcrun", "notarytool", "store-credentials", profile, "--key", str(api_key),
         "--key-id", key_id, "--issuer", issuer, "--keychain", str(keychain)])
    export("RCLONEGO_MACOS_NOTARY_PROFILE", profile)
    export("RCLONEGO_MACOS_NOTARY_KEYCHAIN", str(keychain))


def cleanup():
    location = os.environ.get("RCLONEGO_SIGNING_DIRECTORY")
    if not location:
        return
    runner = Path(required("RUNNER_TEMP")).resolve()
    directory = Path(location).resolve()
    if directory.parent != runner or not directory.name.startswith("rclonego-signing-"):
        raise SystemExit("Signing cleanup target is outside this runner's temporary directory.")
    if not directory.is_dir():
        return
    if platform.system() == "Darwin":
        previous_file = directory / "keychains.json"
        if previous_file.is_file():
            previous = json.loads(previous_file.read_text(encoding="utf-8"))
            run(["security", "list-keychains", "-d", "user", "-s", *previous])
        keychain = directory / "signing.keychain-db"
        if keychain.is_file():
            run(["security", "delete-keychain", str(keychain)])
    shutil.rmtree(directory)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["prepare", "cleanup"])
    args = parser.parse_args()
    (prepare if args.action == "prepare" else cleanup)()
