# Privacy

RcloneGo runs locally. It does not send telemetry or file names to an analytics
service. Transfer requests are sent to the storage providers configured by the
user through rclone.

Application state, paths, logs, credentials and API tokens belong in private
user-data directories, outside the source repository. An existing rclone
configuration is used only when explicitly selected.

The local API does not expose rclone configuration dumps or OAuth tokens.
Task paths and logs are available to authenticated trusted clients.

`rclonego diagnostics --json` exports counters and aggregate state without file
paths, account names, logs, credentials, environment variables or process
command lines. Review any material you add to it before sharing.
Logs and screenshots can contain private file names and paths. Review and redact
them before opening a public issue.

Google Drive/Photos setup should use a user-owned OAuth Client ID following the
[rclone instructions](https://rclone.org/drive/#making-your-own-client-id).
RcloneGo does not distribute a shared project OAuth secret.
Configuration sessions use isolated private drafts. Secrets are supplied in
authenticated local HTTP bodies; the CLI reads sensitive answers from stdin.
Cancelled sessions preserve the original configuration.

Repository examples and verification fixtures use synthetic data. Build output,
runtime databases, configurations, private notes and credentials are ignored.
Private plans and real transfer evidence stay in the ignored `.local/` directory;
public documentation and PR descriptions explain product behavior with sample
data. Never copy an installed profile into an example or build directory.
Source privacy checks complement the ignore list by checking text and filenames.
They do not inspect image pixels, metadata, previous commits or public comments.
Screenshots need visual review; removing a file from the current tree does not
remove earlier public versions.
