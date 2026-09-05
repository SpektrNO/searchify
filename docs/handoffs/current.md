# Handoff: opt-code-symbols

**Status:** implementing  
**Created:** 2026-09-05  
**Specifier:** lean thin handoff  
**Developer:** in-supervisor

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `opt-code-symbols` |
| Parent issue | #60 — https://github.com/SpektrNO/searchify/issues/60 |
| Open tasks | `engine` #62, `verify` #63, `docs` #64 |

Task order: `spec` → `engine` → `verify` → `docs`

## Intent

Python-first code-aware chunking and a symbol/reference index with MCP `lookup_symbol` / `find_references`, per [ADR 001](../adr/001-code-symbols.md).

## Acceptance

- `.py` files chunk on AST units when `python3` works; otherwise text chunker + warn.
- Schema v4: `chunk_symbols`, `symbols`, `symbol_refs`; cleaned on file delete/reindex.
- MCP `lookup_symbol`, `find_references`; `search_local` may include `symbol` / `symbol_kind`.
- Walk skips `venv`, `.venv`, `__pycache__`, `.tox`, `.mypy_cache` (plus existing skips).
- `go test ./...` passes; MCP **0.9.0**.

## Technical contract

| Area | Requirement |
|------|-------------|
| MCP | `lookup_symbol` (`query`, optional `kind`, `path_prefix`, `limit`); `find_references` (`symbol`, optional `path_prefix`, `limit`) |
| Search | Enrich local hits from `chunk_symbols` |
| Schema | v4 side tables; no FTS5 ALTER |
| Parser | `internal/code` Analyzer; Python AST worker (JSON); fail-soft |

## Out of scope

Knowledge graph; Go/TS/C# analyzers; full type inference; HNSW.

## Spec / ADR

[docs/adr/001-code-symbols.md](../adr/001-code-symbols.md)
