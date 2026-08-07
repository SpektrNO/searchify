# Handoff archive: opt-remove-path

Completed 2026-08-07.

## Changes

- MCP tool `remove_paths` and CLI `searchify remove`
- `local.RemovePaths`: delete `files` row + FTS chunks + vectors (exact path or children under `P/`)
- `config.AllowlistedCandidates`: allowlist without requiring disk existence; relative joins + index disambiguation
- Idempotent skip when not in index; outside-root counted as error
- Docs: README, architecture
- MCP server version 0.5.3

## Verification

- `go test ./...`
- `go build -o bin/searchify ./cmd/searchify`

## Manual remaining

- Reload Cursor MCP; try `remove_paths` after deleting an indexed file on disk
