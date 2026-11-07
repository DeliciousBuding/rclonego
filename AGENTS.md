# RcloneGo development

- The product uses MyGo native UI. Preserve the current presentation while
  separating application behavior from UI and HTTP transport.
- GUI, CLI and API must share the same task, run and attempt semantics.
- Use temporary synthetic data and a separate rclone config for verification.
  Do not modify a developer's running transfers or existing mount owners.
- Source, docs, examples and release resources are public. Keep private plans
  in `.local/`; keep runtime state, credentials and logs outside tracked files.
- Validate relevant behavior with `go test ./...`, `go vet ./...` and
  `python scripts/check_public.py --tracked`. Native UI changes also need
  inspection of a real rendered window.
- Distinguish build success, actual platform runtime validation and published
  release status. Update public capability documentation when behavior changes.
- Do not add a second scheduler, arbitrary shell API or unbounded event/history
  cache. Implement supported backend capabilities explicitly.
