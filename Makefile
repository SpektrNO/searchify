.PHONY: build test tidy run

build:
	go build -o bin/searchify ./cmd/searchify

test:
	go test ./...

tidy:
	go mod tidy

run:
	SEARCHIFY_ROOTS="$(SEARCHIFY_ROOTS)" ./bin/searchify mcp stdio
