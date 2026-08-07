# Handoff archive: opt-auto-index-watch

Completed 2026-08-07 (lean-implement).

## Changes

- `SEARCHIFY_WATCH_PATHS` / `SEARCHIFY_WATCH_DEBOUNCE` / `SEARCHIFY_WATCH_RESCAN`
- `local.IndexWatcher` (fsnotify + debounce; create/write → index, remove/rename → remove)
- Starts with `mcp stdio` and `serve http`; `Server.Close` stops watcher
- `index_status`: `watch_enabled`, `watch_paths`
- MCP server version 0.6.2

## Verification

- `go test ./...` (including watcher integration test)
- `go build -o bin/searchify ./cmd/searchify`
