# Handoff: Constrained retrieve (path allow-set)

**Status:** done  
**Created:** 2026-10-28  
**Specifier:** lean (in-supervisor)  
**Developer:** lean (in-supervisor)

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `opt-search-path-allowset` |
| Parent issue | #85 — https://github.com/SpektrNO/searchify/issues/85 |
| Open tasks | _(all closed)_ `spec` #87, `engine` #89, `verify` #90, `docs` #91 |

Task order: `spec` → `engine` → `verify` → `docs`

## Intent

Let ACL-owning clients (e.g. Groundline) pass a path allow-set so Searchify ranks **inside** that set before top-`limit` / RRF — avoiding post-filter starvation. Searchify stays policy-dumb (no identity).

## Technical contract

| Area | Requirement |
|------|-------------|
| MCP / REST | Additive optional `path_prefixes: string[]` and `paths: string[]` on `search_local` and `POST /v1/search` |
| Match | Hit path equals or is under any prefix (path-segment semantics), OR exact `paths` entry; union if both set |
| Timing | Apply in keyword, vector, and hybrid legs **before** scoring/fusion/top-`limit` |
| Omit | Both fields omitted → unscoped (unchanged) |
| Empty | Either field present and allow-set empty → 200 + `results: []` (do not broaden) |
| Roots | Resolve like other APIs; invalid/out-of-root → 400 |
| Modes | `keyword`, `vector`, `hybrid` |
| Acceptance | Allowed hit in; stronger outside hit out; small allow-set fills `limit`; keyword + hybrid tests with `path_prefixes` |

## Touchpoints

- `internal/local/searcher.go` (+ vector load filter), `SearchParams`
- `internal/mcp/tools_search_local.go`, `rest_v1.go`, `executeSearchLocal`
- Tests under `internal/local` and/or `internal/mcp`
- Docs: architecture (done), README search API, backlog ✅

## Out of scope

Identity/ACL policy, Groundline auth, tenant model, post-gen/LLM/audit, changing unscoped default when filters omitted.

---

## Implementation result

### Changes

- `local.PathAllow` + SQL/vector filtering before LIMIT / cosine top-k / RRF
- MCP `search_local` + REST `POST /v1/search` accept `path_prefixes` / `paths`
- Resolve via `AllowlistedCandidates`; out-of-root → client 400
- Tests: keyword exclusion, limit fill, empty allow-set, hybrid, REST empty/constrained/400

### Verification

- [x] `go test ./...`
- [ ] Manual Groundline integration (caller-side)

### Deviations from spec

- None

### Follow-ups

- None required for this feature
