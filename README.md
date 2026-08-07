# Searchify

Hybrid text search MCP server written in Go. Search local files, indexed corpora, and the web via LangSearch.

## Docs

- [Architecture](docs/architecture.md)
- [Feature backlog](docs/feature-backlog.md)
- [Feature completed](docs/feature-completed.md)
- [GitHub workflow](docs/github-workflow.md)

### Spec → implement pipeline

Track features in `docs/feature-backlog.md`. Handoffs live in `docs/handoffs/current.md`. GitHub issue lifecycle: [docs/github-workflow.md](docs/github-workflow.md).

```bash
# Create epic/feature/task issues from open backlog rows
./scripts/create-feature-issues.sh --dry-run
./scripts/create-feature-issues.sh --only <feature-id>

# Resolve parent + ordered tasks
./scripts/load-feature-issue.sh <feature-id>

./scripts/github-issue-status.sh in-progress <task#>
./scripts/record-feature-complete.sh <feature-id> --issue <n> --note "PR #…"
```

Cursor shortcuts: `/spec-only`, `/implement-handoff`, `/spec-and-implement`, `/lean-implement` (see `.cursor/skills/spec-and-implement/SKILL.md`).

## Status

Phase 5 (HTTP + hardening):

- `searchify serve http` — Streamable HTTP MCP with Bearer auth, timeouts, `/healthz`, plus REST `POST /v1/search` and `POST /v1/index`
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

### REST API (same server)

Same Bearer token. Prefer localhost / reverse proxy.

```bash
# Index
curl -sS -X POST http://127.0.0.1:8080/v1/index \
  -H "Authorization: Bearer dev-secret" \
  -H "Content-Type: application/json" \
  -d '{"paths":["/abs/path/to/docs"],"force":false}'

# Search (twin of search_local)
curl -sS -X POST http://127.0.0.1:8080/v1/search \
  -H "Authorization: Bearer dev-secret" \
  -H "Content-Type: application/json" \
  -d '{"query":"path allowlist","mode":"hybrid","limit":10}'
```

| Method | Path | Body |
|--------|------|------|
| `POST` | `/v1/search` | `query`, optional `limit`, `mode`, `rerank` |
| `POST` | `/v1/index` | `paths`, optional `force` |

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
| `SEARCHIFY_PATH_BASE` | no | Preferred base for relative tool/CLI paths (must be under a root) |
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

Re-indexing does **not** remove deleted files from the index. Use `remove_paths` for known deletes, or `index_prune` to reconcile orphans vs disk.

### `remove_paths`

Remove files or directories from the local index (FTS chunks, vectors, and `files` rows). Paths must stay under `SEARCHIFY_ROOTS` but **need not exist on disk**. A directory path drops that path and all indexed children.

```json
{
  "paths": ["/abs/path/to/deleted.md", "docs/old"]
}
```

### `index_prune`

Reconcile the index with disk: drop rows for files that no longer exist, and for indexed paths outside current `SEARCHIFY_ROOTS`. Optional `paths` limits the scan; `dry_run` reports without deleting. Prefer `dry_run` first if mounts may be flaky.

```json
{
  "paths": [],
  "dry_run": true
}
```

CLI: `searchify prune [--dry-run] [paths...]`

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

Responses include `duration_ms` (wall clock). `search_local` also returns a `timing` breakdown (`keyword_ms` / `vector_ms` / `rrf_ms` / `rerank_ms`) for the legs that ran.

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

Search within one file under allowed roots. Paths may be absolute (must stay under a root) or relative: Searchify resolves them against `SEARCHIFY_PATH_BASE` (if set) then each root — no process CWD. Relative paths must exist and be unique; ambiguous matches return an error.

```json
{
  "path": "internal/config/config.go",
  "query": "AllowedPath",
  "limit": 10
}
```

Set `SEARCHIFY_PATH_BASE` to your project root in MCP env when using repo-relative paths (Cursor often passes those).

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
