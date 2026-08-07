# Handoff archive: opt-index-prune

Completed 2026-08-07.

## Changes

- MCP tool `index_prune` and CLI `searchify prune [--dry-run] [paths...]`
- `local.PruneIndex`: scan indexed paths; drop missing-on-disk and out-of-root orphans
- `dry_run` reports would-remove without mutating DB
- Optional path scope (same prefix rules as remove_paths)
- `config.UnderAnyRoot` helper
- Docs: README, architecture
- MCP server version 0.6.1

## Verification

- `go test ./...`
- `go build -o bin/searchify ./cmd/searchify`

## Follow-ups

- `POST /v1/prune` REST
