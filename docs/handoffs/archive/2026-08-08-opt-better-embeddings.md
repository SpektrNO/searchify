# Handoff: opt-better-embeddings

**Status:** done  
**Created:** 2026-08-08  
**Specifier:** lean thin handoff  
**Developer:** lean in-supervisor

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `opt-better-embeddings` |
| Parent issue | #42 — https://github.com/SpektrNO/searchify/issues/42 |
| Open tasks | _(none)_ |
| Closed | `spec` (#43), `engine` (#44), `verify` (#45), `docs` (#46) |

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

### Changes

- `SEARCHIFY_EMBED_MODEL` validated against kjarni models: `minilm-l6-v2` (384d, default), `mpnet-base-v2` (768d), `distilbert-base` (768d).
- On embed/index vector write: if `meta.embed_model` differs and vectors exist → `DELETE FROM chunk_vectors` then rebuild (avoids mixed dims).
- Vector/hybrid search errors on model mismatch with `searchify embed --force` guidance; keyword unaffected.
- MCP **0.8.3**.
- README embedding-models table; architecture + troubleshooting updated.

### Verification

- [x] `go test ./...` (includes `TestValidateEmbedModel`, `TestEmbedModelChangeClearsVectors`, `TestVectorSearchRejectsEmbedModelMismatch`)
- [x] `go build -o bin/searchify ./cmd/searchify`
- [ ] Manual: set `SEARCHIFY_EMBED_MODEL=mpnet-base-v2`, `searchify embed --force`, hybrid search (downloads model on first use; larger RSS)

### Deviations from spec

- No multilingual embed option — kjarni-go has none; documented as follow-up.

### Follow-ups

- Multilingual embeddings if/when kjarni ships one.
- Optional `index_status` list of known models (docs cover this for now).
