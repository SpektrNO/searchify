# Handoff: opt-better-chunking

**Status:** implementing  
**Created:** 2026-08-08  
**Specifier:** lean thin handoff  
**Developer:** lean in-supervisor

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `opt-better-chunking` |
| Parent issue | #47 — https://github.com/SpektrNO/searchify/issues/47 |
| Open tasks | `engine` (#49), `verify` (#50), `docs` (#51) |
| Closed | `spec` (#48) |

Task order: `spec` → `engine` → `verify` → `docs`

## Intent

Improve retrieval chunking: tunable target size and overlap, structure-aware splits (Markdown headings, form-feed pages, paragraphs), and split oversized blocks—so BM25/vector hits align better with document structure. Changing chunk settings requires re-index (`index --force`) then re-embed as needed.

## Technical contract

| Area | Requirement |
|------|-------------|
| Config | `SEARCHIFY_CHUNK_BYTES` (default 3072), `SEARCHIFY_CHUNK_OVERLAP` (default 256, must be `<` chunk bytes). Existing `SEARCHIFY_MAX_CHUNKS_PER_FILE` still truncates. |
| Splits | Hard boundaries: Markdown ATX headings (`#`…`######`), form-feed `\f` (PDF pages), blank-line paragraphs. Soft pack to target size; carry overlap into next chunk; hard-split units larger than target. |
| MCP/REST | Unchanged shapes. |
| Acceptance | (1) heading/page splits produce separate chunks; (2) overlap present when configured; (3) oversized paragraph split; (4) `go test ./...` green; (5) README documents env + re-index story. |

## Touchpoints

- `internal/local/chunk.go`, `internal/config`, `service.indexFile`
- README, architecture, troubleshooting

## Out of scope

- Semantic/LLM chunking, tokenizers, PDF layout analysis beyond `\f`
- Auto-detect stale chunk settings vs meta (document force re-index)
- Changing default max chunks / extract caps

---

## Implementation result

*(Developer fills.)*
