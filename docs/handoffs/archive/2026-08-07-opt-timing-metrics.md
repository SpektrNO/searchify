# Handoff archive: opt-timing-metrics

Completed 2026-08-07.

## Changes

- `duration_ms` on `search_file`, `search_local`, `search_web`, `index_paths`
- `search_local` `timing` breakdown with pointer fields (zeros preserved for used legs)
- Hybrid leg timings from `internal/local`
- slog `tool complete` logs (tool, duration_ms, mode) without query text
- CLI `searchify index` prints `duration_ms=N`
- MCP server version 0.5.1
