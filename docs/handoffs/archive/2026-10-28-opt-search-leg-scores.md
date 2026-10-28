# Handoff: Per-leg search scores

**Status:** done  
**Created:** 2026-10-28  
**Specifier:** lean (in-supervisor)  
**Developer:** lean (in-supervisor)

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `opt-search-leg-scores` |
| Parent issue | #93 — https://github.com/SpektrNO/searchify/issues/93 |
| Open tasks | _(all closed)_ `spec` #94, `engine` #95, `verify` #96, `docs` #97 |

Task order: `spec` → `engine` → `verify` → `docs`

## Intent

Expose cardinal keyword (BM25) and vector (cosine) scores on each search hit so Groundline’s evidence gate can apply absolute floors / cliffing — without putting refuse policy in Searchify.

## Technical contract

| Area | Requirement |
|------|-------------|
| API | Additive optional fields on hits: `keyword_score`, `vector_score`, `keyword_rank`, `vector_rank`; keep `score` |
| Units | `keyword_score` = FTS5 `bm25()`; `vector_score` = cosine (typically [-1,1], higher better); ranks 1-based in that leg’s candidate list |
| Modes | keyword → only keyword fields; vector → only vector; hybrid → per-leg when chunk on that leg (else omit, not 0) |
| Non-goals | No refuse thresholds, no ACL/identity changes, no rename of `score` |
| Acceptance | Hybrid both-leg hit has both scores; single-mode only own leg; path allow-set unchanged aside from new fields; tests + README/architecture note |

## Touchpoints

- `internal/search/types.go` (`Result`)
- `internal/local/searcher.go` (keyword / vector / hybrid decoration)
- Tests + README / architecture `#per-leg-scores`

## Out of scope

Groundline refuse strings, ACL allow-set changes, LLM.

---

## Implementation result

### Changes

- `search.Result` additive `keyword_score` / `vector_score` / `keyword_rank` / `vector_rank`
- Keyword / vector modes decorate their leg; hybrid attaches from candidate lists before RRF trim
- README + architecture units documented

### Verification

- [x] `go test ./...` (keyword / vector / hybrid leg-score tests)
- [ ] Groundline evidence-gate dual-contract integration

### Deviations from spec

- None

### Follow-ups

- None
