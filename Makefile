.PHONY: build build-win test tidy run bench

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
