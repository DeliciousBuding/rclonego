# Local API and CLI

The API listens on loopback and chooses an available port by default.
`instance.json` in the private data directory describes the endpoint.
`api-token` contains its random bearer token.

Except for `GET /api/v1/health`, requests require:

```http
Authorization: Bearer <local-token>
```

Mutations also require `X-RcloneGo-Client: 1`. Browser origins and redirects are
rejected. The owner token grants trusted local-user access. Use
`POST /api/v1/tokens/read` to create an expiring read-only token for observation.
Use `POST /api/v1/tokens/tasks` to delegate specific transfer operations within
explicit local or remote path roots. Grants are session-only, expire within
one day, and can be revoked by the owner without restarting the application.
Engine, account, mount, workflow and global queue configuration require the owner.

## Operations

| Endpoint | Result |
| --- | --- |
| `GET /api/v1` | Capabilities and constraints |
| `GET /api/v1/doctor` | Engine and platform diagnostics |
| `GET /api/v1/health` | Public liveness plus task-service readiness |
| `GET /api/v1/schema` | OpenAPI 3.1 contract |
| `GET /api/v1/engine` | Selected engine |
| `POST /api/v1/engine/install` | Verified recommended download or `archive` import |
| `POST /api/v1/engine/select` | Select a `path` |
| `GET /api/v1/providers` | Provider metadata |
| `POST /api/v1/accounts` | Begin account configuration |
| `GET /api/v1/accounts/{id}` | Current question and configuration status |
| `POST /api/v1/accounts/{id}/answer` | Submit `answer` |
| `POST /api/v1/accounts/{id}/cancel` | Discard draft |
| `GET /api/v1/overview` | Current activity and queue |
| `GET /api/v1/runtime?view=resources` | Client and external rclone process counters, with explicit ownership |
| `GET /api/v1/remotes/{name}/quota` | Provider account quota; omitted values are unknown |
| `GET /api/v1/remotes/{name}/info` | Backend capabilities and hash algorithms |
| `GET /api/v1/browse?path=example-remote:archive` | One directory, sorted entries and pagination |
| `GET /api/v1/storage/usage?path=example-remote:archive` | User-requested recursive file count and byte total |
| `GET /api/v1/jobs` | Steps/jobs and queue paused state |
| `POST /api/v1/jobs` | Create a job |
| `POST /api/v1/jobs/preview` | Dry-run preview and expiring confirmation ID |
| `GET /api/v1/jobs/{id}` | Job details |
| `GET /api/v1/mounts/{id}/cache` | Owned mount VFS cache and bounded writeback queue |
| `POST /api/v1/offline-copies` | Submit an independent persistent copy job |
| `POST /api/v1/logs/cleanup` | Apply configured task-log retention |
| `PUT /api/v1/jobs/{id}` | Pending parameter update |
| `POST /api/v1/jobs/action` | Retry, pause, cancel, delete or reorder |
| `POST /api/v1/queue/pause` | Set queue paused state |
| `POST /api/v1/queue/recover` | Recheck storage and recover interrupted dispatch |
| `GET /api/v1/workflow-definitions` | Reusable definitions |
| `POST /api/v1/workflow-definitions` | Save or update a definition |
| `DELETE /api/v1/workflow-definitions/{id}` | Remove definition, retain executions |
| `POST /api/v1/workflows` | Create a concrete directory workflow |
| `GET /api/v1/runs?offset=0&limit=100` | Newest executions first, maximum 500 per page |
| `POST /api/v1/runs` | Execute a saved definition |
| `GET /api/v1/runs/{id}/report` | Steps, attempts and verification result |
| `POST /api/v1/runs/{id}/{action}` | Pause, resume, retry or cancel |
| `GET /api/v1/events?after=0&limit=100` | State events and next cursor |
| `GET /api/v1/events/watch?after=0` | SSE state events with resumable sequence IDs |
| `POST /api/v1/tokens/read` | Create an expiring global read grant |
| `POST /api/v1/tokens/tasks` | Create an operation/path-scoped task grant |
| `POST /api/v1/tokens/{id}/revoke` | Revoke a delegated grant, including its open event stream |
| `GET /api/v1/automations` | Watch/interval triggers and their current state |
| `POST /api/v1/automations` | Create or update a trigger |
| `PUT /api/v1/automations/{id}` | Update a trigger with the matching ID |
| `DELETE /api/v1/automations/{id}` | Remove a trigger; running work remains under queue ownership |
| `POST /api/v1/automations/{id}/trigger` | Request one execution through the shared queue |
| `GET /api/v1/diagnostics` | Diagnostic export without names, paths, logs or credentials |

Job and concrete workflow submission support idempotency keys. For run
submission, pass `Idempotency-Key` or `idempotency_key`. Keep the key stable when
retrying a request after a network failure.

Quota comes from `rclone about --json`; `status: unsupported` means the provider
does not report a quota. Missing fields are unknown, while `free: 0` means no
free space. Google Drive can include other Google services in account usage.
Capacity reads never fall back to enumerating files.
When both total and free are reported, the GUI uses their difference for account
occupancy and retains the provider's file/other-service breakdown. The API
preserves the engine's original quota fields.

`accounts quota <name>` and `accounts info <name>` expose the same read-only
information through the CLI. `usage <path>` explicitly inventories a directory
and has a one-minute server timeout; canceled or failed scans return an error,
not a partial total. `sizeless` counts entries with unknown size. Storage reads
require owner or global read access; task grants do not expose account-wide data.

## Example workflow

```json
{
  "name": "Example backup",
  "source": "/path/to/source",
  "destination": "example-remote:archive",
  "entries": ["documents", "media", "notes.txt"],
  "verification_mode": "backend",
  "settings": {
    "transfers": 2,
    "checkers": 4,
    "chunk_size": "16M",
    "buffer_size": "8M",
    "cpu_parallelism": 2
  }
}
```

Local paths are interpreted on the computer running RcloneGo. Windows paths
must be escaped as JSON strings.

Use `verification_mode: "content"` only when content-download verification is
intended. It may read all bytes from both sides.

## Agent usage

Use `--json` for stable structured output. Use `runs wait` for completion and
`runs report` for evidence. A completed transfer does not imply that the
workflow's final check has finished.

`runs wait` returns zero only for a completed run. Stopping the wait command does
not cancel the run. Queue pause stops new dispatch; stopping the service
interrupts the active step.

Task pause interrupts that step and leaves it `paused`; recovery reruns rclone's
comparison. It does not promise file-internal byte resumability. `runs wait`
returns 4 when a paused run needs an explicit resume, 3 on timeout and 130 on
interruption of the wait.

Sync requires `allow_delete: true` and a matching, unexpired `preview_id` from
`jobs/preview`. The preview binds all execution parameters and expires after ten
minutes. It describes the current comparison; remote content may change before
execution. `max_delete` defaults to 10 and is enforced by rclone during execution.

Errors include `code`, `message`, `retryable` and `next_action`, with the legacy
`error` string retained. The same fields are preserved by CLI JSON errors.
Specific codes include `scope_denied`, `cursor_expired`, `timeout`, `cancelled`,
`filesystem_permission_denied` and `path_not_found`; service health describes
storage or previous-process blockers. `GET /health` can be live while
`service.ready` is false. Fix the reported cause before `queue recover`.
`rclonego schema` works offline and describes typed requests, responses,
pagination, mutation headers and the SSE response.

## Browsing and previews

`browse` accepts an absolute local directory or an explicit `remote:path`.
Use `offset`, `limit` (1–500) and `directories_only` to select a page.
`has_more` and `next_offset` describe the captured listing; `truncated` means
the scan limit was reached. Pages reflect the current directory and can
change when files are added or removed.

Preview results include the operation, source, destination, changed entries,
summary counts, expiry and report completeness. They use rclone's combined
comparison report. Destination-only files are distinguished from planned
deletions: copy preserves them, sync may delete them. `entries_truncated`
limits displayed detail; `report_incomplete` means the comparison evidence
cannot authorize a sync. A readable log remains available.

## Delegated task grants

The local owner creates a grant with:

```json
{
  "ttl_seconds": 3600,
  "operations": ["copy", "check"],
  "paths": ["/path/to/source", "example-remote:archive"]
}
```

The response includes `id`, `token`, `scope`, `expires_at`, the normalized
grant and `session_only`. Only the token digest is held by the server.
Each request requires the bearer token; each mutation also requires the
local-client header. The owner revokes by ID.

Task grants support standalone job submission/preview and matching job
retry, pause and cancel. Both endpoints of a transfer must be inside the
grant, and its operation must be allowed. Job lists and browse results are
filtered; ungranted parent directories are omitted. Relative local paths,
remote parent traversal, inline backend connection strings, workflow/proof
inputs and ungranted operations are rejected before submission.
Existing symlink ancestors are resolved during validation. Grants do not
grant OS permissions or control other programs running as the same user.
Read grants observe global state but cannot mutate it.

## Events

Polling returns `events`, `next_cursor`, `earliest_cursor` and `retention`.
`after=0` bootstraps from retained history. A nonzero cursor older than the
retained window returns HTTP 410 with `code: cursor_expired`; refresh current
task state before restarting with `after=0`.

SSE emits `cursor`, `event` and `error` frames. Event IDs are sequence numbers.
Reconnect with `after` or `Last-Event-ID`; `after` takes precedence.
Streams send keepalives while idle and close when a read grant expires or
is revoked. An in-stream error requires the same recovery as a polling error.

```sh
rclonego events watch --after 42 --json --timeout 1h
```

CLI watch output is newline-delimited JSON with `type`, numeric `cursor` and
`data`. The default watch duration is one hour. Ctrl+C exits with 130; an
expired watch duration exits with 3. Neither action cancels queued transfers.

## Automatic triggers

Watch triggers observe a local directory and submit one-way copies after
debounce and file stability checks. Interval triggers execute saved workflows.
Both use the same persistent queue, preserve trigger state across restarts,
and avoid overlapping executions of the same trigger.

```json
{
  "name": "Documents upload",
  "kind": "watch",
  "enabled": true,
  "source": "/path/to/source",
  "destination": "example-remote:archive",
  "debounce_seconds": 10,
  "stable_seconds": 10,
  "rescan_seconds": 3600
}
```

Creation and edits contain configuration fields only; generated timestamps,
fingerprints, reservation keys and execution state are not request inputs.
Watch preserves extra destination files. Bidirectional synchronization and
automatic deletion remain separate capabilities.

## Cache and offline copies

`mounts cache <id>` samples only a mount whose process identity belongs to this
instance. The response reports availability, disk and metadata counters,
writeback entries and truncation. Evictable VFS cache is not an offline backup.

`offline --file <copy.json>` accepts `name`, a configured `remote:path` source,
an absolute local `destination`, optional `file` and `idempotency_key`. A file
copy places the source filename in the selected directory. The destination
must be outside mount and VFS cache paths. It creates a normal queued copy with
retained task state. Offline-copy submission and
`logs cleanup` require the owner token. Delegated task grants can instead
submit a regular copy job within their permitted paths.
