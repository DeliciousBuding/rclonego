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
              ├── engine        verified installation and selection
              └── rclone        supervised processes and private RC
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

Only changed rows are written in a transaction. Events retain the newest 10000
records. Startup loads active work, the newest 200 runs and 500 standalone jobs;
older history remains in SQLite and can be read through paginated run queries
and reports. This bounds startup history loading without deleting older reports.

Startup marks previously running steps interrupted. A still-existing process
prevents duplicate dispatch; a stopped process allows the step to be retried.
Recovery uses rclone's source/destination comparison. File-internal resumability
depends on the backend and operation.

## Engine and process ownership

Transfers use a separate rclone process per step, with RC for stats and bandwidth
control. A dedicated config and per-instance RC credentials isolate the engine.
Inherited `RCLONE_*` environment settings are removed.

Only a mount recorded as belonging to this instance may be stopped. Discovery of
a visible drive alone does not grant ownership. PID and process start identity
must both match before stopping it. Resource sampling shows the application and
its direct children, with availability flags for counters.

The engine installer accepts pinned official archives, verifies SHA-256, rejects
unsafe archive members and stages a complete version before selection. Existing
transfers keep their captured engine; later tasks use the new selection.

Account setup uses one configuration writer and an isolated draft. Private RC
request bodies carry configuration answers; secrets do not appear in process
arguments or application logs. Read verification and an unchanged original
configuration are required before committing the draft. Cancellation discards
the draft and stops its configuration process.

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

Closing a window hides it when a tray is available; the queue and local service
continue. The tray provides show, queue pause/resume and explicit quit. Without
tray support, closing the window exits the service. Login startup uses MyGo's
native user-level integration. Headless `serve` runs without a window.
Attaching a GUI to an independently started daemon remains future work.

The native UI retains a readable opaque content canvas with restrained window
material. Brand color and app glyph are separate from service logos and
success/warning/error colors.
Scroll content reserves a gutter for overlay scrollbars. Long row labels
truncate with full-value tooltips. At narrow widths, task details occupy the
content pane so the queue and inspector do not compete for horizontal space.
