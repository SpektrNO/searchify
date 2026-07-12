# Handoff: Local keyword index (index_paths + search_local)

**Status:** spec  
**Created:** 2026-07-12  
**Specifier:** spec complete  
**Developer:** pending

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `phase2-local-keyword` |
| Parent issue | — (not created yet) |
| Open tasks | `spec` ✓, `engine`, `verify`, `docs` |

Task order: `audit` → `spec` → `engine` → `verify` → `docs`

## Intent

Enable **multi-file keyword search** over a persisted local index so agents can query an entire docs/code corpus in one MCP call instead of repeated greps — the first step toward Searchify's hybrid retrieval pipeline.

## Background

Phase 1 (`search_file`) proves MCP wiring and path allowlisting but only searches one file at a time with naive term scoring. Phase 2 adds **indexing + BM25-style lexical search** across many files under `SEARCHIFY_ROOTS`.

## Technical contract

### Storage backend (Phase 2 decision)

Use **SQLite FTS5** via `modernc.org/sqlite` (pure Go, no CGO):

- Index file: `{SEARCHIFY_INDEX_DIR}/index.db`
- Lexical search via FTS5 `MATCH` + BM25 rank (`bm25()` helper if available, else FTS5 rank)
- **Do not** pull in kjarni-go yet — Phase 3 adds vectors/hybrid; keep `internal/local` behind an interface so vector backend can be added without rewriting MCP tools

Rationale: Phase 2 is keyword-only; SQLite FTS5 is lightweight, debuggable, and matches the plan's escape hatch.

### Index schema (minimum)

| Table / object | Purpose |
|----------------|---------|
| `files` | `path` (PK), `mtime_ns`, `size`, `content_hash`, `indexed_at` |
| `chunks` | `id`, `file_path`, `chunk_index`, `line_start`, `line_end`, `text` |
| `chunks_fts` | FTS5 virtual table over `text`, content=`chunks` |

Store absolute normalized paths. Chunk IDs stable as `{path}#chunk-{n}`.

### Chunking rules

| Rule | Value |
|------|-------|
| Target chunk size | ~2–4 KB UTF-8 text |
| Boundaries | Prefer splitting on blank lines; fall back to line splits |
| Overlap | None in v1 (add in Phase 3 if needed for vectors) |
| File types | Index text files: `.md`, `.txt`, `.go`, `.ts`, `.tsx`, `.js`, `.json`, `.yaml`, `.yml`, `.sql`, `.sh`, `.py`, `.rs` |
| Skip | Binary extensions, symlinks to dirs outside roots, hidden dirs (`.git`, `.cursor`, `node_modules`, `vendor`, `bin`, `.searchify`) |
| Max file size | 2 MB per file (configurable constant; skip larger with log/warning in index report) |

### Security

- All index and search paths must pass `config.AllowedPath()` (existing allowlist)
- `index_paths` rejects any path outside `SEARCHIFY_ROOTS`
- Never index or return content outside allowed roots

### MCP tool: `index_paths`

**Description:** Build or incrementally update the local keyword index for files under the given paths.

**Input:**

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `paths` | `[]string` | yes | Files or directories under allowed roots |
| `force` | `bool` | no | Re-index even if mtime/size unchanged (default false) |

**Output:**

```go
type indexPathsOutput struct {
    Indexed  int      `json:"indexed"`   // new files
    Updated  int      `json:"updated"`   // changed files re-chunked
    Skipped  int      `json:"skipped"`   // unchanged
    Errors   int      `json:"errors"`
    Messages []string `json:"messages,omitempty"` // per-path errors, capped at 20
}
```

**Behavior:**

1. Walk each path recursively for indexable files
2. For each file: if `mtime+size` matches `files` row and `force=false`, skip
3. Else delete old chunks for file, re-chunk, insert into FTS5
4. Update `files` metadata
5. Set global `indexed_at` in index metadata table or sidecar JSON `{index_dir}/meta.json`

### MCP tool: `search_local`

**Description:** Keyword search over the persisted local index.

**Input:**

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `query` | `string` | yes | FTS5 query (space-separated terms → AND by default) |
| `limit` | `int` | no | Default 10, max 50 |
| `mode` | `string` | no | Phase 2: only `"keyword"` accepted; if `vector`/`hybrid` passed, return tool error with message "not available until phase 3" |

**Output:**

```go
type searchLocalOutput struct {
    Count   int             `json:"count"`
    Results []search.Result `json:"results"`
}
```

**Result mapping** (reuse `search.Result`):

| Field | Value |
|-------|-------|
| `id` | chunk id |
| `title` | `basename:line_start` |
| `path` | file path |
| `snippet` | chunk text trimmed (~300 chars) |
| `score` | FTS5/BM25 rank (float64) |
| `source` | `"local"` |
| `line` | `line_start` |

**Errors:**

- Index missing or empty → tool error: `"index not ready; call index_paths first"`
- Index DB corrupt → clear error, suggest re-index

### MCP tool: `index_status` (update existing)

Replace stub with live data from index:

| Field | Source |
|-------|--------|
| `document_count` | count of `files` rows |
| `chunk_count` | count of `chunks` rows |
| `indexed_at` | ISO8601 from meta |
| `ready` | `true` if `chunk_count > 0` |
| `message` | empty when ready; else helpful hint |

### CLI: `searchify index`

Implement stub in `cmd/searchify/main.go`:

```bash
searchify index [--force] <path...>
```

Uses same indexer as `index_paths`. Prints summary to stdout (indexed/updated/skipped/errors). Exit code 1 if any errors.

Wire `Server` with shared `*local.Index` or `*local.Service` instance so MCP tools and CLI use one code path.

### Package layout

| File | Responsibility |
|------|----------------|
| `internal/local/index.go` | Open/create DB, migrations, metadata |
| `internal/local/chunk.go` | Split file text into chunks |
| `internal/local/walk.go` | Directory walk + allowlist + skip rules |
| `internal/local/indexer.go` | Incremental index orchestration |
| `internal/local/searcher.go` | FTS5 keyword queries |
| `internal/mcp/tools_index_paths.go` | MCP handler |
| `internal/mcp/tools_search_local.go` | MCP handler |
| `internal/mcp/tools_index_status.go` | Wire to real index stats |
| `internal/mcp/server.go` | Register new tools; hold index service |

Optional: `internal/local/local_test.go` with temp dir + small fixture files.

### Performance targets

| Operation | Target |
|-----------|--------|
| `search_local` keyword | <100 ms p95 on ~50k chunks (dev machine) |
| Indexing throughput | ≥1 MB/s text on dev machine |
| Index open | <50 ms (reuse connection per MCP server process) |

Open index DB once at MCP server startup; do not reopen per tool call.

### Acceptance criteria

1. `index_paths` on `docs/` indexes all markdown files under allowlist
2. `search_local` with query `"shard realm"` returns ranked chunks from indexed docs with correct paths and line numbers
3. Re-running `index_paths` without changes reports `skipped` ≈ total files, completes quickly
4. Editing one file and re-indexing updates only that file's chunks
5. `index_status` shows accurate counts and `ready: true`
6. `searchify index docs/` CLI produces same index as MCP tool
7. Path outside `SEARCHIFY_ROOTS` rejected for both index and search
8. `search_local` with `mode: "hybrid"` returns clear not-implemented error (not silent fallback)
9. All MCP outputs use typed structs (no `json.RawMessage`) — lesson from Phase 1 schema bug
10. `go test ./...` passes

### Verification plan

```bash
export SEARCHIFY_ROOTS="$PWD"
export SEARCHIFY_INDEX_DIR="/tmp/searchify-test-index"

# CLI index
go run ./cmd/searchify index docs/

# MCP manual: index_status → ready true
# MCP manual: search_local query="feature backlog" → hits feature-backlog.md
# MCP manual: search_local on unindexed dir → error

go test ./internal/local/...
```

## Touchpoints

- [cmd/searchify/main.go](../cmd/searchify/main.go) — implement `runIndex`
- [internal/mcp/server.go](../internal/mcp/server.go) — register tools, shared index service
- [internal/mcp/tools_index_status.go](../internal/mcp/tools_index_status.go) — live stats
- [internal/local/](../internal/local/) — new implementation (replace `doc.go` stub)
- [go.mod](../go.mod) — add `modernc.org/sqlite`
- [README.md](../README.md) — document `index_paths`, `search_local`, CLI `index`
- [docs/architecture.md](./architecture.md) — note Phase 2 storage choice

## Out of scope (defer to later phases)

- Vector embeddings and hybrid search (Phase 3)
- LangSearch rerank (Phase 3)
- `search_web` (Phase 4)
- HTTP transport (Phase 5)
- PDF/Office parsing
- Query router auto mode selection (`internal/search/router.go`) — Phase 3; Phase 2 hard-requires `keyword` or defaults to keyword
- Regex in FTS queries (plain term search is enough for v1)
- Background/async indexing ( synchronous is fine; document that large trees may take time)
- kjarni-go integration

## Implementation notes for developer

1. **FTS5 query sanitization:** escape or strip characters that break FTS syntax (`"`, `*`, etc.) — simplest approach: tokenize like `internal/file/scanner.go` and join with `AND`
2. **MCP server lifecycle:** create `local.Service` in `NewServer(cfg)` after validating index dir is creatable
3. **Idempotent migrations:** version table `schema_version`; migration SQL in `embed` or const strings
4. **Do not break Phase 1:** `search_file` unchanged
5. After done: `./scripts/record-feature-complete.sh phase2-local-keyword --note "..."` and archive this handoff to `docs/handoffs/archive/2026-07-12-phase2-local-keyword.md`

---

## Implementation result

*(Developer agent fills this section.)*

### Changes

- 

### Verification

- [ ] How tested
- [ ] What remains manual

### Deviations from spec

- None / list with rationale

### Follow-ups

- 
