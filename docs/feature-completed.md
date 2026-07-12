# Feature Completed

Shipped features registry. Complements [feature-backlog.md](./feature-backlog.md).

```bash
./scripts/record-feature-complete.sh <feature-id> [--issue N] [--note "..."]
```

---

## Recent completions

| Date | ID | Feature | GitHub | Notes |
|------|-----|---------|--------|-------|
| 2026-07-11 | `phase1-mcp-skeleton` | MCP stdio server, search_file, index_status | — | Initial project scaffolding |
| 2026-07-12 | `phase2-local-keyword` | index_paths + search_local (BM25) | — | SQLite FTS5 index_paths search_local CLI index |
| 2026-07-12 | `phase3-hybrid-local` | In-process vectors, RRF, optional LangSearch rerank | — | Completed via spec→implement pipeline |
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
