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

Use `events --after <cursor>` for incremental state changes. Events retain the
newest 10000 records; refresh current status if a cursor falls outside that
window. Run lists support `offset` and `limit`; reports can read older history.

For a mirror, submit its parameters to `jobs preview`, inspect the result, then
submit the same parameters with `allow_delete` and the returned `preview_id`.
The execution applies a deletion limit. Copy operations preserve extra target
files.

Read-only observers can use a delegated token created by the local owner at
`POST /api/v1/tokens/read`. Store the token in a private file and select it using
`RCLONEGO_TOKEN_FILE`. Configuration and task mutations require the owner token.
Treat all authenticated task paths and logs as private.

Use `diagnostics --json` for a shareable aggregate report. It excludes names,
locations and logs. The [API](api.md) and [account](accounts.md) documents define
the supported operations; arbitrary rclone RC and shell execution are not
exposed.
