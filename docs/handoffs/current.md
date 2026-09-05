# Handoff: opt-code-symbols-ts

**Status:** implementing  
**Created:** 2026-09-05  
**Specifier:** lean thin handoff  
**Developer:** in-supervisor

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `opt-code-symbols-ts` |
| Parent issue | #71 — https://github.com/SpektrNO/searchify/issues/71 |
| Open tasks | `engine`, `verify`, `docs` |

Task order: `spec` → `engine` → `verify` → `docs`

## Intent

Node worker Analyzer for `.ts` / `.tsx` / `.js` / `.jsx` so TS/JS code gets symbol-aware chunks and `lookup_symbol` / `find_references`, matching ADR 001 (fail-soft when `node` missing).

## Technical contract

| Area | Requirement |
|------|-------------|
| MCP tools | Reuse `lookup_symbol` / `find_references`; bump MCP patch version; mention TS/JS in descriptions |
| Analyzer | `TSAnalyzer` via short-lived Node worker; prefer `typescript` from project `node_modules` (walk-up); else built-in heuristic AST-ish parse |
| Fail-soft | No `node` or worker error → text chunks (existing path) |
| Exts | `.ts` `.tsx` `.js` `.jsx` |
| Acceptance | `go test` covers analyzer (+ index lookup when Node present); docs/backlog ✅ |

## Touchpoints

- `internal/code/` (worker + Go wrapper)
- README / architecture / ADR 001 follow-on table
- MCP tool blurbs + version

## Out of scope

- ts-morph dependency / shipping `node_modules`
- Cross-file type resolution / full LSP
- C# analyzer
