# Searchify architecture

Hybrid text-search MCP server in Go.

## Components

| Layer | Package | Role |
|-------|---------|------|
| CLI | `cmd/searchify` | `mcp stdio`, `serve http`, `index` subcommands |
| MCP | `internal/mcp` | Tool registration and transports |
| Config | `internal/config` | Env vars, path allowlist |
| File search | `internal/file` | Single-file keyword scan |
| Local index | `internal/local` | SQLite FTS5 persisted keyword index (phase 2) |
| Web | `internal/web` | LangSearch client (phase 4) |
| Rank | `internal/rank` | RRF fusion, rerank helpers (phase 3) |

## Storage (phase 2)

- Index database: `{SEARCHIFY_INDEX_DIR}/index.db`
- Engine: SQLite FTS5 via `modernc.org/sqlite` (pure Go)
- Incremental updates keyed on file `mtime` + `size`

## MCP tools

| Tool | Phase | Description |
|------|-------|-------------|
| `search_file` | 1 | Keyword search in one file |
| `index_status` | 1 | Index readiness and config |
| `index_paths` | 2 | Build/update local index |
| `search_local` | 2–3 | Query persisted index |
| `search_web` | 4 | Internet search via LangSearch |

## Search modes

- **keyword** — BM25 / line scan
- **vector** — embedding similarity
- **hybrid** — parallel retrieval + RRF fusion

## External APIs

- [LangSearch Web Search](https://docs.langsearch.com/api/web-search-api)
- [LangSearch Semantic Rerank](https://docs.langsearch.com/api/semantic-rerank-api)

## Environment

See [README.md](../README.md#environment-variables).
