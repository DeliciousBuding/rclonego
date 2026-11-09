<p align="center"><img src="assets/wordmark.svg" width="400" alt="RcloneGo"></p>
<p align="center">Reliable transfers. Visible workflows.</p>
<p align="center">
  <a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> ·
  <a href="https://github.com/DeliciousBuding/rclonego/releases">Downloads</a> ·
  <a href="docs/agents.md">Agent API</a>
</p>
<p align="center">
  <a href="https://github.com/DeliciousBuding/rclonego/actions/workflows/release.yml"><img src="https://github.com/DeliciousBuding/rclonego/actions/workflows/release.yml/badge.svg" alt="Manual release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache--2.0-318b9d" alt="Apache-2.0"></a>
  <a href="https://mygo.egoist.dev/"><img src="https://img.shields.io/badge/native_UI-MyGo-318b9d" alt="Built with MyGo"></a>
</p>

RcloneGo is a native desktop workspace for transfers, mounts and verification,
powered by [rclone](https://rclone.org/) and built with
[MyGo](https://mygo.egoist.dev/). Its GUI, CLI and authenticated local API share
one persistent scheduler.

This is an independent community project under active alpha development.
It is not an official rclone or MyGo application.

**Version status:** this README describes the alpha.3 development source.
The latest published preview is [v0.1.0-alpha.2](https://github.com/DeliciousBuding/rclonego/releases/tag/v0.1.0-alpha.2).
Changes introduced in alpha.3 require a source build or a later published preview.
No stable release is available yet.

<p align="center"><img src="assets/screenshots/overview-dark.png" width="900" alt="RcloneGo transfer overview with storage provider marks"></p>

Native component previews use sample accounts and transfers:
[Files](assets/screenshots/files-dark.png) ·
[Cloud accounts](assets/screenshots/accounts-light.png) ·
[Settings](assets/screenshots/settings-light.png).

## What it does

- **Transfer with evidence.** Copy, single-file copy, one-way mirror and checks.
  Reusable workflows create separate runs with ordered steps and retained attempts.
- **See and control execution.** Current folders/files, resource counters,
  concurrency, bandwidth, schedules, queue pause and task pause/resume.
- **Connect storage.** Verified engine installation, offline import, provider-driven
  account setup, browser OAuth and read verification. Mounts stay under recorded ownership.
- **Work from any interface.** Native light/dark/system appearance, tray background
  operation, English/Chinese UI, CLI JSON, offline OpenAPI, event streaming and
  revocable observer/task tokens.
- **Reuse and automate.** Edit, clone, import and export workflows. Opt-in
  directory watching and recurring workflows submit to the same queue.
- **Choose and inspect paths.** Browse cloud folders, inspect structured dry-run
  changes, observe owned VFS cache/writeback state and save independent offline copies.
- **Manage files.** Use the native [Files workspace](docs/files.md) for breadcrumbs,
  multi-selection, text/image previews, upload/download, copy, rename and confirmed
  single-file or empty-folder deletion. Observe existing Windows rclone drives.
  Batch feedback identifies queued and failed items, retries only items not added,
  and links to their tasks in the queue.
- **Understand storage.** Read provider quota, hash algorithms and backend
  capabilities. Browse files with sizes and request cancellable directory
  statistics. See other rclone processes separately from this client's usage.
- **Continue legacy work.** Preview a Data Ops Console import, preserve task
  order and exclusion evidence, and let its current upload finish before the
  shared queue takes over the remaining steps.

A mirror requires a matching dry-run preview and explicit deletion permission.
Verification reports distinguish sizes, available hashes and content-download
comparison. A completed copy is not automatically a completed content check.
Partial workflows verify the selected top-level entries and their descendants.
Storage failures suspend new dispatch until recovery. Hidden windows stop UI polling;
idle windows refresh less often, and logs have bounded retention.

## Install

Choose a package from [Releases](https://github.com/DeliciousBuding/rclonego/releases).
Available assets and platform validation are listed with each release.

| Platform | Package and validation |
| --- | --- |
| Windows amd64 | Per-user EXE installer recommended; no-install ZIP or executable available; native UI/CLI, upgrade and uninstall retention tested |
| Windows arm64 | EXE installer and no-install packages; native release runner configured, manual validation pending |
| Linux amd64 | `.deb` or `.tar.gz`; Ubuntu WSLg UI, transfers and FUSE mount/read/unmount tested |
| macOS amd64 / arm64 | DMG recommended, application ZIP available; native Intel/Apple Silicon release runners configured, manual native validation pending |

Each release lists its actually available assets and validation; configured
targets are not a claim that their packages have already been published.
Windows packages are not Authenticode signed; macOS packages are not Developer ID
notarized. SHA256SUMS verifies artifact integrity and does not replace OS signing.

On Windows, run the installer and launch from the Start menu. On macOS, open the
DMG, drag RcloneGo to Applications, eject the image and launch the installed app.
See [installation, updates and removal](docs/install.md) for detailed steps.
No-install Windows users can run the executable alone; the default user-data
directory still stores accounts and task history. SQLite is embedded and
extracted to a verified private cache on first use.

Open **Settings → rclone engine** to download the pinned official rclone release, select an
existing executable or import its verified ZIP offline. This does not modify PATH
or a package-manager installation. Mounts separately require WinFsp on Windows,
FUSE on Linux, or a compatible FUSE/engine combination on macOS.

The [Settings page](assets/screenshots/settings-light.png) contains appearance, interface language, login startup,
client updates and collapsible defaults for new tasks. **Resources** shows
current process usage and live bandwidth controls.
New profiles follow the system theme and language; manual choices persist.
Login startup and automatic update checks are opt-in in the alpha.

## First workflow

1. Open RcloneGo and install or select an engine.
2. Add an account or use local directories. Google Drive requires your own OAuth
   Client ID; see [account setup](docs/accounts.md).
3. Create a copy task, or save a directory workflow with final verification.
4. Observe the queue and read the execution report.

Task forms keep preview and submission actions visible while options scroll.
Run results show the completed steps, verification method and scope, with a
recovery action for unfinished work.

Closing a window keeps the service running when a tray is available. Use **Quit**
to stop the service. Without tray support, closing exits. Restart recovery
compares source and destination again; file-internal resume depends on the backend.

## CLI and Agents

```sh
rclonego --help
rclonego schema
rclonego status --json
rclonego engine install
rclonego workflows save --file workflow.json --json
rclonego workflows run <workflow-id> --json
rclonego runs wait <run-id> --timeout 1h --json
rclonego runs report <run-id> --json
```

The CLI discovers the same local instance as the GUI. For an isolated profile,
set `RCLONEGO_DATA`; `RCLONEGO_TOKEN_FILE` selects a private token file.
Ctrl+C stops waiting without cancelling the task.

See the [Agent guide](docs/agents.md), [API](docs/api.md),
[architecture](docs/architecture.md) and [privacy](docs/privacy.md).
See [automations](docs/automation.md), [mount caching](docs/mounts.md) and
[languages](docs/languages.md). Automatic copies retain destination-only files.
Bidirectional synchronization, separate-daemon GUI attachment and MCP remain
future work. The official MyGo native updater supports signed releases and
stable/preview channels; the release page identifies which builds/feed are available.
Renamed standalone Windows executables use manual updates.

## Develop and contribute

Requires Go 1.27.3+ and Python 3.12+. Native UI needs no JavaScript toolchain.

```sh
go run ./cmd/rclonego
go test ./internal/ui
go vet ./internal/ui
python scripts/build.py --platform windows/amd64
```

Use the affected packages in place of `internal/ui` for other changes.
`scripts/test.py` downloads a verified temporary integration engine. Add
`--installer` to the Windows build to create an installer. `-data`, `-rclone`
and `-config` select explicit private state, engine and config paths.
The default app configuration starts empty and does not take over existing uploads.
Routine changes use focused local checks. Pushes, pull requests and tags do not
start Actions; a maintainer explicitly dispatches the Release workflow for an
existing version tag when full platform packages are needed.

Read [CONTRIBUTING](CONTRIBUTING.md) for focused changes and UI verification,
[releasing](docs/releasing.md) for packaging, and [SECURITY](SECURITY.md) for private
security reports. Source uses [Apache-2.0](LICENSE); dependencies and service
marks retain their [own terms](docs/third-party.md).
License texts are embedded in the portable executable and available through
About or `rclonego licenses`.
