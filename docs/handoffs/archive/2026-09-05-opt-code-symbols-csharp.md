# Handoff: opt-code-symbols-csharp

**Status:** done  
**Created:** 2026-09-05  
**Specifier:** lean thin handoff  
**Developer:** in-supervisor

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `opt-code-symbols-csharp` |
| Parent issue | #76 — https://github.com/SpektrNO/searchify/issues/76 |
| Open tasks | _(none)_ |

## Intent

C# Analyzer for `.cs` → schema v4 + MCP symbol tools.

## Implementation result

- `CSharpAnalyzer`: Roslyn worker (`codeparse_csharp/`) when `dotnet` present; else Go heuristic
- Passthrough allowlist adds `.cs` (and `.jsx`)
- MCP **0.9.3**; tests `TestCSharpAnalyze`, `TestIndexCSharpSymbols`

## Verification

`go test ./...` passed.

## Deviations

ADR said fail-soft to text when `dotnet` missing; shipped heuristic fallback so `.cs` still gets symbols without an SDK.
