# Feature Completed

Shipped features registry. Complements [feature-backlog.md](./feature-backlog.md).

```bash
./scripts/record-feature-complete.sh <feature-id> [--issue N] [--note "..."]
```

---

## Recent completions

| Date | ID | Feature | GitHub | Notes |
|------|-----|---------|--------|-------|
| 2026-08-07 | `opt-richer-file-types` | Extract and index PDF, Office, images (OCR), HTML/CSV and related formats beyond the text/code allowlist | #24 | PDF/Office/HTML extractors + optional OCR; MCP 0.7.0 |
| 2026-08-07 | `opt-auto-index-watch` | Optional fsnotify (or periodic rescan) on configured watch paths to index new/changed files | #19 | fsnotify watch + optional rescan; MCP 0.6.2 |
| 2026-08-07 | `opt-index-prune` | Reconcile index vs disk: drop orphan DB rows for files missing under a root | — | index_prune MCP + CLI prune; MCP 0.6.1 |
| 2026-08-07 | `opt-rest-v1-search` | Plain REST `POST /v1/search` for app backends (e.g. Groundline) without MCP JSON-RPC | — | POST /v1/search; MCP 0.6.0 |
| 2026-08-07 | `opt-rest-v1-index` | Plain REST `POST /v1/index` twin of `index_paths` (`paths`, `force`) for app-driven ingest | — | POST /v1/index; MCP 0.6.0 |
| 2026-08-07 | `opt-remove-path` | Remove deleted files from the index (MCP tool and/or REST); prune FTS + vectors | — | remove_paths MCP + CLI; MCP 0.5.3 |
| 2026-08-07 | `opt-relative-path-resolve` | Resolve relative `search_file` / index paths against roots or workspace, not MCP process cwd | — | relative paths via PATH_BASE + roots; MCP 0.5.2 |
| 2026-08-07 | `opt-timing-metrics` | Return process timing on search/index | — | duration_ms + search_local timing |
| 2026-08-07 | `phase5-http-hardening` | Streamable HTTP, auth, benchmarks | — | serve http + healthz + benches |
| 2026-08-07 | `phase4-web-search` | LangSearch web API + search_web tool | — | search_web + web.Client cache/429 |
| 2026-07-11 | `phase1-mcp-skeleton` | MCP stdio server, search_file, index_status | — | Initial project scaffolding |
| 2026-07-12 | `phase2-local-keyword` | index_paths + search_local (BM25) | — | SQLite FTS5 index_paths search_local CLI index |
| 2026-07-12 | `phase3-hybrid-local` | In-process vectors, RRF, optional LangSearch rerank | — | Completed via spec→implement pipeline |
| 2026-08-08 | `opt-embed-worker` | Isolate/replace in-process kjarni ONNX embeds so bulk index does not pin multi-GB native RSS — prefer subprocess embed worker + optional lighter backend; keep `--skip-embed` / keyword path — [#36](https://github.com/SpektrNO/searchify/issues/36) | #36 | MCP 0.8.0: SEARCHIFY_EMBED_BACKEND=process + searchify embed |
| 2026-08-08 | `opt-better-embeddings` | Higher-quality embedding model than default MiniLM-L6 (e.g. mpnet / multilingual / selectable via config); safe re-embed of existing indexes; keep process-worker + skip-embed paths — [#42](https://github.com/SpektrNO/searchify/issues/42) | #42 | MCP 0.8.3: SEARCHIFY_EMBED_MODEL allowlist + safe re-embed |
| 2026-08-08 | `opt-better-chunking` | Improve chunking for retrieval: tunable size/overlap, structure-aware splits (headings/pages/paragraphs), fewer truncated/oversized chunks; re-index/re-embed story — [#47](https://github.com/SpektrNO/searchify/issues/47) | #47 | SEARCHIFY_CHUNK_BYTES/OVERLAP + structure-aware splits |
| 2026-08-14 | `opt-embed-engine-adapter` | Pluggable embed/vector engine behind the existing `Embedder` interface so kjarni is one backend among others (e.g. ONNX Runtime / HTTP worker with NB or multilingual models); config to select engine + model; safe re-embed on switch; keep process-worker isolation — [#54](https://github.com/SpektrNO/searchify/issues/54) | #54 | MCP 0.8.4: SEARCHIFY_EMBED_ENGINE=kjarni|ollama|http |
| 2026-09-05 | `opt-code-symbols` | Code-aware chunking + symbol/ref index: extensible `Analyzer` (Python v1 via AST worker; later Go/TS/C#); schema for `chunk_symbols` / `symbols` / `symbol_refs`; MCP `lookup_symbol` + `find_references`; enrich `search_local` with optional symbol fields; walk skips for `venv` / `__pycache__` / etc. | #60 | MCP 0.9.0: Analyzer + lookup_symbol/find_references |
| 2026-09-05 | `opt-code-symbols-go` | Go Analyzer (`go/parser` + `go/ast` in-process): units/symbols/refs for `.go`; reuse schema v4 + MCP `lookup_symbol` / `find_references`; fail-soft to text chunks | #66 | Completed via spec→implement pipeline |
| 2026-09-05 | `opt-code-symbols-ts` | TypeScript/JavaScript Analyzer (Node worker, e.g. ts-morph or equivalent): units/symbols/refs for `.ts` / `.tsx` / `.js` / `.jsx`; reuse schema + MCP tools; fail-soft when Node missing | #71 | Completed via spec→implement pipeline |
| 2026-09-05 | `opt-code-symbols-csharp` | C# Analyzer (Roslyn-oriented worker): units/symbols/refs for `.cs`; reuse schema + MCP tools; fail-soft when `dotnet` missing | #76 | Completed via spec→implement pipeline |
| _—_ | _pipeline completions append here (newest first)_ | | | |

---

## A. MCP foundation

| ID | Feature | Completed | Spec | Notes |
|----|---------|-----------|------|-------|
| `phase1-mcp-skeleton` | MCP stdio server, search_file, index_status | 2026-07-11 | [architecture.md](./architecture.md) | Initial project scaffolding |

## B. Local keyword index

| ID | Feature | Completed | Spec | Notes |
|----|---------|-----------|------|-------|
| `phase2-local-keyword` | index_paths + search_local (BM25) | 2026-07-12 | [handoffs/current.md](./handoffs/current.md) | SQLite FTS5 index_paths search_local CLI index |

## C. Hybrid local search

| ID | Feature | Completed | Spec | Notes |
|----|---------|-----------|------|-------|
| `phase3-hybrid-local` | In-process vectors, RRF, optional LangSearch rerank | 2026-07-12 | [handoffs/current.md](./handoffs/current.md) | Completed via spec→implement pipeline |

## D. Web search integration

| ID | Feature | Completed | Spec | Notes |
|----|---------|-----------|------|-------|
| `phase4-web-search` | LangSearch web API + search_web tool | 2026-08-07 | [handoffs/archive/2026-08-07-phase4-web-search.md](./handoffs/archive/2026-08-07-phase4-web-search.md) | search_web + web.Client cache/429 |

## E. HTTP and hardening

| ID | Feature | Completed | Spec | Notes |
|----|---------|-----------|------|-------|
| `phase5-http-hardening` | Streamable HTTP, auth, benchmarks | 2026-08-07 | [handoffs/archive/2026-08-07-phase5-http-hardening.md](./handoffs/archive/2026-08-07-phase5-http-hardening.md) | serve http + healthz + benches |

## Other

| ID | Feature | Completed | Spec | Notes |
|----|---------|-----------|------|-------|
| `opt-relative-path-resolve` | Resolve relative `search_file` / index paths against roots or workspace, not MCP process cwd | 2026-08-07 | [handoffs/archive/2026-08-07-opt-relative-path-resolve.md](./handoffs/archive/2026-08-07-opt-relative-path-resolve.md) | relative paths via PATH_BASE + roots; MCP 0.5.2 |
| `opt-remove-path` | Remove deleted files from the index (MCP tool and/or REST); prune FTS + vectors | 2026-08-07 | [handoffs/archive/2026-08-07-opt-remove-path.md](./handoffs/archive/2026-08-07-opt-remove-path.md) | remove_paths MCP + CLI; MCP 0.5.3 |
| `opt-rest-v1-search` | Plain REST `POST /v1/search` for app backends (e.g. Groundline) without MCP JSON-RPC | 2026-08-07 | [handoffs/archive/2026-08-07-opt-rest-v1.md](./handoffs/archive/2026-08-07-opt-rest-v1.md) | POST /v1/search; MCP 0.6.0 |
| `opt-rest-v1-index` | Plain REST `POST /v1/index` twin of `index_paths` (`paths`, `force`) for app-driven ingest | 2026-08-07 | [handoffs/archive/2026-08-07-opt-rest-v1.md](./handoffs/archive/2026-08-07-opt-rest-v1.md) | POST /v1/index; MCP 0.6.0 |
| `opt-index-prune` | Reconcile index vs disk: drop orphan DB rows for files missing under a root | 2026-08-07 | [handoffs/archive/2026-08-07-opt-index-prune.md](./handoffs/archive/2026-08-07-opt-index-prune.md) | index_prune MCP + CLI prune; MCP 0.6.1 |
| `opt-auto-index-watch` | Optional fsnotify (or periodic rescan) on configured watch paths to index new/changed files | 2026-08-07 | [handoffs/archive/2026-08-07-opt-auto-index-watch.md](./handoffs/archive/2026-08-07-opt-auto-index-watch.md) | fsnotify watch + optional rescan; MCP 0.6.2 |
| `opt-richer-file-types` | Extract and index PDF, Office, images (OCR), HTML/CSV and related formats beyond the text/code allowlist | 2026-08-07 | [handoffs/archive/2026-08-07-opt-richer-file-types.md](./handoffs/archive/2026-08-07-opt-richer-file-types.md) | PDF/Office/HTML extractors + optional OCR; MCP 0.7.0 |
| `opt-embed-worker` | Isolate/replace in-process kjarni ONNX embeds so bulk index does not pin multi-GB native RSS — prefer subprocess embed worker + optional lighter backend; keep `--skip-embed` / keyword path — [#36](https://github.com/SpektrNO/searchify/issues/36) | 2026-08-08 | [architecture.md](./architecture.md) | MCP 0.8.0: SEARCHIFY_EMBED_BACKEND=process + searchify embed |
| `opt-better-embeddings` | Higher-quality embedding model than default MiniLM-L6 (e.g. mpnet / multilingual / selectable via config); safe re-embed of existing indexes; keep process-worker + skip-embed paths — [#42](https://github.com/SpektrNO/searchify/issues/42) | 2026-08-08 | [architecture.md](./architecture.md) | MCP 0.8.3: SEARCHIFY_EMBED_MODEL allowlist + safe re-embed |
| `opt-better-chunking` | Improve chunking for retrieval: tunable size/overlap, structure-aware splits (headings/pages/paragraphs), fewer truncated/oversized chunks; re-index/re-embed story — [#47](https://github.com/SpektrNO/searchify/issues/47) | 2026-08-08 | [architecture.md](./architecture.md) | SEARCHIFY_CHUNK_BYTES/OVERLAP + structure-aware splits |
| `opt-embed-engine-adapter` | Pluggable embed/vector engine behind the existing `Embedder` interface so kjarni is one backend among others (e.g. ONNX Runtime / HTTP worker with NB or multilingual models); config to select engine + model; safe re-embed on switch; keep process-worker isolation — [#54](https://github.com/SpektrNO/searchify/issues/54) | 2026-08-14 | [architecture.md](./architecture.md) | MCP 0.8.4: SEARCHIFY_EMBED_ENGINE=kjarni|ollama|http |
| `opt-code-symbols` | Code-aware chunking + symbol/ref index: extensible `Analyzer` (Python v1 via AST worker; later Go/TS/C#); schema for `chunk_symbols` / `symbols` / `symbol_refs`; MCP `lookup_symbol` + `find_references`; enrich `search_local` with optional symbol fields; walk skips for `venv` / `__pycache__` / etc. | 2026-09-05 | [adr/001-code-symbols.md](./adr/001-code-symbols.md) | MCP 0.9.0: Analyzer + lookup_symbol/find_references |
| `opt-code-symbols-go` | Go Analyzer (`go/parser` + `go/ast` in-process): units/symbols/refs for `.go`; reuse schema v4 + MCP `lookup_symbol` / `find_references`; fail-soft to text chunks | 2026-09-05 | [adr/001-code-symbols.md](./adr/001-code-symbols.md) | Completed via spec→implement pipeline |
| `opt-code-symbols-ts` | TypeScript/JavaScript Analyzer (Node worker, e.g. ts-morph or equivalent): units/symbols/refs for `.ts` / `.tsx` / `.js` / `.jsx`; reuse schema + MCP tools; fail-soft when Node missing | 2026-09-05 | [adr/001-code-symbols.md](./adr/001-code-symbols.md) | Completed via spec→implement pipeline |
| `opt-code-symbols-csharp` | C# Analyzer (Roslyn-oriented worker): units/symbols/refs for `.cs`; reuse schema + MCP tools; fail-soft when `dotnet` missing | 2026-09-05 | [adr/001-code-symbols.md](./adr/001-code-symbols.md) | Completed via spec→implement pipeline |