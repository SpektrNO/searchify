# Feature Backlog

Segmentation index for feature-by-feature implementation.

```text
/spec-only <issue#|feature-id|title fragment>
/spec-and-implement <issue#|feature-id|title fragment> — full
/lean-implement <feature> — thin handoff + implement in-supervisor
```

Agents load parent issue + sub-tasks via `./scripts/load-feature-issue.sh`. Bootstrap GitHub issues with `./scripts/create-feature-issues.sh`.

**Legend:** ✅ Implemented · 🟡 Partial · ⬜ Spec only

Shipped: [feature-completed.md](./feature-completed.md)

GitHub lifecycle: [github-workflow.md](./github-workflow.md)

Architecture: [architecture.md](./architecture.md)

---

## A. MCP foundation

| ID | Feature | Status | Spec |
|----|---------|--------|------|
| `phase1-mcp-skeleton` | MCP stdio server, search_file, index_status | ✅ | [architecture.md](./architecture.md) |

## B. Local keyword index

| ID | Feature | Status | Spec |
|----|---------|--------|------|
| `phase2-local-keyword` | index_paths + search_local (BM25) | ✅ | [handoffs/current.md](./handoffs/current.md) |

## C. Hybrid local search

| ID | Feature | Status | Spec |
|----|---------|--------|------|
| `phase3-hybrid-local` | In-process vectors, RRF, optional LangSearch rerank | ✅ | [handoffs/current.md](./handoffs/current.md) |

## D. Web search integration

| ID | Feature | Status | Spec |
|----|---------|--------|------|
| `phase4-web-search` | LangSearch web API + search_web tool | ✅ | [handoffs/archive/2026-08-07-phase4-web-search.md](./handoffs/archive/2026-08-07-phase4-web-search.md) |

## E. HTTP and hardening

| ID | Feature | Status | Spec |
|----|---------|--------|------|
| `phase5-http-hardening` | Streamable HTTP, auth, benchmarks | ✅ | [handoffs/archive/2026-08-07-phase5-http-hardening.md](./handoffs/archive/2026-08-07-phase5-http-hardening.md) |

## F. Optional scale and ops

Optional follow-ups — not required for v1. Spec with `/spec-only` when prioritized.

| ID | Feature | Status | Spec |
|----|---------|--------|------|
| `opt-embed-worker` | Isolate/replace in-process kjarni ONNX embeds so bulk index does not pin multi-GB native RSS — prefer subprocess embed worker + optional lighter backend; keep `--skip-embed` / keyword path — [#36](https://github.com/SpektrNO/searchify/issues/36) | ✅ | [architecture.md](./architecture.md) |
| `opt-better-embeddings` | Higher-quality embedding model than default MiniLM-L6 (e.g. mpnet / multilingual / selectable via config); safe re-embed of existing indexes; keep process-worker + skip-embed paths — [#42](https://github.com/SpektrNO/searchify/issues/42) | ✅ | [architecture.md](./architecture.md) |
| `opt-embed-engine-adapter` | Pluggable embed/vector engine behind the existing `Embedder` interface so kjarni is one backend among others (e.g. ONNX Runtime / HTTP worker with NB or multilingual models); config to select engine + model; safe re-embed on switch; keep process-worker isolation — [#54](https://github.com/SpektrNO/searchify/issues/54) | ✅ | [architecture.md](./architecture.md) |
| `opt-better-chunking` | Improve chunking for retrieval: tunable size/overlap, structure-aware splits (headings/pages/paragraphs), fewer truncated/oversized chunks; re-index/re-embed story — [#47](https://github.com/SpektrNO/searchify/issues/47) | ✅ | [architecture.md](./architecture.md) |
| `opt-code-symbols` | Code-aware chunking + symbol/ref index: extensible `Analyzer` (Python v1 via AST worker; later Go/TS/C#); schema for `chunk_symbols` / `symbols` / `symbol_refs`; MCP `lookup_symbol` + `find_references`; enrich `search_local` with optional symbol fields; walk skips for `venv` / `__pycache__` / etc. | ✅ | [adr/001-code-symbols.md](./adr/001-code-symbols.md) |
| `opt-code-symbols-go` | Go Analyzer (`go/parser` + `go/ast` in-process): units/symbols/refs for `.go`; reuse schema v4 + MCP `lookup_symbol` / `find_references`; fail-soft to text chunks | ✅ | [adr/001-code-symbols.md](./adr/001-code-symbols.md) |
| `opt-code-symbols-ts` | TypeScript/JavaScript Analyzer (Node worker, e.g. ts-morph or equivalent): units/symbols/refs for `.ts` / `.tsx` / `.js` / `.jsx`; reuse schema + MCP tools; fail-soft when Node missing | ⬜ | [adr/001-code-symbols.md](./adr/001-code-symbols.md) |
| `opt-code-symbols-csharp` | C# Analyzer (Roslyn-oriented worker): units/symbols/refs for `.cs`; reuse schema + MCP tools; fail-soft when `dotnet` missing | ⬜ | [adr/001-code-symbols.md](./adr/001-code-symbols.md) |
| `opt-hnsw-vectors` | HNSW (or other ANN) for faster vector/hybrid search at large chunk counts | ⬜ | [architecture.md](./architecture.md) |
| `opt-store-adapter` | Pluggable index store: SQLite default; Postgres+pgvector via config (prefer shared Groundline Postgres, not a second server) | ⬜ | [architecture.md](./architecture.md) |
| `opt-tls-reverse-proxy` | Document/deploy TLS termination via reverse proxy in front of `serve http` | ⬜ | [architecture.md](./architecture.md) |
| `opt-rest-v1-search` | Plain REST `POST /v1/search` for app backends (e.g. Groundline) without MCP JSON-RPC | ✅ | [handoffs/archive/2026-08-07-opt-rest-v1.md](./handoffs/archive/2026-08-07-opt-rest-v1.md) |
| `opt-rest-v1-index` | Plain REST `POST /v1/index` twin of `index_paths` (`paths`, `force`) for app-driven ingest | ✅ | [handoffs/archive/2026-08-07-opt-rest-v1.md](./handoffs/archive/2026-08-07-opt-rest-v1.md) |
| `opt-remove-path` | Remove deleted files from the index (MCP tool and/or REST); prune FTS + vectors | ✅ | [handoffs/archive/2026-08-07-opt-remove-path.md](./handoffs/archive/2026-08-07-opt-remove-path.md) |
| `opt-index-prune` | Reconcile index vs disk: drop orphan DB rows for files missing under a root | ✅ | [handoffs/archive/2026-08-07-opt-index-prune.md](./handoffs/archive/2026-08-07-opt-index-prune.md) |
| `opt-auto-index-watch` | Optional fsnotify (or periodic rescan) on configured watch paths to index new/changed files | ✅ | [handoffs/archive/2026-08-07-opt-auto-index-watch.md](./handoffs/archive/2026-08-07-opt-auto-index-watch.md) |
| `opt-relative-path-resolve` | Resolve relative `search_file` / index paths against roots or workspace, not MCP process cwd | ✅ | [handoffs/archive/2026-08-07-opt-relative-path-resolve.md](./handoffs/archive/2026-08-07-opt-relative-path-resolve.md) |
| `opt-richer-file-types` | Extract and index PDF, Office, images (OCR), HTML/CSV and related formats beyond the text/code allowlist | ✅ | [handoffs/archive/2026-08-07-opt-richer-file-types.md](./handoffs/archive/2026-08-07-opt-richer-file-types.md) |
| `opt-timing-metrics` | Return process timing (e.g. `duration_ms`) on search/index responses; log p50/p95-friendly fields for corpus health | ✅ | [handoffs/archive/2026-08-07-opt-timing-metrics.md](./handoffs/archive/2026-08-07-opt-timing-metrics.md) |
