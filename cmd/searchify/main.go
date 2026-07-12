package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/local"
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

	server, err := searchmcp.NewServer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer server.Local().Close()

	if err := server.RunStdio(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func runServe(args []string) {
	fmt.Fprintln(os.Stderr, "searchify serve http: not implemented yet (phase 5)")
	os.Exit(2)
}

func runIndex(args []string) {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	force := fs.Bool("force", false, "re-index files even when metadata is unchanged")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: searchify index [--force] <path...>")
	}
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}
	if fs.NArg() == 0 {
		fs.Usage()
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	svc, err := local.NewService(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer svc.Close()

	report, err := svc.IndexPaths(fs.Args(), *force)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("indexed=%d updated=%d skipped=%d errors=%d\n",
		report.Indexed, report.Updated, report.Skipped, report.Errors)
	for _, msg := range report.Messages {
		fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
	}
	if report.Errors > 0 {
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `searchify - hybrid text search MCP server

usage:
  searchify mcp stdio          Run MCP server over stdin/stdout
  searchify index [--force] <paths...>
                               Build or refresh local keyword index
  searchify serve http         Run MCP server over HTTP (coming soon)

environment:
  SEARCHIFY_ROOTS              Required comma-separated allowed search roots
  SEARCHIFY_INDEX_DIR          Index storage path (default: ~/.searchify/index)
  LANGSEARCH_API_KEY           LangSearch API key for web search and rerank
  SEARCHIFY_HTTP_TOKEN         Bearer token for HTTP transport
  SEARCHIFY_EMBED_MODEL        Embedding model name (default: minilm-l6-v2)
`)
}
