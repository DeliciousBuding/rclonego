# RcloneGo

**Reliable transfers. Visible workflows.**

A native workspace for cloud transfers, verification and automation.
Powered by [rclone](https://rclone.org/). Built with [MyGo](https://mygo.egoist.dev/).

<img src="assets/icon.png" alt="RcloneGo" width="96">

RcloneGo is an independent community project. It is not an official rclone or
MyGo application.

## Development preview

This repository is under active development. There is no stable release yet.

The current preview includes:

- Native UI with light, dark and system appearance.
- A persistent queue for copy, single-file copy, one-way mirror and checks.
- Ordered directory workflows with a final verification step.
- Reusable workflow definitions, separate runs and retained execution attempts.
- Restart recovery, parameter controls and live transfer information.
- A loopback API and CLI that operate the same scheduler.
- An isolated application rclone configuration by default.
- SQLite transactions and state-change events.
- Verified official engine download, offline archive import and engine selection.
- Provider-driven account configuration sessions, browser OAuth and read verification.
- Tray background operation, explicit quit and user-level login startup.
- Dry-run confirmation for mirrors, task pause/resume and run-level controls.
- Offline OpenAPI discovery, event cursors and expiring read-only Agent tokens.
- A Windows executable with its SQLite runtime embedded.

Verification reports distinguish size checks, available backend hashes and
explicit content comparison. A completed transfer is not automatically a
completed content verification.

## Build and run

Requires Go 1.27.3 or later. A native UI build needs no JavaScript toolchain.
The UI can open without an engine. In **Resources**, download the recommended
rclone release, import its official ZIP offline or select an existing executable.

```sh
go test ./...
go vet ./...
python scripts/build.py
```

`scripts/build.py` embeds MyGo's verified platform SQLite library into the
executable. On first use it extracts the library to a private, content-addressed
cache. Windows portable users can copy the executable alone. rclone remains an
independently managed engine. Add `--installer` to produce a Windows installer.

For development:

```sh
go run ./cmd/rclonego
go run ./cmd/rclonego serve
go run ./cmd/rclonego --help
```

`-rclone` selects an engine, `-data` selects private application state and
`-config` explicitly selects an existing rclone configuration. Without `-config`,
RcloneGo creates an empty application configuration and does not import accounts.

## CLI

```sh
rclonego status
rclonego doctor
rclonego engine install
rclonego accounts providers --json
rclonego schema
rclonego jobs list --json
rclonego workflows save --file workflow.json
rclonego workflows list
rclonego workflows run <workflow-id> --json
rclonego runs report <run-id>
rclonego runs wait <run-id> --timeout 1h --json
```

The CLI discovers the local running instance. For a separate development profile,
set `RCLONEGO_DATA` to its state directory. An explicit API URL may be selected
with `--api` or `RCLONEGO_API`; `RCLONEGO_TOKEN_FILE` selects its private token file.
Authentication tokens should not be passed as command-line arguments.

See [API](docs/api.md), [architecture](docs/architecture.md) and
[privacy](docs/privacy.md). Account setup is described in [accounts](docs/accounts.md);
Agent usage is described in [agents](docs/agents.md).

## Platform status

| Platform | Current scope |
| --- | --- |
| Windows amd64 | Primary development target; native UI and synthetic transfer/recovery tests |
| Windows arm64 | Build target; native runtime validation pending |
| macOS amd64/arm64 | Experimental build targets; native runtime validation pending |
| Linux amd64 | Native UI inspected under Ubuntu WSLg; synthetic transfer and content verification tested |
| Linux arm64 | Build target; native runtime validation pending |

Windows mounts require WinFsp. Linux mounts use FUSE; macOS mounts require a
compatible FUSE installation and engine build. POSIX mount management is
implemented; native macOS mount validation is pending. Resource counters are
scoped to this application and its direct children. Unavailable counters are
reported explicitly. Normal transfers do not require mounting.

## Verification

`go test ./...` includes a real local transfer/recovery test when rclone is
available. Set `RCLONEGO_TEST_ENGINE` to test a particular engine. That test uses
temporary synthetic directories and an empty configuration.
`python scripts/test.py` downloads and verifies a temporary integration engine,
then runs the suite. Linux CI also runs the race detector.

## License

RcloneGo source is licensed under [Apache-2.0](LICENSE).
Dependency licenses and service trademarks retain their own terms; see
[third-party notices](docs/third-party.md).
