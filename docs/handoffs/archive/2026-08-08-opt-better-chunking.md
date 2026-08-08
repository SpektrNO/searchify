# Handoff: opt-better-chunking

**Status:** done  
**Created:** 2026-08-08  
**Specifier:** lean thin handoff  
**Developer:** lean in-supervisor

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `opt-better-chunking` |
| Parent issue | #47 — https://github.com/SpektrNO/searchify/issues/47 |
| Open tasks | _(none)_ |
| Closed | `spec` (#48), `engine` (#49), `verify` (#50), `docs` (#51) |

## Intent

Improve retrieval chunking: tunable target size and overlap, structure-aware splits (Markdown headings, form-feed pages, paragraphs), and split oversized blocks. Changing chunk settings requires re-index (`index --force`) then re-embed as needed.

## Implementation result

### Changes

- `SEARCHIFY_CHUNK_BYTES` (default 3072), `SEARCHIFY_CHUNK_OVERLAP` (default 256; must be `<` bytes).
- Hard pack boundaries: Markdown ATX headings, `\f` page breaks; paragraph packing; oversized unit hard-split; overlap suffix carry.
- Wired through `indexFile` via `ChunkParams`.
- README / architecture / troubleshooting re-index story.

### Verification

- [x] `go test ./...` (chunk unit tests for headings, form-feed, overlap, oversized)
- [ ] Manual: re-index vault with new defaults and spot-check section-aligned hits

### Deviations

- None material.

### Follow-ups

- Token-based sizing; store chunk params in meta for stale detection (`opt-embed-engine-adapter` unrelated).
