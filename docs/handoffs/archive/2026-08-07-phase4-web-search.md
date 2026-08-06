# Handoff: phase4-web-search

**Status:** done  
**Created:** 2026-08-07  
**Specifier:** spec complete  
**Developer:** complete

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `phase4-web-search` |
| Parent issue | — (create via `create-feature-issues.sh` when wired) |
| Open tasks | — |

Task order: `audit` → `spec` → `engine` → `verify` → `docs`

## Intent

Add a `search_web` MCP tool backed by the LangSearch Web Search API so agents can retrieve live internet results (titles, URLs, LLM-friendly summaries) with clear auth/quota errors and light caching—without leaving the Searchify stdio server.

## Context (phase 3 baseline)

- Local hybrid search is complete (`search_local` keyword/vector/hybrid + optional LangSearch rerank).
- [`internal/rank/rerank.go`](../../internal/rank/rerank.go) already calls LangSearch with Bearer auth; HTTP plumbing is duplicated-ready to share.
- [`internal/web`](../../internal/web/doc.go) is a stub package.
- Config already exposes `LANGSEARCH_API_KEY`.
- Unified result type [`search.Result`](../../internal/search/types.go) already has `URL` and `Source` fields.

## Technical contract

*(See original spec above in git history; implemented as specified.)*

---

## Implementation result

### Changes

- `internal/web`: shared LangSearch `Client` with 429 backoff, TTL cache, `WebSearch`, and `Rerank`
- `search_web` MCP tool (`query`, `limit`, `freshness`, `summary` default true)
- `rank.Rerank` / `RerankWithClient` thin wrappers over `web.Client`
- `index_status` adds `langsearch_configured`
- MCP server version **0.4.0**; README + architecture updated

### Verification

- [x] `go test ./...` (mocked LangSearch: success, cache, 429 retry/exhaust)
- [x] `go build -o bin/searchify ./cmd/searchify`
- [ ] Manual: set `LANGSEARCH_API_KEY`, call `search_web` from Cursor
- [ ] Manual: omit key → clear error
- [ ] Manual: identical query twice → second call fast (cache)

### Deviations from spec

- None

### Follow-ups

- Persist or share quota telemetry in phase 5 if useful
- Archive this handoff after PR merge
