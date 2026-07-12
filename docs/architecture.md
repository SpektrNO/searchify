# Searchify architecture

Hybrid text-search MCP server in Go.

## Components

| Layer | Package | Role |
|-------|---------|------|
| CLI | `cmd/searchify` | `mcp stdio`, `serve http`, `index` subcommands |
| MCP | `internal/mcp` | Tool registration and transports |
| Config | `internal/config` | Env vars, path allowlist |
| File search | `internal/file` | Single-file keyword scan |
| Local index | `internal/local` | SQLite FTS5 + chunk vectors (phase 2–3) |
| Web | `internal/web` | LangSearch client (phase 4) |
| Rank | `internal/rank` | RRF fusion, LangSearch rerank (phase 3) |

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
