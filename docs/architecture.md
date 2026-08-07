# Searchify architecture

Hybrid text-search MCP server in Go.

## Components

| Layer | Package | Role |
|-------|---------|------|
| CLI | `cmd/searchify` | `mcp stdio`, `serve http`, `index` subcommands |
| MCP | `internal/mcp` | Tool registration; stdio + Streamable HTTP |
| Config | `internal/config` | Env vars, path allowlist |
| File search | `internal/file` | Single-file keyword scan |
| Local index | `internal/local` | SQLite FTS5 + chunk vectors (phase 2–3) |
| Web | `internal/web` | LangSearch web search + shared HTTP client (phase 4) |
| Rank | `internal/rank` | RRF fusion; rerank delegates to `internal/web` (phase 3–4) |

## Storage (phase 2–3)

- Index database: `{SEARCHIFY_INDEX_DIR}/index.db`
- Lexical: SQLite FTS5 via `modernc.org/sqlite` (pure Go)
- Vectors: `chunk_vectors` table (float32 embeddings from kjarni-go)
- Embeddings: in-process via `SEARCHIFY_EMBED_MODEL` (default `minilm-l6-v2`)
- Incremental updates keyed on file `mtime` + `size`; vectors updated with re-indexed files

## MCP tools

| Tool | Phase | Description |
|------|-------|-------------|
| `search_file` | 1 | Keyword search in one file |
| `index_status` | 1 | Index readiness and config |
| `index_paths` | 2 | Build/update local index |
| `remove_paths` | opt | Remove files/dirs from index (FTS + vectors) |
| `index_prune` | opt | Drop orphans missing on disk / outside roots |
| `search_local` | 2–3 | Query persisted index |
| `search_web` | 4 | Internet search via LangSearch |

## Web search (phase 4)

- Client: `internal/web.Client` (shared by `search_web` and local rerank)
- Endpoint: `POST https://api.langsearch.com/v1/web-search`
- In-memory TTL cache (~15 minutes, max 128 entries)
- HTTP 429: exponential backoff with jitter (up to 3 retries)

## Search modes

- **keyword** — BM25 / line scan
- **vector** — embedding similarity
- **hybrid** — parallel retrieval + RRF fusion

## External APIs

- [LangSearch Web Search](https://docs.langsearch.com/api/web-search-api)
- [LangSearch Semantic Rerank](https://docs.langsearch.com/api/semantic-rerank-api)

## HTTP transport (phase 5)

- CLI: `searchify serve http [--addr] [--path]`
- go-sdk `NewStreamableHTTPHandler` with **stateless** sessions
- Auth: `Authorization: Bearer` must match `SEARCHIFY_HTTP_TOKEN` (required to start)
- Probes: `GET /healthz` (no auth)
- Timeouts: `ReadHeaderTimeout` 5s; ~60s request budget
- TLS: not terminated in-process — put a reverse proxy in front when exposing beyond localhost (see backlog `opt-tls-reverse-proxy`)
- App integration: MCP Streamable HTTP plus REST `POST /v1/search` + `POST /v1/index` (same Bearer token); optional REST remove/status later
- Deletions: MCP/CLI `remove_paths` / `searchify remove`; reconcile orphans with `index_prune` / `searchify prune` (`opt-index-prune`)
- Auto-index: optional watch/rescan (`opt-auto-index-watch`)
- Path UX: relative paths resolve via `SEARCHIFY_PATH_BASE` then roots (no process CWD); absolute paths must stay under roots
- File types: richer extractors beyond the extension allowlist (`opt-richer-file-types`)
- Observability: search/index tool responses include `duration_ms`; `search_local` adds per-leg `timing` (backlog was `opt-timing-metrics`)

## Optional scale

- Vector search is brute-force cosine today; ANN/HNSW is optional when corpora grow (backlog `opt-hnsw-vectors`)
- Storage stays SQLite FTS5 + blob vectors by default; optional store adapter (backlog `opt-store-adapter`) would select backend via config (e.g. `SEARCHIFY_STORE=sqlite|postgres`) so PostgreSQL + pgvector can replace the local DB without changing MCP tools

## Environment

See [README.md](../README.md#environment-variables).
