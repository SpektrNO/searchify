package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/local"
	searchmcp "github.com/spektr/searchify/internal/mcp"
)

func main() {
	log.SetFlags(0)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

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
	case "remove":
		runRemove(os.Args[2:])
	case "prune":
		runPrune(os.Args[2:])
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
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "", "listen address (default SEARCHIFY_HTTP_ADDR or :8080)")
	path := fs.String("path", "/mcp", "MCP Streamable HTTP path")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: searchify serve http [--addr HOST:PORT] [--path /mcp]")
	}
	_ = fs.Parse(args)

	if fs.NArg() != 1 || fs.Arg(0) != "http" {
		fs.Usage()
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if strings.TrimSpace(cfg.HTTPToken) == "" {
		slog.Error("SEARCHIFY_HTTP_TOKEN is required for HTTP mode")
		os.Exit(1)
	}

	listenAddr := *addr
	if listenAddr == "" {
		listenAddr = cfg.HTTPAddr
	}

	server, err := searchmcp.NewServer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer server.Local().Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.RunHTTP(ctx, searchmcp.HTTPOptions{
		Addr: listenAddr,
		Path: *path,
	}); err != nil && err != context.Canceled {
		log.Fatal(err)
	}
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

	start := time.Now()
	report, err := svc.IndexPaths(fs.Args(), *force)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("indexed=%d updated=%d skipped=%d errors=%d duration_ms=%d\n",
		report.Indexed, report.Updated, report.Skipped, report.Errors,
		int(time.Since(start)/time.Millisecond))

	for _, msg := range report.Messages {
		fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
	}
	if report.Errors > 0 {
		os.Exit(1)
	}
}

func runRemove(args []string) {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: searchify remove <path...>")
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

	start := time.Now()
	report, err := svc.RemovePaths(fs.Args())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("removed=%d skipped=%d errors=%d duration_ms=%d\n",
		report.Removed, report.Skipped, report.Errors,
		int(time.Since(start)/time.Millisecond))

	for _, msg := range report.Messages {
		fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
	}
	if report.Errors > 0 {
		os.Exit(1)
	}
}

func runPrune(args []string) {
	fs := flag.NewFlagSet("prune", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "report orphans without deleting")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: searchify prune [--dry-run] [path...]")
	}
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
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

	start := time.Now()
	report, err := svc.PruneIndex(fs.Args(), *dryRun)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("scanned=%d removed=%d skipped=%d errors=%d dry_run=%v duration_ms=%d\n",
		report.Scanned, report.Removed, report.Skipped, report.Errors, report.DryRun,
		int(time.Since(start)/time.Millisecond))

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
  searchify remove <paths...>  Remove files/dirs from the local index
  searchify prune [--dry-run] [paths...]
                               Drop index rows for missing files / out-of-root paths
  searchify serve http [--addr HOST:PORT] [--path /mcp]
                               Run MCP server over Streamable HTTP

environment:
  SEARCHIFY_ROOTS              Required comma-separated allowed search roots
  SEARCHIFY_INDEX_DIR          Index storage path (default: ~/.searchify/index)
  SEARCHIFY_PATH_BASE          Preferred base for relative paths (under a root)
  LANGSEARCH_API_KEY           LangSearch API key for web search and rerank
  SEARCHIFY_HTTP_TOKEN         Required Bearer token for HTTP transport
  SEARCHIFY_HTTP_ADDR          Default listen address for serve http (:8080)
  SEARCHIFY_EMBED_MODEL        Embedding model name (default: minilm-l6-v2)
`)
}
