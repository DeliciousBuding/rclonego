# Local API and CLI

The API listens on loopback and chooses an available port by default.
`instance.json` in the private data directory describes the endpoint.
`api-token` contains its random bearer token.

Except for `GET /api/v1/health`, requests require:

```http
Authorization: Bearer <local-token>
```

Mutations also require `X-RcloneGo-Client: 1`. Browser origins and redirects are
rejected. The token grants trusted local-user access; granular directory and
operation scopes are planned. Do not share the token with an untrusted client.

## Operations

| Endpoint | Result |
| --- | --- |
| `GET /api/v1` | Capabilities and constraints |
| `GET /api/v1/doctor` | Engine and platform diagnostics |
| `GET /api/v1/overview` | Current activity and queue |
| `GET /api/v1/jobs` | Steps/jobs and queue paused state |
| `POST /api/v1/jobs` | Create a job |
| `GET /api/v1/jobs/{id}` | Job details |
| `PUT /api/v1/jobs/{id}` | Pending parameter update |
| `POST /api/v1/jobs/action` | Retry, cancel, delete or reorder |
| `POST /api/v1/queue/pause` | Set queue paused state |
| `GET /api/v1/workflow-definitions` | Reusable definitions |
| `POST /api/v1/workflow-definitions` | Save a definition |
| `POST /api/v1/workflows` | Create a concrete directory workflow |
| `GET /api/v1/runs` | Execution records |
| `POST /api/v1/runs` | Execute a saved definition |
| `GET /api/v1/runs/{id}/report` | Steps, attempts and verification result |

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

OpenAPI, event streaming, scoped tokens and structured retryable errors remain
planned; the current API uses JSON error messages.
