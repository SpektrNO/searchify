# ADR 001: Code-aware chunking and symbol index

- **Status:** Accepted (not implemented)
- **Date:** 2026-09-05
- **Backlog:** [`opt-code-symbols`](../feature-backlog.md)
- **Deciders:** Searchify maintainers / Spektr

## Context

Searchify already indexes source as passthrough text (including `.py`), with paragraph/heading/`\f` chunking and hybrid FTS+vector search. That is enough for “find this string,” but weak for codebases: chunks cut mid-function, hits lack symbol identity, and agents cannot ask “where is `X` defined?” or “who references `X`?” without a knowledge graph product.

We want smarter **code retrieval** inside Searchify (not a Graphify-style architecture graph):

1. **Code-aware chunking** — prefer function/class (and method) boundaries.
2. **Symbol / reference index** — defs and best-effort refs for exact lookup tools.

v1 targets **Python**; Go, TypeScript, and C# must fit the same design later. Windows + fail-soft indexing remain constraints (workers over heavy CGO where possible).

## Decision

1. **Extensible `Analyzer` interface** (`internal/code`) per language: `Analyze(path, src) → units, symbols, refs`. Registry by extension; unknown langs keep today’s text chunker.
2. **Python v1 via short-lived AST worker** (stdlib `ast` over JSON), same ops pattern as `searchify extract` / `embed`. If `python3` is missing or the worker fails → **warn and fall back** to text chunking (no hard index failure).
3. **Schema v4 side tables** (do not alter FTS5 columns):
   - `chunk_symbols` — optional symbol/kind on a retrieval chunk (like `chunk_pages`).
   - `symbols` — definitions (`kind`, `name`, `qual_name`, path, line, optional `chunk_id`).
   - `symbol_refs` — best-effort refs (`import` / `call` / `name`); no full type inference in v1.
4. **MCP agent surface:** new tools `lookup_symbol` and `find_references`. Also enrich `search_local` results with optional `symbol` / `symbol_kind` when known.
5. **Walk hygiene:** skip common noise dirs (`venv`, `.venv`, `__pycache__`, `.git`, `node_modules`, …) during index walks.
6. **Out of scope for this ADR:** knowledge graphs, cross-file type resolution, HNSW, shipping Go/TS/C# analyzers (follow-ons only).

### Follow-on language approach (non-binding detail)

| Lang | Expected analyzer | Notes |
|------|-------------------|--------|
| Go | `go/parser` in-process | No worker |
| TypeScript | Node/ts-morph or Tree-sitter worker | PATH or grammar dep |
| C# | Roslyn-oriented worker | `dotnet` on PATH |

Same tables and MCP tools; string `kind` / `lang` values, no schema fork.

## Alternatives considered

| Alternative | Why rejected |
|-------------|----------------|
| **Only enrich `search_local` (no symbol tools)** | Agents need explicit def/ref answers; keyword ranking is a poor substitute for `lookup_symbol`. |
| **In-process Tree-sitter (CGO) for Python** | Strong multi-lang story, but CGO/Windows friction conflicts with current packaging and worker-first ops. Revisit later behind the same interface if needed. |
| **Heuristic regex `def`/`class` only** | Too brittle for Python (decorators, nested defs, async); worker AST is cheap and accurate. |
| **Full knowledge graph (Graphify-like)** | Different product: structure/explain vs retrieve. Keep Searchify as search + symbol tables. |
| **Bake Python into `chunk.go` with no interface** | Blocks Go/TS/C# without rewrite; rejects extensibility goal. |

## Consequences

**Positive**

- Clear extension point for more languages without redesigning MCP or SQLite layout.
- Windows-friendly Python path; fail-soft when Python is absent.
- Agents get dedicated symbol tools; hybrid search still improves via better chunk boundaries.

**Negative / costs**

- Index depends on `python3` for best Python quality (documented; fallback exists).
- Refs are best-effort (especially calls); not an LSP.
- Re-index (`index --force`) required after ship so `chunk_symbols` / symbol tables populate.
- New MCP tools ⇒ version bump (e.g. 0.9.0) when implemented.

**Operational**

- Implementation tracked as backlog `opt-code-symbols` (⬜ until shipped).
- Spec/implement via normal `/spec-only` or `/lean-implement opt-code-symbols` when prioritized.
