# Agent entry point

Use the same running RcloneGo instance as the human interface. Discover the
contract with `rclonego --help` and `rclonego schema`; both work offline. Then
read `status`, `doctor` and `engine status` before submitting work.

Prefer JSON output, stable idempotency keys and one saved workflow per recurring
operation. Start an execution with `workflows run`, observe it with `runs wait`
or `runs watch`, and use `runs report` to establish what was verified. Do not
treat transferred bytes as a completed content check.

Queue pause stops dispatch. Job pause interrupts one step and retains attempts.
Explicit resume/retry compares the source and destination again. Ctrl+C stops
CLI waiting without cancelling the underlying execution.

Use `events --after <cursor>` for incremental state changes, or
`events watch --after <cursor> --json` for newline-delimited event frames.
Events retain the newest 10000 records; `cursor_expired` (HTTP 410) requires
a current-state refresh before restarting at `after=0`. Persist the last
processed event sequence, not a timestamp. Run lists support `offset` and
`limit`; reports can read older history.

For a mirror, submit its parameters to `jobs preview`, inspect the result, then
submit the same parameters with `allow_delete` and the returned `preview_id`.
Read the structured added/changed/deleted summary and check
`report_incomplete`; destination-only entries in a copy are preserved.
The execution applies a deletion limit. Copy operations preserve extra target
files.

Browse a selected directory with `browse <absolute-local-path|remote:path>`.
Use `--directories` for directory selection and `--offset`/`--limit` for
pagination. A truncated listing requires a more specific directory.

Read-only observers can use `tokens read --ttl 3600`. For task execution,
the owner can use `tokens tasks --file <grant.json>` with explicit `operations`
and `paths`. Both transfer endpoints must match a path root. Task grants
submit and observe standalone jobs and can retry, pause or cancel matching
jobs; they cannot change accounts, engines, mounts, workflows or global state.
`tokens revoke <id>` revokes a grant immediately; all grants also expire and
end with the server session. Store the issued token in a private file and
select it using `RCLONEGO_TOKEN_FILE`.

Use `automations save --file <json>` for opt-in watch uploads or interval
execution of an existing workflow. Do not repeatedly submit a manual job
as an imitation of an automatic trigger. Automation state and queue jobs
share the same persistence and recovery semantics.

Check `/health`'s `service.ready` before new work. If persistence or a previous
owned process blocks dispatch, follow `service.next_action` and use
`queue recover` after resolving the cause. Treat all authenticated task paths
and logs as private.

Use `diagnostics --json` for a shareable aggregate report. It excludes names,
locations and logs. The [API](api.md) and [account](accounts.md) documents define
the supported operations; arbitrary rclone RC and shell execution are not
exposed.
