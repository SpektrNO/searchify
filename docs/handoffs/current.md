# Handoff: opt-better-embeddings

**Status:** implementing  
**Created:** 2026-08-08  
**Specifier:** lean thin handoff  
**Developer:** lean in-supervisor

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `opt-better-embeddings` |
| Parent issue | #42 — https://github.com/SpektrNO/searchify/issues/42 |
| Open tasks | `engine` (#44), `verify` (#45), `docs` (#46) |
| Closed | `spec` (#43) |

Task order: `spec` → `engine` → `verify` → `docs`

## Intent

Make higher-quality kjarni embedding models selectable via `SEARCHIFY_EMBED_MODEL`, and safely re-embed when the model (and thus vector dimension) changes—without breaking process-worker / skip-embed paths.

## Technical contract

| Area | Requirement |
|------|-------------|
| Config | Validate `SEARCHIFY_EMBED_MODEL` against kjarni-supported names: `minilm-l6-v2` (384d, default), `mpnet-base-v2` (768d), `distilbert-base` (768d). Clear error on unknown. |
| Re-embed | If index `meta.embed_model` ≠ current config and `chunk_vectors` exist: **clear all vectors** before writing new ones (index embed / `searchify embed`). Prefer `searchify embed --force` after changing model. |
| Search | `mode=vector` (and hybrid vector leg) **errors** if meta model ≠ config when vectors exist—do not silently mix dims. Keyword-only still works. |
| Backends | `SEARCHIFY_EMBED_BACKEND=process\|onnx\|none` and `--skip-embed` unchanged. |
| MCP/REST | No new tools; `index_status.embed_model` already surfaces stored model. Bump MCP patch version. |
| Acceptance | (1) unknown model rejected at config load; (2) model switch clears stale vectors on embed/index write; (3) mismatched search fails with actionable message; (4) `go test ./...` green; (5) README documents models + re-embed steps. |

## Touchpoints

- `internal/config` — allowlist / validate
- `internal/local` — reconcile on write; require match on vector search
- `README.md`, `docs/architecture.md`, handoff result

## Out of scope

- True multilingual embed models (kjarni has none today; classifiers only)
- Changing default away from MiniLM
- HNSW / new embed backends / LangSearch rerank
- Auto-download UI beyond kjarni’s existing first-use cache

---

## Implementation result

*(Developer fills.)*
