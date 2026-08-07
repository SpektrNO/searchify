# Handoff archive: phase5-http-hardening

Completed 2026-08-07.

## Changes

- `searchify serve http` with Streamable HTTP (stateless), Bearer auth, `/healthz`, timeouts, slog logging
- MCP server version 0.5.0; `SEARCHIFY_HTTP_ADDR` config
- Benchmarks: `BenchmarkSearchKeyword` / `BenchmarkSearchHybrid`
- Tests: healthz, 401 without token, authorized initialize smoke

## Follow-ups

- Resolve relative `search_file` paths against workspace / roots, not MCP process cwd
