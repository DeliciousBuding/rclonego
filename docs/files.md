# Files and folders

The **Files** workspace browses configured rclone accounts, absolute local
directories and mounted drives. Opening **Browse files** from an account or
mount takes you to this workspace. The smaller directory picker remains
available in task and mount forms. It lists folders only and fills the current
form's path; selecting a folder does not submit a task.

## Navigation and selection

Use breadcrumbs, Back/Forward, or the location menu. Edit the path with its
edit button or Ctrl/Cmd+L, then press Enter. Deep paths keep the nearest three
levels visible; the ancestor menu provides earlier levels. Double-click
or Enter opens a folder; Backspace goes to its parent. Lists keep folders first
and show names, known sizes and modification times returned by the backend.
Search and sorting apply to the current page. They do not recursively search
cloud storage. Large folders use pagination; capped results are clearly marked.

Ctrl/Cmd-click and Shift-click select multiple entries. Ctrl/Cmd+C copies the
current entry's rclone or local path. Right-click exposes actions for that entry.
F2 opens Rename, Delete asks for confirmation, and Space previews a file.

## Preview and transfer

Text preview reads at most the first 256 KiB and requires UTF-8 text. PNG, JPEG,
GIF and WebP previews are limited to 8 MiB and 16 megapixels. Images show the
first frame. Unsupported files can be downloaded and opened in another app.
Preview fetches only on request and cancels when dismissed or replaced.
Reading failures show a short explanation and applicable actions such as retry,
account settings or download. Original error details remain available on demand.

**Upload files** opens the platform file picker. Files and folders dropped from
the system file manager are also accepted. **Download** saves selected entries
to a chosen local directory. **Copy to** selects a local or cloud destination.
Folders retain their own names at the destination.

The latest file operation shows how many items entered the queue. A failed item
does not stop later items from being submitted; inspect the failed items and
retry only those not added. Already queued items retain their task IDs and are
not submitted again. **View these tasks** opens their statuses and details in
the existing queue; **Return to Files** preserves the folder, search and page.
This temporary result is replaced by the next file operation or cleared on
restart. Tasks and attempts remain in durable history.

All transfers enter the same durable task queue, with its progress, cancellation,
retry and attempt history. Enqueuing is separate from completion. Copy retains
destination-only files; matching files follow rclone's comparison rules and may
be updated. Use the task page when you need a comparison preview or verification.

## File changes

New folder, Rename and Delete also use the task queue. Rename uses a different
name in the same directory, checks that the destination is absent, and does not
overwrite existing files. Folder rename uses rclone's supported directory move;
if the source remains after execution, the task reports failure rather than
claiming a complete rename.

The queued selection records its type, file size and available file modification time.
Execution rejects a changed selection so a delayed action requires a fresh
selection instead of silently acting on a modified file.

Delete requires confirmation and removes one file or an empty folder. It does
not move data to the recycle bin. Nonempty folders are rejected. No recursive
purge or arbitrary rclone command is exposed. Changes sharing a path with the
armed legacy handoff wait through the existing queue rules.

The visible folder refreshes when completed, failed or cancelled work could
have changed it, including writes through a mapped drive. Unrelated tasks,
verification and dry runs leave the current selection and listing in place.
Interrupted file changes remain available for inspection and explicit retry
after restart; they are not automatically replayed against a possibly changed path.

## API and CLI

`GET /api/v1/files/preview?path=...` returns text or a base64 image with its type,
size and truncation status. `POST /api/v1/files/actions` accepts:

```json
{"operation":"rename","path":"example:documents/old.txt","name":"new.txt"}
```

For deletion, add `"confirm_delete": true`; `"is_dir": true` selects empty-folder
removal. New folder takes the parent path and a single child `name`.
The response is the queued job. Owner credentials are required for changes;
delegated transfer grants cannot acquire file deletion permissions.

```text
rclonego files preview example:documents/notes.txt --json
rclonego files rename --file rename.json --json
rclonego mounts targets --json
```

The native interaction patterns were informed by
[Rclone UI](https://github.com/rclone-ui/rclone-ui),
[Rclone Browser NG](https://github.com/SysAdminDoc/RcloneBrowserNG) and
[RClone Manager](https://github.com/RClone-Manager/rclone-manager).
RcloneGo uses its own MyGo implementation and retains one task queue.
