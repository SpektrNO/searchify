# Handoff archive: opt-relative-path-resolve

Completed 2026-08-07.

## Changes

- `config.AllowedPath`: relative paths resolve via `SEARCHIFY_PATH_BASE` (optional) then each `SEARCHIFY_ROOTS` entry; must exist and be unique; process CWD never used
- Absolute paths: unchanged allowlist check
- Ambiguous / missing / `..` escape: clear errors
- Reject path `.`
- MCP `search_file` schema path description updated
- Docs: README, `.env.example`, `.cursor/mcp.json.example`, architecture, CLI usage
- MCP server version 0.5.2

## Verification

- `go test ./...` (including `internal/config` path tests)
- `go build -o bin/searchify ./cmd/searchify`

## Manual remaining

- Reload Cursor MCP; try `search_file` with a repo-relative path after setting `SEARCHIFY_PATH_BASE`
