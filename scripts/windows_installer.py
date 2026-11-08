"""Build a per-user NSIS installer that never interrupts running RcloneGo."""
import ctypes
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import struct
import subprocess
import tempfile


INSTALLATION_CHECK = r"""$ErrorActionPreference = 'Stop'
try {
    $directory = [IO.Path]::GetFullPath($env:RCLONEGO_INSTALLER_DIR).TrimEnd('\')
    if (!$directory -or $directory -eq [IO.Path]::GetPathRoot($directory).TrimEnd('\')) { exit 3 }
    $prefix = $directory + '\'
    if ($env:RCLONEGO_INSTALLER_ACTION -eq 'remove-startup') {
        $id = $env:RCLONEGO_INSTALLER_ID
        $runPath = 'Software\Microsoft\Windows\CurrentVersion\Run'
        $run = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey($runPath, $true)
        if ($run) {
            try {
                $command = [string]$run.GetValue($id, '')
                $match = [regex]::Match($command, '^\s*(?:"([^"]+)"|(\S+))')
                $executable = if ($match.Groups[1].Success) { $match.Groups[1].Value } else { $match.Groups[2].Value }
                if ($executable -and [IO.Path]::GetFullPath($executable).StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)) {
                    $run.DeleteValue($id, $false)
                    $approved = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run', $true)
                    if ($approved) { try { $approved.DeleteValue($id, $false) } finally { $approved.Dispose() } }
                }
            } finally { $run.Dispose() }
        }
        exit 0
    }
    $parent = (Get-CimInstance Win32_Process -Filter "ProcessId=$PID").ParentProcessId
    foreach ($process in (Get-CimInstance Win32_Process)) {
        if ($process.ProcessId -eq $PID -or $process.ProcessId -eq $parent) { continue }
        if ($process.ExecutablePath) {
            $path = [IO.Path]::GetFullPath($process.ExecutablePath)
            if ($path.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)) { exit 2 }
        } elseif ($process.Name -ieq $env:RCLONEGO_INSTALLER_EXE) { exit 3 }
    }
    exit 0
} catch { exit 3 }
"""


def nsis_string(value):
    return str(value).replace("$", "$$").replace('"', '$\\"').replace("\r", "$\\r").replace("\n", "$\\n")


def _compiler(module_dir):
    available = shutil.which("makensis") or shutil.which("makensis.exe")
    if available:
        return Path(available)
    source = (module_dir / "internal/nsis/nsis.go").read_text(encoding="utf-8")
    version = re.search(r'Version:\s*"([^"]+)"', source)[1]
    cache = Path(os.environ["LOCALAPPDATA"]) / "mygo" / f"nsis-{version}"
    for relative in ["Bin/makensis.exe", "makensis.exe"]:
        if (cache / relative).is_file():
            return cache / relative
    # MyGo's ordinary installer build bootstraps its pinned, hash-verified NSIS.
    # Use the same pin here so clean hosted runners can build without an
    # unrelated package manager installation.
    import hashlib
    import urllib.request
    import zipfile
    digest = re.search(r'SHA256:\s*"([a-f0-9]+)"', source)[1]
    urls = re.findall(r'"(https://[^"]+\.zip)"', source)
    cache.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="rclonego-nsis-", dir=cache.parent) as temporary:
        archive = Path(temporary) / "nsis.zip"
        for url in urls:
            try:
                request = urllib.request.Request(url, headers={"User-Agent": "RcloneGo-build"})
                with urllib.request.urlopen(request, timeout=60) as response:
                    data = response.read(32 * 1024 * 1024)
                if hashlib.sha256(data).hexdigest() != digest:
                    continue
                archive.write_bytes(data)
                break
            except OSError:
                continue
        else:
            raise SystemExit("Cannot obtain MyGo's verified NSIS. Install NSIS and retry --installer.")
        unpacked = Path(temporary) / "unpacked"
        with zipfile.ZipFile(archive) as members:
            for member in members.infolist():
                destination = unpacked / member.filename
                if not destination.resolve().is_relative_to(unpacked.resolve()):
                    raise SystemExit("Unsafe path in the official NSIS archive.")
            members.extractall(unpacked)
        compiler = next(unpacked.rglob("makensis.exe"), None)
        if compiler is None:
            raise SystemExit("The official NSIS archive is missing makensis.")
        installation = compiler.parent.parent if compiler.parent.name.lower() == "bin" else compiler.parent
        if not cache.exists():
            shutil.copytree(installation, cache)
    for relative in ["Bin/makensis.exe", "makensis.exe"]:
        if (cache / relative).is_file():
            return cache / relative
    raise SystemExit("The verified NSIS installation is incomplete.")


def _payload(root, stage, executable):
    """Use MyGo's ownership list, rather than picking unrelated stage files."""
    files = []
    for name in (stage / ".mygo-files").read_text(encoding="utf-8").splitlines():
        relative = PurePosixPath(name.replace("\\", "/"))
        if not relative.parts or relative.is_absolute() or ".." in relative.parts or ":" in name:
            raise SystemExit("Unsafe path in MyGo's installed-file list")
        source = stage / relative
        if not source.resolve().is_relative_to(stage.resolve()) or source.is_symlink():
            raise SystemExit("MyGo's installed-file list leaves its build directory")
        candidates = sorted(source.rglob("*")) if source.is_dir() else [source]
        for candidate in candidates:
            if candidate.is_symlink() or not candidate.resolve().is_relative_to(stage.resolve()):
                raise SystemExit("Symlink in the installed application payload")
            if candidate.is_file():
                files.append((candidate.relative_to(stage).as_posix(), candidate))
    if executable not in {name for name, _ in files}:
        raise SystemExit("The Windows application executable is missing from MyGo's installed-file list")
    files.append((".mygo-files", stage / ".mygo-files"))
    files.extend((name, root / name) for name in ["LICENSE", "NOTICE", "docs/third-party.md"])
    if len({name for name, _ in files}) != len(files) or any(not path.is_file() for _, path in files):
        raise SystemExit("Missing or duplicate Windows installer payload")
    return files


def _application_icon(executable, destination):
    """Reuse the icon MyGo embedded in the application, without a bitmap tool."""
    library = ctypes.WinDLL("kernel32", use_last_error=True)
    library.LoadLibraryExW.argtypes = [ctypes.c_wchar_p, ctypes.c_void_p, ctypes.c_uint32]
    library.LoadLibraryExW.restype = ctypes.c_void_p
    library.FindResourceW.argtypes = [ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p]
    library.FindResourceW.restype = ctypes.c_void_p
    library.LoadResource.argtypes = [ctypes.c_void_p, ctypes.c_void_p]
    library.LoadResource.restype = ctypes.c_void_p
    library.LockResource.argtypes = [ctypes.c_void_p]
    library.LockResource.restype = ctypes.c_void_p
    library.SizeofResource.argtypes = [ctypes.c_void_p, ctypes.c_void_p]
    library.SizeofResource.restype = ctypes.c_uint32
    library.FreeLibrary.argtypes = [ctypes.c_void_p]
    module = library.LoadLibraryExW(str(executable), None, 0x22)
    if not module:
        raise OSError(ctypes.get_last_error(), "Cannot read the application icon")

    def resource(identifier, kind):
        handle = library.FindResourceW(module, ctypes.c_void_p(identifier), ctypes.c_void_p(kind))
        if not handle:
            raise OSError(ctypes.get_last_error(), "The application icon resource is missing")
        data = library.LockResource(library.LoadResource(module, handle))
        return ctypes.string_at(data, library.SizeofResource(module, handle))

    try:
        group = resource(1, 14)
        reserved, kind, count = struct.unpack_from("<HHH", group)
        entries, images = [], []
        offset = 6 + 16 * count
        for index in range(count):
            entry = group[6 + 14 * index:20 + 14 * index]
            identifier = struct.unpack_from("<H", entry, 12)[0]
            image = resource(identifier, 3)
            entries.append(entry[:8] + struct.pack("<II", len(image), offset))
            images.append(image)
            offset += len(image)
        destination.write_bytes(struct.pack("<HHH", reserved, kind, count) + b"".join(entries + images))
    finally:
        library.FreeLibrary(module)


def build_installer(root, config, module_dir, env, architecture):
    """Consume an official MyGo Windows stage; return the checked installer path."""
    if os.name != "nt":
        raise SystemExit("--installer requires a Windows build host")
    root, module_dir = Path(root).resolve(), Path(module_dir).resolve()
    stage = root / config.get("out", "build") / f"windows-{architecture}"
    if architecture not in {"amd64", "arm64"} or not stage.resolve().is_relative_to(root):
        raise SystemExit("Invalid Windows installer target")
    windows = config.get("windows", {})
    name = windows.get("displayName") or config["name"]
    executable = config["name"] + ".exe"
    output = stage / f'{config["slug"]}-setup-{config["version"]}-{architecture}.exe'
    files = _payload(root, stage, executable)
    compiler = _compiler(module_dir)
    template = (root / "scripts/windows-installer.nsi").read_text(encoding="utf-8")
    signing_env = env.copy()
    with tempfile.TemporaryDirectory(prefix="rclonego-installer-") as directory:
        temporary = Path(directory)
        icon = temporary / "application.ico"
        _application_icon(stage / executable, icon)
        signing = ""
        helper = temporary / "mygo.exe"
        if windows.get("certificate") or windows.get("signCommand"):
            subprocess.run(["go", "build", "-o", str(helper), "github.com/egoist/mygo/cmd/mygo"],
                           cwd=root, env=env, check=True)
            signing_env["MYGO_SIGNER"] = str(helper)
            signing_env["MYGO_SIGN_SETTINGS"] = json.dumps({"root": str(root), "windows": windows})
            signing = """!uninstfinalize '"%MYGO_SIGNER%" sign-uninstaller "%1"' = 0"""
        install = []
        for relative, source in files:
            folder = PurePosixPath(relative).parent
            output_directory = "$INSTDIR" if str(folder) == "." else "$INSTDIR\\" + nsis_string(str(folder).replace("/", "\\"))
            install.append(f'  SetOutPath "{output_directory}"')
            install.append(f'  File /oname={nsis_string(PurePosixPath(relative).name)} "{nsis_string(source)}"')
        directories = {str(parent) for name, _ in files
                       for parent in PurePosixPath(name).parents if str(parent) != "."}
        values = {
            "NAME": nsis_string(name), "ID": nsis_string(config["identifier"]),
            "EXE": nsis_string(executable), "VERSION": nsis_string(config["version"]),
            "PUBLISHER": nsis_string(config.get("publisher", config["name"])),
            "FOLDER": nsis_string(windows.get("installDirName") or config["name"]),
            "OUTPUT": nsis_string(output), "ICON": nsis_string(icon), "SIGNING": signing,
            "GUARD_LINES": "\n".join(
                f'  FileWrite $2 "{nsis_string(line)}$\\r$\\n"' for line in INSTALLATION_CHECK.splitlines()),
            "INSTALL_FILES": "\n".join(install),
            "UNINSTALL_FILES": "\n".join(
                f'  Delete "$INSTDIR\\{nsis_string(name.replace("/", chr(92)))}"' for name, _ in files),
            "UNINSTALL_DIRECTORIES": "\n".join(
                f'  RMDir "$INSTDIR\\{nsis_string(name.replace("/", chr(92)))}"'
                for name in sorted(directories, key=lambda item: len(PurePosixPath(item).parts), reverse=True)),
        }
        for token, value in values.items():
            template = template.replace(f"@@{token}@@", value)
        if "@@" in template:
            raise SystemExit("Unfilled Windows installer template field")
        script = temporary / "installer.nsi"
        script.write_text(template, encoding="utf-8-sig")
        subprocess.run([str(compiler), "-V2", str(script)], cwd=root, env=signing_env, check=True)
        if signing:
            subprocess.run([str(helper), "sign-uninstaller", str(output)],
                           cwd=root, env=signing_env, check=True)
    if not output.is_file() or output.stat().st_size < 1024:
        raise SystemExit("NSIS did not produce a Windows installer")
    return output
