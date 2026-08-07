# Handoff: opt-embed-worker

**Status:** implementing  
**Created:** 2026-08-08  
**Specifier:** lean thin handoff  
**Developer:** in progress

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `opt-embed-worker` |
| Parent issue | #36 — https://github.com/SpektrNO/searchify/issues/36 |
| Open tasks | `engine` (#38), `verify` (#39), `docs` (#40) |
| Closed | `spec` (#37) |

## Intent

Isolate ONNX embedding from the long-lived index/serve process via a short-lived `searchify embed` worker (and optional spawn during index) so bulk vectorization cannot ratchet multi-GB native RSS.

## Acceptance

1. `searchify embed [paths…]` backfills `chunk_vectors` for indexed files (missing vectors by default; `--force` re-embeds).
2. `SEARCHIFY_EMBED_BACKEND=process` (default when embedding): index writes FTS then spawns `searchify embed --file <path>` per file; parent never loads ONNX.
3. `SEARCHIFY_EMBED_BACKEND=onnx` keeps in-process embed (with existing reload).
4. `SEARCHIFY_SKIP_EMBED` / `index --skip-embed` / backend `none` remain FTS-only.
5. Progress on stderr for `embed`; `go test ./...` green; Windows build works.
6. README documents two-pass and process backend.

## Out of scope

- HTTP embed backend, new model weights, HNSW, changing hybrid math.

## Implementation result

*(filled when done)*
