# Privacy

RcloneGo runs locally. It does not send telemetry or file names to an analytics
service. Transfer requests are sent to the storage providers configured by the
user through rclone.

Application state, paths, logs, credentials and API tokens belong in private
user-data directories, outside the source repository. An existing rclone
configuration is used only when explicitly selected.

The local API does not expose rclone configuration dumps or OAuth tokens.
Task paths and logs are available to authenticated trusted clients.

The current development preview does not include automatic diagnostic export.
Logs and screenshots can contain private file names and paths. Review and redact
them before opening a public issue.

Google Drive/Photos setup should use a user-owned OAuth Client ID following the
[rclone instructions](https://rclone.org/drive/#making-your-own-client-id).
RcloneGo does not distribute a shared project OAuth secret.

Repository examples and verification fixtures use synthetic data. Build output,
runtime databases, configurations, private notes and credentials are ignored.
Source privacy checks complement this ignore list.
