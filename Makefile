.PHONY: build build-win test tidy run run-http bench curl-http

build:
	go build -o bin/searchify ./cmd/searchify

# Cross-compile a Windows amd64 binary (run from Linux/WSL/macOS).
build-win:
	GOOS=windows GOARCH=amd64 go build -o bin/searchify.exe ./cmd/searchify

test:
	go test ./...

tidy:
	go mod tidy

bench:
	go test -bench=BenchmarkSearch -benchmem ./internal/local/

# Loads .env if present (for SEARCHIFY_*). Cursor MCP uses .cursor/mcp.json instead.
run: build
	@set -a; \
	[ -f .env ] && . ./.env; \
	set +a; \
	if [ -z "$${SEARCHIFY_ROOTS}" ]; then \
		echo 'error: SEARCHIFY_ROOTS is required. Set it in .env or run: make run SEARCHIFY_ROOTS=/path' >&2; \
		exit 1; \
	fi; \
	./bin/searchify mcp stdio

run-http: build
	@set -a; \
	[ -f .env ] && . ./.env; \
	set +a; \
	if [ -z "$${SEARCHIFY_ROOTS}" ]; then \
		echo 'error: SEARCHIFY_ROOTS is required. Set it in .env or the environment.' >&2; \
		exit 1; \
	fi; \
	if [ -z "$${SEARCHIFY_HTTP_TOKEN}" ]; then \
		echo 'error: SEARCHIFY_HTTP_TOKEN is required. Set it in .env or the environment.' >&2; \
		exit 1; \
	fi; \
	addr="$${SEARCHIFY_HTTP_ADDR:-127.0.0.1:8080}"; \
	./bin/searchify serve http --addr "$$addr"

curl-http:
	curl -X POST http://127.0.0.1:8080/mcp \
		-H "Authorization: Bearer $${SEARCHIFY_HTTP_TOKEN}" \
		-H "Content-Type: application/json" \
		-d '{"jsonrpc": "2.0", "method": "mcp.describe", "params": {}}'