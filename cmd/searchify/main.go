package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/spektr/searchify/internal/config"
	searchmcp "github.com/spektr/searchify/internal/mcp"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "mcp":
		runMCP(os.Args[2:])
	case "serve":
		runServe(os.Args[2:])
	case "index":
		runIndex(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func runMCP(args []string) {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: searchify mcp stdio")
	}
	_ = fs.Parse(args)

	if fs.NArg() != 1 || fs.Arg(0) != "stdio" {
		fs.Usage()
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	server := searchmcp.NewServer(cfg)
	if err := server.RunStdio(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func runServe(args []string) {
	fmt.Fprintln(os.Stderr, "searchify serve http: not implemented yet (phase 5)")
	os.Exit(2)
}

func runIndex(args []string) {
	fmt.Fprintln(os.Stderr, "searchify index: not implemented yet (phase 2)")
	os.Exit(2)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `searchify - hybrid text search MCP server

usage:
  searchify mcp stdio          Run MCP server over stdin/stdout
  searchify serve http         Run MCP server over HTTP (coming soon)
  searchify index <paths...>   Build or refresh local index (coming soon)

environment:
  SEARCHIFY_ROOTS              Required comma-separated allowed search roots
  SEARCHIFY_INDEX_DIR          Index storage path (default: ~/.searchify/index)
  LANGSEARCH_API_KEY           LangSearch API key for web search and rerank
  SEARCHIFY_HTTP_TOKEN         Bearer token for HTTP transport
  SEARCHIFY_EMBED_MODEL        Embedding model name (default: minilm-l6-v2)
`)
}
