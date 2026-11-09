# RcloneGo development

- RcloneGo is a reliable transfer and verification workspace. Add capabilities
  for demonstrated workflows; prefer a useful default and an existing command
  surface before adding an option or page.
- Use MyGo native UI and its keyed, virtualized collections. Presentation
  decisions live in `docs/design.md`; execution boundaries in
  `docs/architecture.md`. Keep UI, HTTP transport and application behavior separate.
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
- Keep disk/network/process work outside view construction. Cancel obsolete
  reads and retain only bounded transient UI state. Profile an observed cost
  before adding a cache, service or new dependency.
- When a flow changes, consolidate duplicated commands and documentation;
  preserve essential actions, error evidence and keyboard accessibility.
