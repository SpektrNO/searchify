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

Phase 3 (hybrid local search):

- `index_paths` — incrementally index + embed chunks (kjarni-go)
- `search_local` — keyword, vector, or hybrid search with optional LangSearch rerank
- `index_status` — document/chunk/vector counts and readiness
- CLI: `searchify index [--force] <paths...>`

Also available: `search_file` (single-file keyword search).

Coming next: web search (phase 4), HTTP transport (phase 5).

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

## Index and search (CLI)

```bash
export SEARCHIFY_ROOTS="/home/you/dev/spektr/searchify"
export SEARCHIFY_INDEX_DIR="/tmp/searchify-index"   # optional

./bin/searchify index docs/
# indexed=N updated=N skipped=N errors=N
```

Then use MCP tools `search_local` and `index_status` from Cursor.

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

### `index_paths`

Index or refresh files under one or more paths (must be under `SEARCHIFY_ROOTS`).

```json
{
  "paths": ["docs"],
  "force": false
}
```

### `search_local`

Search the persisted index. Default mode is `hybrid` when vectors exist, otherwise `keyword`.

```json
{
  "query": "how does path allowlisting work",
  "mode": "hybrid",
  "limit": 10,
  "rerank": false
}
```

Modes: `keyword` (FTS5 BM25), `vector` (cosine similarity), `hybrid` (RRF fusion). Set `rerank: true` to reorder results with LangSearch (requires `LANGSEARCH_API_KEY`).

### Upgrading from phase 2

Schema v2 adds vector storage. Existing keyword indexes keep working. Run `index_paths` with `"force": true` (or `searchify index --force`) once to embed all chunks.

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
