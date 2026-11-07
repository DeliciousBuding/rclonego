# Architecture

RcloneGo separates presentation, transport and execution:

```text
cmd/rclonego
    ├── internal/ui        native views
    ├── internal/cli       human and JSON output; HTTP client
    └── internal/api       authenticated /api/v1
              │
         internal/core     shared application and queue
              ├── persistence   SQLite adapter
              └── rclone        supervised engine processes and RC
```

The current core includes platform and engine adapters in platform-specific
files. Further extraction follows real module boundaries; UI and API handlers
already call the same application service.

## Definitions and executions

A saved workflow defines source, destination, ordered top-level entries and
transfer settings. Each execution creates a run with its own ID and concrete
steps. A retry creates a new attempt while retaining the previous attempt.

The queue runs one step at a time. rclone controls file transfer and checking
concurrency. Queue position controls pending order; workflow dependency order
prevents a final check from bypassing unfinished steps.

Workflow sources currently must be local directories. Arbitrary DAGs and
remote-source workflow discovery are outside the current implementation.

## Persistence

SQLite WAL stores settings, workflows, runs, steps, attempts and state-change
events. A workflow is inserted transactionally. Persistence failures restore the
in-memory snapshot.

The current adapter replaces snapshot tables in a transaction and upserts
attempts. Incremental row updates, history retention and pagination are planned
before large historical workloads.

Startup marks previously running steps interrupted. A still-existing process
prevents duplicate dispatch; a stopped process allows the step to be retried.
Recovery uses rclone's source/destination comparison. File-internal resumability
depends on the backend and operation.

## Engine and process ownership

Transfers use a separate rclone process per step, with RC for stats and bandwidth
control. A dedicated config and per-instance RC credentials isolate the engine.
Inherited `RCLONE_*` environment settings are removed.

Only a mount recorded as belonging to this instance may be stopped. Discovery of
a visible drive alone does not grant ownership. Windows resource sampling shows
the application and its direct child processes.

## Verification

- `backend`: rclone check compares size and hashes where available; the report
  does not attest that every file had a common hash.
- `size`: explicit size-only comparison.
- `content`: `rclone check --download` compares content; it may require downloading
  remote content and consuming bandwidth.

Checks are one-way: extra destination files are outside the check scope.
Reports describe this scope and distinguish execution status from verification.
Attempt bytes measure transfer traffic, not unique dataset coverage.

## UI lifecycle

Closing the current GUI stops its service and interrupts an active transfer.
Headless `serve` mode runs without a window. Background/tray lifecycle and
reconnection to a separate daemon are planned.

The native UI retains a readable opaque content canvas with restrained window
material. Brand color and app glyph are separate from service logos and
success/warning/error colors.
