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
Delegated tokens are session-only and cannot mutate tasks or configuration.
Directory-scoped task permissions remain future work.

## Operations

| Endpoint | Result |
| --- | --- |
| `GET /api/v1` | Capabilities and constraints |
| `GET /api/v1/doctor` | Engine and platform diagnostics |
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
| `GET /api/v1/jobs` | Steps/jobs and queue paused state |
| `POST /api/v1/jobs` | Create a job |
| `POST /api/v1/jobs/preview` | Dry-run preview and expiring confirmation ID |
| `GET /api/v1/jobs/{id}` | Job details |
| `PUT /api/v1/jobs/{id}` | Pending parameter update |
| `POST /api/v1/jobs/action` | Retry, pause, cancel, delete or reorder |
| `POST /api/v1/queue/pause` | Set queue paused state |
| `GET /api/v1/workflow-definitions` | Reusable definitions |
| `POST /api/v1/workflow-definitions` | Save or update a definition |
| `DELETE /api/v1/workflow-definitions/{id}` | Remove definition, retain executions |
| `POST /api/v1/workflows` | Create a concrete directory workflow |
| `GET /api/v1/runs?offset=0&limit=100` | Newest executions first, maximum 500 per page |
| `POST /api/v1/runs` | Execute a saved definition |
| `GET /api/v1/runs/{id}/report` | Steps, attempts and verification result |
| `POST /api/v1/runs/{id}/{action}` | Pause, resume, retry or cancel |
| `GET /api/v1/events?after=0&limit=100` | State events and next cursor |
| `GET /api/v1/diagnostics` | Diagnostic export without names, paths, logs or credentials |

Job and concrete workflow submission support idempotency keys. For run
submission, pass `Idempotency-Key` or `idempotency_key`. Keep the key stable when
retrying a request after a network failure.

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
`rclonego schema` works offline. Event streaming is future work; cursor reads
avoid retrieving the same history repeatedly.
