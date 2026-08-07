# Searchify

Hybrid text search MCP server written in Go. Search local files, indexed corpora, and the web via LangSearch.

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

Phase 5 (HTTP + hardening):

- `searchify serve http` — Streamable HTTP MCP with Bearer auth, timeouts, `/healthz`
- `search_web` / `search_local` / `search_file` / `index_*` — unchanged over stdio and HTTP
- Benchmarks: `make bench` (keyword + hybrid, report-only)

Stdio remains the recommended Cursor transport for local use.

## Requirements

- Go 1.25+ (required by MCP Go SDK)
- `SEARCHIFY_ROOTS` environment variable

## Build

```bash
go mod tidy
go build -o bin/searchify ./cmd/searchify
# or: make build
```

## Benchmarks

Report-only latency for local keyword/hybrid (stub embeddings, ~1.2k docs):

```bash
make bench
# or: go test -bench=BenchmarkSearch -benchmem ./internal/local/
```

## Run (stdio MCP)

```bash
export SEARCHIFY_ROOTS="/home/you/dev"
./bin/searchify mcp stdio
```

## Run (HTTP MCP)

Requires a non-empty `SEARCHIFY_HTTP_TOKEN`. Prefer binding to localhost; put TLS termination on a reverse proxy if exposing beyond the machine.

```bash
export SEARCHIFY_ROOTS="/home/you/dev"
export SEARCHIFY_HTTP_TOKEN="dev-secret"
./bin/searchify serve http --addr 127.0.0.1:8080 --path /mcp
# GET http://127.0.0.1:8080/healthz → ok
```

Remote MCP config (illustrative; Cursor versions differ — prefer stdio for local):

```json
{
  "mcpServers": {
    "searchify-http": {
      "url": "http://127.0.0.1:8080/mcp",
      "headers": {
        "Authorization": "Bearer dev-secret"
      }
    }
  }
}
```

HTTP mode uses go-sdk **stateless** Streamable HTTP for simpler proxying.

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
| `LANGSEARCH_API_KEY` | for web/rerank | LangSearch API key (`search_web` and local `rerank`) |
| `SEARCHIFY_HTTP_TOKEN` | for HTTP | Bearer token (required to start `serve http`) |
| `SEARCHIFY_HTTP_ADDR` | no | Default listen address (`:8080`) |
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

### `search_web`

Internet search via LangSearch. Requires `LANGSEARCH_API_KEY`. Results are cached in-memory for ~15 minutes.

```json
{
  "query": "Go MCP Streamable HTTP transport",
  "limit": 5,
  "freshness": "oneMonth",
  "summary": true
}
```

`freshness`: `oneDay` | `oneWeek` | `oneMonth` | `oneYear` | `noLimit` (default). `summary` defaults to `true` for LLM-friendly snippets.

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

Returns configured roots, index directory, vector readiness, and `langsearch_configured`.

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
