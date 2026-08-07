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
