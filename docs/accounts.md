# Account configuration

Install or select rclone in Resources before adding an account. The account
screen discovers providers and their questions from that engine, so the form
follows the installed provider rather than a hardcoded Google-only flow.

The configuration session moves through `configuring`, `needs_input`,
`verifying`, then `completed`. A browser authorization may occur during
configuration. Complete the browser step and return to answer any remaining
provider questions. A browser redirect alone does not establish read access.

Google Drive creation requires your own OAuth client. Follow the official
[rclone client instructions](https://rclone.org/drive/#making-your-own-client-id),
then enter the Client ID and secret in the account form. RcloneGo does not
distribute a shared OAuth client.

Use reconnect for an existing account. Creation requires a new remote name.
Only one configuration session writes at a time. Sessions expire after twenty
minutes. Cancel stops its private configuration engine and discards the draft.
If another program changes the original configuration, RcloneGo preserves that
version and asks you to restart setup.

The draft is committed only after a directory read succeeds. Some providers
may require additional permissions or a provider-specific root before that
probe succeeds. Failed verification preserves the original configuration.

For a trusted local Agent:

```sh
rclonego accounts providers --json
rclonego accounts create --file account.json --json
rclonego accounts show <session-id> --json
rclonego accounts answer <session-id> --stdin --json
rclonego accounts cancel <session-id> --json
```

The answer command reads one JSON object from stdin, such as
`{"answer":"provider-answer"}`. Keep actual credentials out of shell arguments,
history, public workflow files and bug reports. Configuration responses expose
the current question and status, never an rclone config dump.
