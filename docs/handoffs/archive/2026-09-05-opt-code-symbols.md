# Handoff: opt-code-symbols

**Status:** done  
**Created:** 2026-09-05  
**Specifier:** lean thin handoff  
**Developer:** in-supervisor

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `opt-code-symbols` |
| Parent issue | #60 — https://github.com/SpektrNO/searchify/issues/60 |
| Open tasks | _(none — engine/verify/docs closed)_ |

## Intent

Python-first code-aware chunking and symbol/reference index with MCP `lookup_symbol` / `find_references`, per ADR 001.

## Implementation result

- `internal/code`: Analyzer registry, Python AST worker (embedded script), `ChunkFromUnits`
- Schema v4: `chunk_symbols`, `symbols`, `symbol_refs`; delete with file
- Index path uses analyzer when available; fail-soft to text chunks
- MCP 0.9.0: `lookup_symbol`, `find_references`; `search_local` symbol fields
- Walk skips: venv, `.venv`, `__pycache__`, `.tox`, `.mypy_cache`
- Tests: `internal/code`, `TestIndexPythonSymbols`

## Verification

`go test ./...` passed.

## Spec / ADR

[docs/adr/001-code-symbols.md](../adr/001-code-symbols.md)
