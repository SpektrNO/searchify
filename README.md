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

- `searchify serve http` — Streamable HTTP MCP with Bearer auth, timeouts, `/healthz`, plus REST `POST /v1/search`, `POST /v1/index`, `GET /v1/files`, `GET /v1/stats`
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

# Windows amd64 cross-compile (from Linux/WSL):
make build-win
# → bin/searchify.exe
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

`serve http` (and `mcp stdio`) **only serve the API**. They do **not** crawl roots or refresh the index unless you set `SEARCHIFY_WATCH_PATHS` (see [Indexing lifecycle](#indexing-lifecycle)).

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

# List indexed file paths (optional ?prefix=)
curl -sS "http://127.0.0.1:8080/v1/files" \
  -H "Authorization: Bearer dev-secret"

# Index stats
curl -sS "http://127.0.0.1:8080/v1/stats" \
  -H "Authorization: Bearer dev-secret"
```

| Method | Path | Body / query |
|--------|------|------|
| `POST` | `/v1/search` | `query`, optional `limit`, `mode`, `rerank` |
| `POST` | `/v1/index` | `paths`, optional `force` |
| `GET` | `/v1/files` | optional `prefix` query (exact path or directory prefix) |
| `GET` | `/v1/stats` | — (`file_count`, `folder_count`, `vector_chunk_count`, `total_bytes`, `last_index_change`) |

## Indexing lifecycle

Typical flow:

1. **Index once (or on demand)** — CLI `searchify index <paths…>` or REST `POST /v1/index` / MCP `index_paths`.
2. **Serve** — `searchify serve http` or `mcp stdio` answers search/status from the existing SQLite index.
3. **Optional live updates** — set `SEARCHIFY_WATCH_PATHS` so the running server watches those paths (must stay under `SEARCHIFY_ROOTS`).

### What happens on re-index

- Unchanged files (same size + mtime as last index) are **skipped** (`skipped=` in the CLI summary).
- New or modified files are extracted and indexed/updated.
- `--force` (CLI) or `"force": true` (REST/MCP) re-processes every matching file even when metadata is unchanged.
- Deletes are **not** inferred by a normal index pass. Use `searchify remove` / `remove_paths`, or `searchify prune` / `index_prune`, for orphans.

CLI `index` prints progress on **stderr** (`[i/N] indexing …`) so long catalogues are distinguishable from a hang; the final `indexed=` line is on stdout.

**Large corpora / Windows OOM:** the ONNX embedder (kjarni) can grow native RSS into many GB. Default `SEARCHIFY_EMBED_BACKEND=process` keeps ONNX out of the long-lived index process (spawns `searchify embed --file` per file). Alternatives:

```bash
# FTS only (lowest RAM; use mode=keyword)
# Put flags BEFORE or AFTER paths — both work:
searchify index --skip-embed /path/to/corpus
searchify index /path/to/corpus --skip-embed

# Confirm on stderr: "keyword-only — ONNX/embedder will NOT load"
# If you instead see "EMBED_BACKEND=process" or "onnx", skip-embed did not apply.

# If RAM still spikes immediately, use a fresh index dir + text-only (no PDF/Office parsers):
set SEARCHIFY_INDEX_DIR=C:\temp\searchify-fts
searchify index --skip-embed --text-only /path/to/corpus
# stderr also prints index.db size and per-file heap≈MiB to show which file blows up
```

Or set `SEARCHIFY_SKIP_EMBED=1` / `SEARCHIFY_EMBED_BACKEND=none` (no quotes inside the value in CMD).

### Watch vs serve

| Mode | Behavior |
|------|----------|
| `serve http` / `mcp stdio` alone | Serves only; index stays as-of the last `index` / `/v1/index` |
| + `SEARCHIFY_WATCH_PATHS` | fsnotify: create/write → reindex (debounced); remove/rename → drop from index |
| + `SEARCHIFY_WATCH_RESCAN` (e.g. `5m`) | Periodic full re-index of watch paths — useful when fsnotify is flaky (OneDrive, some network mounts) |

Without `SEARCHIFY_WATCH_PATHS`, nothing is scanned routinely after serve starts.

### Windows / WSL note

- Native Windows: set env vars in the same CMD/PowerShell session as `searchify.exe` (CMD `set` must not put quotes into the value).
- From WSL2, `localhost` usually is **not** the Windows host. Prefer the default gateway (`ip route show default` → gateway IP) or the Windows LAN IP, e.g. `curl http://172.x.x.x:8080/healthz`. Do not use the `nameserver` from `/etc/resolv.conf` when it is `10.255.255.254`.

## Index and search (CLI)

```bash
export SEARCHIFY_ROOTS="/home/you/dev/spektr/searchify"
export SEARCHIFY_INDEX_DIR="/tmp/searchify-index"   # optional

./bin/searchify index docs/
# stderr progress: index: N indexable file(s) / [i/N] indexing path …
# indexed=N updated=N skipped=N errors=N

# Optional: backfill / repair vectors without re-extracting
./bin/searchify embed docs/
# files=N embedded=N skipped=N errors=N
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
| `SEARCHIFY_WATCH_PATHS` | no | Comma-separated paths to auto-index (under roots); empty disables watch |
| `SEARCHIFY_WATCH_DEBOUNCE` | no | Coalesce fs events (Go duration, default `1s`) |
| `SEARCHIFY_WATCH_RESCAN` | no | Periodic re-index of watch paths (e.g. `5m`); `0`/empty disables |
| `SEARCHIFY_OCR` | no | `1`/`true`/`on` enables OCR for images and scanned-PDF fallback (needs `tesseract` on `PATH`; PDF OCR also needs `pdftoppm`) |
| `SEARCHIFY_OCR_LANG` | no | Tesseract language (default `eng`) |
| `SEARCHIFY_MAX_FILE_BYTES` | no | Skip source files larger than this during index (default `2097152` / 2 MiB) |
| `SEARCHIFY_MAX_EXTRACT_BYTES` | no | Truncate extracted text before chunking (default `524288` / 512 KiB) |
| `SEARCHIFY_MAX_CHUNKS_PER_FILE` | no | Max chunks embedded per file (default `64`) |
| `SEARCHIFY_EMBED_BATCH` | no | ONNX batch size (default `1`) |
| `SEARCHIFY_EMBED_BACKEND` | no | `process` (default: spawn `searchify embed` per file), `onnx` (in-process), `none` (FTS only) |
| `SEARCHIFY_SKIP_EMBED` | no | `1`/`true` — FTS/keyword index only; **do not load ONNX** (same idea as `EMBED_BACKEND=none`) |
| `SEARCHIFY_TEXT_ONLY` | no | `1`/`true` — index passthrough text/code only (disables PDF/Office/HTML extractors; helps low-RAM ingest) |
| `SEARCHIFY_EMBED_RELOAD` | no | Close/reopen embedder after each file when `backend=onnx` (default **on**; set `0` to disable) |
| `SEARCHIFY_EXTRACT_TIMEOUT` | no | Per-file extract deadline (Go duration, default `30s`) |
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

### Indexable file types

Passthrough text/code: `.md` `.txt` `.go` `.ts` `.tsx` `.js` `.json` `.yaml` `.yml` `.sql` `.sh` `.py` `.rs` plus `.xml` `.toml` `.ini` `.log` `.rst` `.adoc` `.markdown`.

Extracted formats: `.pdf` `.docx` `.xlsx` `.csv` `.html`/`.htm`, plus stretch `.pptx` `.odt`/`.ods`/`.odp` `.rtf` `.eml`. Images (`.png` `.jpg` `.jpeg` `.webp` `.tif` `.tiff` `.gif`) index only when `SEARCHIFY_OCR=1`. Other extensions are ignored. `index_status` reports `ocr_enabled` and `index_extensions`.

Indexing caps memory via `SEARCHIFY_MAX_FILE_BYTES` (default 2 MiB), `SEARCHIFY_MAX_EXTRACT_BYTES` (512 KiB), `SEARCHIFY_MAX_CHUNKS_PER_FILE` (64), batched embeds (`SEARCHIFY_EMBED_BATCH`, default 1), default `SEARCHIFY_EMBED_BACKEND=process` (short-lived embed worker), optional `SEARCHIFY_SKIP_EMBED` / `index --skip-embed` / `backend=none` (FTS only), and embedder reload when `backend=onnx` (`SEARCHIFY_EMBED_RELOAD`, default on).

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

### Auto-index watch

**Off by default.** Set `SEARCHIFY_WATCH_PATHS` (comma-separated, under roots) to enable fsnotify-based indexing while `mcp stdio` or `serve http` runs. Writes debounce (`SEARCHIFY_WATCH_DEBOUNCE`, default `1s`); deletes call `remove_paths`. Optional `SEARCHIFY_WATCH_RESCAN` (e.g. `5m`) periodically re-indexes watch roots — recommended for OneDrive / cloud-synced folders where file events are unreliable. `index_status` reports `watch_enabled` / `watch_paths`. See [Indexing lifecycle](#indexing-lifecycle).

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

### `list_indexed_files`

Returns paths currently stored in the local index. Optional `prefix` limits to that path and its indexed descendants.

```json
{
  "prefix": ""
}
```

### `index_stats`

Aggregate inventory: `file_count`, `folder_count` (unique parent dirs of indexed files), `vector_chunk_count`, `total_bytes`, `last_index_change` (when index content last changed). Does not track no-op scans.

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
