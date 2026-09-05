# Setup & run

Short guide to install Searchify and run it as **stdio MCP** or **HTTP**. Full env/reference: [README](../README.md). Stuck: [troubleshooting](./troubleshooting.md).

## 1. Prerequisites

| Need | Notes |
|------|--------|
| **Go 1.25+** | Build only |
| **`SEARCHIFY_ROOTS`** | Comma-separated absolute dirs you allow Searchify to read (siblings OK; nested paths collapse to the outer one) |
| **Poppler** (`pdftotext`) | Recommended for PDFs |
| Optional | `python3` / `node` / `dotnet` for richer code symbols; `LANGSEARCH_API_KEY` for `search_web` |

```bash
# Ubuntu/Debian
sudo apt install poppler-utils
```

Windows: Scoop `scoop install poppler`, or see README → Install Poppler.

## 2. Build

```bash
git clone https://github.com/SpektrNO/searchify.git
cd searchify
go mod tidy
go build -o bin/searchify ./cmd/searchify
# Windows exe from WSL/Linux: make build-win → bin/searchify.exe
```

Put `bin/searchify` (or `searchify.exe`) somewhere stable and on `PATH` if you like.

## 3. Index once

Serving does **not** crawl roots by itself. Index first:

```bash
export SEARCHIFY_ROOTS="/path/to/code,/path/to/docs"   # one or more roots
./bin/searchify index --skip-embed "$SEARCHIFY_ROOTS"  # fast FTS-only first pass
# later, for hybrid/vector search:
# ./bin/searchify embed --force
```

Default index DB: `~/.searchify/index` (override with `SEARCHIFY_INDEX_DIR`).

## 4. Run as MCP (stdio) — local Cursor

Recommended for a machine that can spawn the binary.

```bash
export SEARCHIFY_ROOTS="/path/to/code,/path/to/docs"
./bin/searchify mcp stdio
```

Cursor / MCP client config (edit paths):

```json
{
  "mcpServers": {
    "searchify": {
      "command": "/ABS/PATH/TO/bin/searchify",
      "args": ["mcp", "stdio"],
      "env": {
        "SEARCHIFY_ROOTS": "/path/to/code,/path/to/docs",
        "SEARCHIFY_EMBED_MODEL": "minilm-l6-v2"
      }
    }
  }
}
```

Optional in `env`: `LANGSEARCH_API_KEY`, `SEARCHIFY_INDEX_DIR`, `SEARCHIFY_PATH_BASE`, `SEARCHIFY_WATCH_PATHS`.

## 5. Run as HTTP server

Same process serves **Streamable HTTP MCP** (`/mcp` by default), **`/healthz`**, and REST `/v1/*`. Requires a Bearer token.

```bash
export SEARCHIFY_ROOTS="/path/to/code,/path/to/docs"
export SEARCHIFY_HTTP_TOKEN="change-me"
./bin/searchify serve http --addr 127.0.0.1:8080 --path /mcp
# curl -sS http://127.0.0.1:8080/healthz
```

Bind to localhost unless you terminate TLS on a reverse proxy.

### Cursor → HTTP MCP

```json
{
  "mcpServers": {
    "searchify-http": {
      "url": "http://127.0.0.1:8080/mcp",
      "headers": {
        "Authorization": "Bearer change-me"
      }
    }
  }
}
```

### REST (same token)

```bash
curl -sS -X POST http://127.0.0.1:8080/v1/search \
  -H "Authorization: Bearer change-me" \
  -H "Content-Type: application/json" \
  -d '{"query":"example","mode":"hybrid","limit":10}'
```

Also: `POST /v1/index`, `GET /v1/files`, `GET /v1/stats`.

## 6. Typical loop

1. Set `SEARCHIFY_ROOTS` → **build** → **index**  
2. Either **stdio MCP** in Cursor, or **`serve http`** + HTTP MCP / REST  
3. After adding files: re-run `index` (or set `SEARCHIFY_WATCH_PATHS` for live updates while serving)

## Windows note

Copy `scripts/searchify-env.example.bat`, set roots/token, `call` it, then run `searchify.exe index` / `mcp stdio` / `serve http`. Details in README.
