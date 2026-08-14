# Handoff: opt-embed-engine-adapter

**Status:** done  
**Created:** 2026-08-14  
**Specifier:** lean thin handoff  
**Developer:** lean in-supervisor

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `opt-embed-engine-adapter` |
| Parent issue | #54 — https://github.com/SpektrNO/searchify/issues/54 |
| Open tasks | _(none)_ |
| Closed | `spec` (#55), `engine` (#56), `verify` (#57), `docs` (#58) |

## Intent

Make the embedding stack pluggable behind `Embedder`: kjarni remains default; add Ollama and generic HTTP engines.

## Implementation result

### Changes

- `SEARCHIFY_EMBED_ENGINE=kjarni|ollama|http` (default kjarni); `SEARCHIFY_EMBED_URL` for Ollama base or HTTP endpoint.
- Ollama: `POST /api/embed` with `model` + `input` array.
- HTTP: `POST {"model","input"}` → `embedding` / `embeddings`.
- Meta `embed_engine` + `embed_model`; switch clears vectors; search rejects mismatch.
- `index_status.embed_engine`; MCP **0.8.4**.

### Verification

- [x] `go test ./...` (config resolve, httptest ollama/http, engine-switch clear)
- [ ] Manual: Ollama + `nomic-embed-text` / Qwen embed model + `embed --force`

### Deviations

- No in-process ONNX Runtime loader; HTTP/Ollama covers local-server models.

### Follow-ups

- Optional OpenAI-compatible `/v1/embeddings` shape if a host needs it.
