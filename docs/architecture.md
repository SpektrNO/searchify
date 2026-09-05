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
- App integration: MCP Streamable HTTP plus REST `POST /v1/search`, `POST /v1/index`, `GET /v1/files`, `GET /v1/stats` (same Bearer token); optional REST remove later
- Index inventory: MCP `list_indexed_files` / REST `GET /v1/files`; MCP `index_stats` / REST `GET /v1/stats` (`file_count`, `folder_count`, `vector_chunk_count`, `total_bytes`, `last_index_change`)
- Deletions: MCP/CLI `remove_paths` / `searchify remove`; reconcile orphans with `index_prune` / `searchify prune` (`opt-index-prune`)
- Auto-index: optional `SEARCHIFY_WATCH_PATHS` fsnotify (+ optional `SEARCHIFY_WATCH_RESCAN`) starts with `mcp stdio` / `serve http`
- Path UX: relative paths resolve via `SEARCHIFY_PATH_BASE` then roots (no process CWD); absolute paths must stay under roots. Nested `SEARCHIFY_ROOTS` entries are collapsed to outermost roots so the same tree is not walked twice.
- Snippets: default max is `SEARCHIFY_SNIPPET_CHARS` (300); per-query `snippet_max` on `search_local` / `POST /v1/search` (hard cap 8000)
- PDF pages: extract inserts form-feeds between pages (`pdftotext`, Go PDF fallback, OCR). Chunker stores 1-based `page` in `chunk_pages` (schema v3). Search hits expose optional `page` and title `file.pdf:p.N` when known; markdown/code keep `file:line`.
- PPTX / ODP: same `\f` + `page` path — one unit per slide (`deck.pptx:p.2`). ODT/ODS unchanged (no slide/page markers).
- File types: pluggable extractors (`internal/extract`) index passthrough text/code plus PDF/DOCX/XLSX/CSV/HTML (and stretch PPTX/ODF/RTF/EML). Images and scanned-PDF OCR are optional via `SEARCHIFY_OCR` (Tesseract on `PATH`; `pdftoppm` for PDF OCR). Memory: `SEARCHIFY_MAX_FILE_BYTES` (default 2 MiB), extract/chunk caps, `SEARCHIFY_EMBED_BATCH` (default 1), `SEARCHIFY_EMBED_BACKEND` (`process` default / `onnx` / `none`), `SEARCHIFY_SKIP_EMBED` for FTS-only ingest, `SEARCHIFY_EMBED_RELOAD` (default on) when using in-process ONNX. See `opt-richer-file-types` (#24), `opt-embed-worker` (#36)
- Observability: search/index tool responses include `duration_ms`; `search_local` adds per-leg `timing` (backlog was `opt-timing-metrics`)

## Optional scale

- **Embedding memory (`opt-embed-worker`):** in-process kjarni→ONNX can pin multi-GB native RSS (outside Go GC). Default `SEARCHIFY_EMBED_BACKEND=process`: index writes FTS then spawns short-lived `searchify embed --file` so the parent never loads ONNX and the OS reclaims worker RSS on exit. `onnx` keeps legacy in-process embeds (+ `SEARCHIFY_EMBED_RELOAD`). `none` / `SEARCHIFY_SKIP_EMBED` / `index --skip-embed` stay FTS-only; backfill with `searchify embed`. Keep `mode=keyword` first-class.
- **Embedding quality (`opt-better-embeddings`):** `SEARCHIFY_EMBED_MODEL` selects a kjarni model — `minilm-l6-v2` (384-d, default), `mpnet-base-v2` / `distilbert-base` (768-d). Unknown names fail at config load. Switching models clears `chunk_vectors` on the next embed/index write and vector search refuses mismatched meta until `searchify embed --force` (or re-index). Process/skip-embed backends unchanged. No multilingual embed in kjarni today.
- **Embed engine adapter (`opt-embed-engine-adapter`):** `SEARCHIFY_EMBED_ENGINE=kjarni|ollama|http` selects the vector factory (`Embedder`). Kjarni remains default (allowlisted models). Ollama calls `POST {SEARCHIFY_EMBED_URL}/api/embed` (default `http://127.0.0.1:11434`). HTTP posts `{"model","input"}` to a full URL. Meta stores `embed_engine` + `embed_model`; mismatch clears vectors on write and fails vector search until `embed --force`. `SEARCHIFY_EMBED_BACKEND` still means *where* embeds run.
- **Chunking quality (`opt-better-chunking`):** `SEARCHIFY_CHUNK_BYTES` (default 3072) and `SEARCHIFY_CHUNK_OVERLAP` (default 256) pack extracted text. Hard boundaries: Markdown ATX headings, form-feed `\f` (PDF pages), blank-line paragraphs; oversized units hard-split. Changing chunk settings needs `searchify index --force` (+ embed refresh). Cap still `SEARCHIFY_MAX_CHUNKS_PER_FILE`.
- **Code symbols (`opt-code-symbols`):** planned — language `Analyzer` plugins, Python-first AST worker, symbol/ref tables, MCP `lookup_symbol` / `find_references`. See [ADR 001](./adr/001-code-symbols.md). Not implemented yet.
- **Extract memory:** PDF/Office/HTML parsers (especially `ledongthuc/pdf`) can hang or spike multi-GB heap on some real-world PDFs. Default: index spawns short-lived `searchify extract --file` for non-passthrough types; PDF prefers **`pdftotext`** (Poppler) when on `PATH`, else page-loop Go parser; extract timeouts become **skip** so the rest of the corpus continues (`SEARCHIFY_EXTRACT_INPROCESS=1` restores old in-process behavior).
- Vector search is brute-force cosine today; ANN/HNSW is optional when corpora grow (backlog `opt-hnsw-vectors`)
- Storage stays SQLite FTS5 + blob vectors by default; optional store adapter (backlog `opt-store-adapter`) would select backend via config (e.g. `SEARCHIFY_STORE=sqlite|postgres`) so PostgreSQL + pgvector can replace the local DB without changing MCP tools
- **Postgres hosting decision:** when `opt-store-adapter` lands, prefer the **same Postgres instance as Groundline** on a given host (shared server, Searchify-owned schema/database or prefixed tables)—do not run a second Postgres solely for Searchify unless isolation requirements force it. Connection via env (DSN) pointing at that shared instance.

## Environment

See [README.md](../README.md#environment-variables).
