# Handoff: opt-embed-engine-adapter

**Status:** implementing  
**Created:** 2026-08-14  
**Specifier:** lean thin handoff  
**Developer:** lean in-supervisor

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `opt-embed-engine-adapter` |
| Parent issue | #54 — https://github.com/SpektrNO/searchify/issues/54 |
| Open tasks | `engine` (#56), `verify` (#57), `docs` (#58) |
| Closed | `spec` (#55) |

## Intent

Make the embedding stack pluggable behind `Embedder`: kjarni remains default; add Ollama and generic HTTP engines so Norwegian/multilingual (or Qwen) models can produce vectors without forking kjarni. Keep process-worker isolation and safe re-embed on engine/model switch.

## Technical contract

| Area | Requirement |
|------|-------------|
| Config | `SEARCHIFY_EMBED_ENGINE=kjarni\|ollama\|http` (default `kjarni`). `SEARCHIFY_EMBED_URL` — Ollama base (default `http://127.0.0.1:11434`) or full HTTP embeddings URL. `SEARCHIFY_EMBED_MODEL` — kjarni allowlist when engine=kjarni; free-form non-empty for ollama/http. |
| Meta | Store `embed_engine` + `embed_model`; clear vectors when either changes; vector search rejects mismatch. |
| Engines | `kjarni` — existing. `ollama` — `POST {base}/api/embed`. `http` — `POST` JSON `{"model","input"}` → `{"embedding"}` or `{"embeddings"}`. |
| Backends | `SEARCHIFY_EMBED_BACKEND=process\|onnx\|none` unchanged (where embeds run). |
| MCP | `index_status` includes `embed_engine`. Bump MCP patch. |
| Acceptance | (1) kjarni path unchanged; (2) ollama/http factory + httptest tests; (3) engine switch clears vectors; (4) `go test ./...`; (5) README documents engines + Ollama example. |

## Out of scope

- Shipping ONNX Runtime native loader in-process (HTTP/Ollama covers remote/local server case)
- Changing hybrid math / HNSW
- Auto-pull Ollama models

---

## Implementation result

*(Developer fills.)*
