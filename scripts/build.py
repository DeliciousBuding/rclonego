"""Build the native application with pinned MyGo tooling."""
import argparse
import os
from pathlib import Path
import subprocess
import hashlib
import json
import platform
import urllib.request
import urllib.parse
import shutil
import sys
import tempfile


def signing_configuration(config, environment, root):
    """Keep machine-specific signing inputs out of the repository config."""
    result = json.loads(json.dumps(config))
    windows = result.get("windows", {})
    certificate = environment.get("RCLONEGO_WINDOWS_CERTIFICATE")
    sign_command = environment.get("RCLONEGO_WINDOWS_SIGN_COMMAND")
    if certificate and sign_command:
        raise SystemExit("Choose a Windows certificate or a signing command, not both.")
    if certificate:
        certificate = Path(certificate).expanduser().resolve()
        if not certificate.is_file():
            raise SystemExit("The Windows signing certificate is not a file.")
        windows.pop("signCommand", None)
        windows["certificate"] = str(certificate)
    elif sign_command:
        windows.pop("certificate", None)
        windows["signCommand"] = sign_command
    if windows:
        result["windows"] = windows
    macos = result.get("macos", {})
    if identity := environment.get("RCLONEGO_MACOS_SIGNING_IDENTITY"):
        macos["signingIdentity"] = identity
    if profile := environment.get("RCLONEGO_MACOS_NOTARY_PROFILE"):
        macos["notarize"] = {"keychainProfile": profile}
        if keychain := environment.get("RCLONEGO_MACOS_NOTARY_KEYCHAIN"):
            macos["notarize"]["keychain"] = str(Path(keychain).expanduser().resolve())
    if macos:
        result["macos"] = macos
    return result


def copy_build_source(root, destination):
    # Signing configuration is applied only to this disposable source snapshot.
    # Include current source changes, but no ignored profiles, logs or secrets.
    tracked = subprocess.check_output(
        ["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard"],
        cwd=root).decode("utf-8").split("\0")
    files = {root / name for name in tracked if name}
    files.update((root / "internal/persistence").glob("native_embed_*.go"))
    files.update((root / "internal/persistence/native").glob("*.bin"))
    for source in files:
        if not source.is_file() or source.is_symlink():
            continue
        target = destination / source.relative_to(root)
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source, target)


def verify_os_signing(stage, operating_system, architecture, config):
    if operating_system == "windows":
        files = [stage / "RcloneGo.exe"]
        installer = stage / f"{config['slug']}-setup-{config['version']}-{architecture}.exe"
        if installer.is_file():
            files.append(installer)
        for file in files:
            environment = os.environ.copy()
            environment["RCLONEGO_SIGNED_FILE"] = str(file)
            command = (
                "$signature = Get-AuthenticodeSignature -LiteralPath $env:RCLONEGO_SIGNED_FILE; "
                "if ($signature.Status -ne 'Valid') { exit 1 }"
            )
            subprocess.run(["powershell.exe", "-NoProfile", "-NonInteractive",
                            "-Command", command], env=environment, check=True)
    elif operating_system == "darwin":
        bundle = stage / "RcloneGo.app"
        subprocess.run(["codesign", "--verify", "--deep", "--strict", str(bundle)], check=True)
        subprocess.run(["spctl", "--assess", "--type", "execute", str(bundle)], check=True)
        image = stage / f"{config['slug']}-{config['version']}-{architecture}.dmg"
        subprocess.run(["xcrun", "stapler", "validate", str(image)], check=True)

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--platform", help="MyGo target, e.g. windows/amd64 or darwin/universal")
parser.add_argument("--installer", action="store_true", help="also build a Windows installer")
parser.add_argument("--require-signed-updates", action="store_true",
                    help="fail unless official MyGo signed update artifacts are generated")
parser.add_argument("--require-os-signing", action="store_true",
                    help="require verified Windows Authenticode or notarized macOS packages")
args = parser.parse_args()
root = Path(__file__).resolve().parents[1]
env = os.environ.copy()
config = json.loads((root / "mygo.json").read_text(encoding="utf-8"))
build_config = signing_configuration(config, env, root)
updates = config.get("updates")
if args.require_signed_updates and not updates:
    raise SystemExit("Signed updates require updates.publicKey and updates.github/url in mygo.json.")
if args.require_signed_updates and not (
    env.get("MYGO_UPDATER_PRIVATE_KEY") or updates.get("privateKey")
):
    raise SystemExit("Signed updates require the matching MYGO_UPDATER_PRIVATE_KEY.")
module = json.loads(subprocess.check_output(
    ["go", "mod", "download", "-json", "github.com/egoist/mygo"], cwd=root))
manifest = json.loads((Path(module["Dir"]) / "plugins/sqlite/mygo-plugin.json").read_text())
libraries = manifest["libraries"][0]["files"]
host_os = {"Windows": "windows", "Darwin": "darwin", "Linux": "linux"}[platform.system()]
host_arch = {"amd64": "amd64", "x86_64": "amd64", "arm64": "arm64", "aarch64": "arm64"}[platform.machine().lower()]
targets = (args.platform or f"{host_os}/{host_arch}").split(",")
if "darwin/universal" in targets:
    targets.remove("darwin/universal")
    targets += ["darwin/amd64", "darwin/arm64"]
if any(target.split("/")[0] != host_os for target in targets) and (
    args.require_signed_updates or env.get("MYGO_UPDATER_PRIVATE_KEY")
    or updates and updates.get("privateKey")
):
    raise SystemExit(
        "Signed release archives require a matching operating-system host; "
        "cross-build without signing for compilation checks only."
    )
if args.require_os_signing:
    for target in targets:
        operating_system = target.split("/")[0]
        if operating_system != host_os:
            raise SystemExit("OS signing verification requires a native platform build host.")
        if operating_system == "windows" and not (
            build_config.get("windows", {}).get("certificate") or build_config.get("windows", {}).get("signCommand")
        ):
            raise SystemExit("Windows OS signing requires a trusted certificate or signing command.")
        if operating_system == "darwin" and (
            build_config.get("macos", {}).get("signingIdentity", "-") == "-"
            or not build_config.get("macos", {}).get("notarize")
        ):
            raise SystemExit("macOS OS signing requires Developer ID and a notary keychain profile.")
for target in targets:
    operating_system, architecture = target.split("/")
    library = libraries[f"{operating_system}-{architecture}"]
    assets = root / "internal/persistence/native"
    assets.mkdir(parents=True, exist_ok=True)
    name = f"{operating_system}_{architecture}.bin"
    blob = assets / name
    if not blob.exists() or hashlib.sha256(blob.read_bytes()).hexdigest() != library["sha256"]:
        request = urllib.request.Request(library["url"], headers={"User-Agent": "RcloneGo-build"})
        with urllib.request.urlopen(request, timeout=120) as response:
            data = response.read(16 * 1024 * 1024)
        if hashlib.sha256(data).hexdigest() != library["sha256"]:
            raise SystemExit("SQLite library SHA-256 mismatch")
        blob.write_bytes(data)
    generated = root / f"internal/persistence/native_embed_{operating_system}_{architecture}.go"
    generated.write_text(
        "// Code generated by scripts/build.py; DO NOT EDIT.\n"
        "//go:build rclonego_embedded\n\npackage persistence\n\nimport _ \"embed\"\n\n"
        f"//go:embed native/{name}\nvar bundledLibrary []byte\n\n"
        f"const bundledLibraryName = {json.dumps(library['name'])}\n", encoding="utf-8")
    # A previous signed build must not supply the manifest of a new unsigned
    # build. Only remove this target's generated metadata, inside its stage.
    stage = root / config.get("out", "build") / f"{operating_system}-{architecture}"
    update_metadata = stage / f"update-{operating_system}-{architecture}.json"
    if not update_metadata.resolve().is_relative_to((root / config.get("out", "build")).resolve()):
        raise SystemExit("update output is outside the configured build directory")
    update_metadata.unlink(missing_ok=True)
    signed_destination = root / "dist" / f"signed-updates-{operating_system}-{architecture}"
    if not signed_destination.resolve().is_relative_to((root / "dist").resolve()):
        raise SystemExit("signed update output is outside dist")
    previous_checksums = signed_destination / "SHA256SUMS"
    if previous_checksums.is_file():
        for line in previous_checksums.read_text(encoding="utf-8").splitlines():
            _, previous_name = line.split("  ", 1)
            if Path(previous_name).name != previous_name:
                raise SystemExit("unsafe previous signed output name")
            previous_file = signed_destination / previous_name
            if previous_file.is_file() and not previous_file.is_symlink():
                previous_file.unlink()
        previous_checksums.unlink()
env["GOFLAGS"] = (env.get("GOFLAGS", "") + " -trimpath -tags=rclonego_embedded").strip()
command = ["go", "run", "github.com/egoist/mygo/cmd/mygo", "build"]
# The application uses the official MyGo build and updater. Its installer is
# adapted separately because a tray application must never be forcibly stopped.
command += ["-skip-installer"]
if args.platform:
    command += ["-platform", args.platform]
with tempfile.TemporaryDirectory(prefix="rclonego-build-") as snapshot:
    build_root = root
    if build_config != config:
        build_root = Path(snapshot)
        copy_build_source(root, build_root)
        (build_root / "mygo.json").write_text(
            json.dumps(build_config, indent=2) + "\n", encoding="utf-8")
    subprocess.run(
        command + ["-o", str(root / config.get("out", "build"))],
        cwd=build_root, env=env, check=True,
    )
if args.installer:
    sys.dont_write_bytecode = True
    from windows_installer import build_installer
    for target in targets:
        if target.startswith("windows/"):
            build_installer(root, build_config, Path(module["Dir"]), env,
                            target.split("/")[1])
if args.require_os_signing:
    for target in targets:
        operating_system, architecture = target.split("/")
        verify_os_signing(root / config.get("out", "build") / f"{operating_system}-{architecture}",
                          operating_system, architecture, build_config)

# Copy MyGo's update output without reimplementing its signing or installer.
# package.py independently prepares portable packages; signed update archives
# keep their original names because the generated manifests refer to them.
for target in targets:
    operating_system, architecture = target.split("/")
    stage = root / config.get("out", "build") / f"{operating_system}-{architecture}"
    manifest_path = stage / f"update-{operating_system}-{architecture}.json"
    if not manifest_path.exists():
        if args.require_signed_updates:
            raise SystemExit(f"Missing signed update manifest for {target}.")
        continue
    if not updates:
        raise SystemExit("Unexpected signed update output without public-key configuration.")
    subprocess.run(
        ["go", "run", "./internal/updates/cmd/verify", "--directory", str(stage),
         "--config", str(root / "mygo.json"), "--required"],
        cwd=root, check=True,
    )
    update_manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    names = [manifest_path.name, Path(urllib.parse.urlparse(update_manifest["url"]).path).name]
    names += [Path(urllib.parse.urlparse(delta["url"]).path).name
              for delta in update_manifest.get("deltas", [])]
    destination = root / "dist" / f"signed-updates-{operating_system}-{architecture}"
    destination.mkdir(parents=True, exist_ok=True)
    for name in names:
        shutil.copy2(stage / name, destination / name)
    checksums = []
    for name in sorted(set(names)):
        with (destination / name).open("rb") as stream:
            checksums.append(f"{hashlib.file_digest(stream, 'sha256').hexdigest()}  {name}\n")
    (destination / "SHA256SUMS").write_text("".join(checksums), encoding="utf-8")
    subprocess.run(
        [sys.executable, str(root / "scripts/check_artifacts.py"),
         "--directory", str(destination), "--require-signed-updates"],
        cwd=root, check=True,
    )
