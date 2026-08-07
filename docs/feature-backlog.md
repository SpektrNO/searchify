# Feature Backlog

Segmentation index for feature-by-feature implementation.

```text
/spec-only <issue#|feature-id|title fragment>
/spec-and-implement <issue#|feature-id|title fragment> — full
```

Agents load parent issue + sub-tasks via `./scripts/load-feature-issue.sh` (add script when creating GitHub issues).

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
| `opt-hnsw-vectors` | HNSW (or other ANN) for faster vector/hybrid search at large chunk counts | ⬜ | [architecture.md](./architecture.md) |
| `opt-tls-reverse-proxy` | Document/deploy TLS termination via reverse proxy in front of `serve http` | ⬜ | [architecture.md](./architecture.md) |
| `opt-rest-v1-search` | Plain REST `POST /v1/search` for app backends (e.g. Groundline) without MCP JSON-RPC | ⬜ | [architecture.md](./architecture.md) |
| `opt-rest-v1-index` | Plain REST `POST /v1/index` twin of `index_paths` (`paths`, `force`) for app-driven ingest | ⬜ | [architecture.md](./architecture.md) |
| `opt-remove-path` | Remove deleted files from the index (MCP tool and/or REST); prune FTS + vectors | ⬜ | [architecture.md](./architecture.md) |
| `opt-index-prune` | Reconcile index vs disk: drop orphan DB rows for files missing under a root | ⬜ | [architecture.md](./architecture.md) |
| `opt-auto-index-watch` | Optional fsnotify (or periodic rescan) on configured watch paths to index new/changed files | ⬜ | [architecture.md](./architecture.md) |
| `opt-relative-path-resolve` | Resolve relative `search_file` / index paths against roots or workspace, not MCP process cwd | ⬜ | [architecture.md](./architecture.md) |
| `opt-richer-file-types` | Expand indexable types (e.g. PDF/Office text extractors) beyond current extension allowlist | ⬜ | [architecture.md](./architecture.md) |
| `opt-timing-metrics` | Return process timing (e.g. `duration_ms`) on search/index responses; log p50/p95-friendly fields for corpus health | ⬜ | [architecture.md](./architecture.md) |
