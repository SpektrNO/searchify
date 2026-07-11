# Searchify

Hybrid text search MCP server written in Go. Search local files and (soon) indexed corpora and the web via LangSearch.

## Docs

- [Architecture](docs/architecture.md)
- [Feature backlog](docs/feature-backlog.md)
- [Feature completed](docs/feature-completed.md)
- [GitHub workflow](docs/github-workflow.md)

### Spec → implement pipeline

Track features in `docs/feature-backlog.md`. Handoffs live in `docs/handoffs/current.md`.

```bash
./scripts/github-issue-status.sh in-progress <task#>
./scripts/record-feature-complete.sh <feature-id> --issue <n> --note "PR #…"
```

When ready for GitHub issues, copy `create-feature-issues.sh` and `load-feature-issue.sh` from the Relativity reference repo (see `setup-project-scaffolding` skill).

## Status

Phase 1 scaffolding:

- MCP server over stdio (Cursor / Claude Desktop)
- `search_file` tool — keyword search within a single file
- `index_status` tool — index readiness stub

Coming next: local indexing, hybrid BM25+vector search, web search, HTTP transport.

## Requirements

- Go 1.25+ (required by MCP Go SDK)
- `SEARCHIFY_ROOTS` environment variable

## Build

```bash
go mod tidy
go build -o bin/searchify ./cmd/searchify
```

## Run (stdio MCP)

```bash
export SEARCHIFY_ROOTS="/home/you/dev"
./bin/searchify mcp stdio
```

## Cursor configuration

Copy [`.cursor/mcp.json.example`](.cursor/mcp.json.example) into your Cursor MCP settings and adjust paths.

## Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `SEARCHIFY_ROOTS` | yes | Comma-separated allowed directory roots |
| `SEARCHIFY_INDEX_DIR` | no | Index path (default: `~/.searchify/index`) |
| `LANGSEARCH_API_KEY` | no | LangSearch API key (web search / rerank) |
| `SEARCHIFY_HTTP_TOKEN` | no | Bearer token for HTTP mode |
| `SEARCHIFY_EMBED_MODEL` | no | Embedding model (default: `minilm-l6-v2`) |

## MCP tools

### `search_file`

Search within one file under allowed roots.

```json
{
  "path": "internal/config/config.go",
  "query": "AllowedPath",
  "limit": 10
}
```

### `index_status`

Returns configured roots, index directory, and whether a local index exists.

## Project layout

```
cmd/searchify/          CLI entrypoint
internal/config/        Environment and path allowlist
internal/mcp/           MCP server and tool handlers
internal/file/          Single-file keyword search
internal/search/        Shared types
internal/local/         Local index (phase 2+)
internal/web/           LangSearch client (phase 4+)
internal/rank/          Reranking helpers (phase 3+)
```
