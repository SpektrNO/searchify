# Handoff: opt-code-symbols-csharp

**Status:** implementing  
**Created:** 2026-09-05  
**Specifier:** lean thin handoff  
**Developer:** in-supervisor

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `opt-code-symbols-csharp` |
| Parent issue | #76 — https://github.com/SpektrNO/searchify/issues/76 |
| Open tasks | `engine`, `verify`, `docs` |

## Intent

C# Analyzer for `.cs` → schema v4 symbols + MCP `lookup_symbol` / `find_references` (ADR 001).

## Technical contract

| Area | Requirement |
|------|-------------|
| Exts | `.cs` |
| Analyzer | Prefer Roslyn via `dotnet` worker when available; else in-process Go heuristic (so indexing still gets symbols without SDK) |
| Fail-soft | Worker/heuristic error → existing text-chunk path |
| MCP | Reuse tools; bump patch; mention C# |
| Acceptance | `go test` covers analyzer + index lookup; docs/backlog ✅ |

## Out of scope

- Full project compilation / cross-file semantics
- Shipping a prebuilt Roslyn binary
