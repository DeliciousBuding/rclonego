# Automatic copies and recurring workflows

Automations are opt-in triggers for RcloneGo's existing queue. They share queue
ordering, pause, task recovery and resource settings with manual work.

Two trigger kinds are supported:

- `watch` copies an absolute local source directory to a configured remote or
  another absolute local directory. It never deletes extra destination files.
- `interval` runs an existing saved workflow at a fixed interval, including that
  workflow's final verification step.

New definitions are disabled unless `enabled` is explicitly `true`. A manual
trigger can run a disabled definition once.

## Folder observation

RcloneGo uses [fsnotify](https://github.com/fsnotify/fsnotify) for filesystem
notifications and keeps periodic full scans enabled, following the
[watcher plus rescan model](https://docs.syncthing.net/users/syncing.html).
Notifications alone can miss changes after a watcher overflow or on some
filesystems.

Changes are coalesced for three seconds by default. Before submitting a copy,
two observations must show a stable metadata baseline for at least two seconds.
The default periodic scan interval is five minutes.

The baseline uses relative paths, sizes and modification times, matching ordinary
rclone copy comparison. It does not promise detection of content changes that
preserve both size and time. Symbolic links are not followed. Write stability is
an observation delay, not an exclusive lock on files.

Each scan is bounded to 100,000 entries, 4,096 directories and ten seconds.
At most 32 definitions are supported. A limit or scan failure is shown as
`attention` and requires an explicit run or configuration correction. If
filesystem notifications are unavailable, periodic scanning continues and the
definition exposes `watch_warning`.

## Dispatch and recovery

An automation will not submit a second task while its previous task is queued,
running, paused or interrupted. Changes during an active copy are reconciled
after it completes. Manual run requests received while busy are coalesced into
one later run.

Before dispatch, a persistent idempotency key reserves the trigger. Restarting
between reservation and queue submission reuses that key. It does not create
another copy of the task.

Missed interval ticks are coalesced rather than replayed as a backlog. The next
interval begins after the previous run completes. Failed or cancelled tasks
require an explicit action; they do not generate a retry storm.

Deleting a definition removes its future triggers and leaves already submitted
queue tasks and execution history intact.

## Examples

An automatic one-way copy:

```json
{
  "name": "Documents backup",
  "kind": "watch",
  "enabled": true,
  "source": "/path/to/documents",
  "destination": "example-remote:backup/documents",
  "debounce_seconds": 3,
  "stable_seconds": 2,
  "rescan_seconds": 300
}
```

A recurring saved workflow:

```json
{
  "name": "Hourly verified backup",
  "kind": "interval",
  "enabled": true,
  "workflow_id": "saved-workflow-id",
  "interval_seconds": 3600
}
```

Windows local paths must be absolute and JSON-escaped. Source and local
destination must not overlap. The source must also be outside the application's
private state directory and must not contain it, so state writes cannot trigger
an endless automatic-copy loop.

This release does not add automatic deletion, remote-side file watching,
two-way bisync or conflict resolution. Use a manually previewed sync when
destination deletion is intended.
