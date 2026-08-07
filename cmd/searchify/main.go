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
	case "embed":
		runEmbed(os.Args[2:])
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
	defer server.Close()

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
	args = rearrangeFlags(args, map[string]struct{}{"addr": {}, "path": {}})
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
	defer server.Close()

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
	skipEmbed := fs.Bool("skip-embed", false, "FTS/keyword only; do not load ONNX embedder (low RAM)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: searchify index [--force] [--skip-embed] <path...>")
		fmt.Fprintln(os.Stderr, "  Flags may appear before or after paths.")
	}
	args = rearrangeFlags(args, nil)
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}
	if err := rejectDanglingFlags(fs.Args()); err != nil {
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
	if *skipEmbed {
		cfg.SkipEmbed = true
	}

	svc, err := local.NewService(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer svc.Close()

	switch {
	case !cfg.WantVectors():
		fmt.Fprintln(os.Stderr, "index: keyword-only — ONNX/embedder will NOT load (SEARCHIFY_SKIP_EMBED / --skip-embed / SEARCHIFY_EMBED_BACKEND=none)")
	case cfg.UseProcessEmbed():
		fmt.Fprintln(os.Stderr, "index: SEARCHIFY_EMBED_BACKEND=process — FTS then short-lived embed worker per file (child may use multi-GB RAM)")
	case cfg.UseInProcessEmbed():
		fmt.Fprintln(os.Stderr, "index: SEARCHIFY_EMBED_BACKEND=onnx — in-process embeds (high RSS risk on large corpora)")
	}

	start := time.Now()
	report, err := svc.IndexPathsOpts(fs.Args(), local.IndexPathsOptions{
		Force: *force,
		Progress: func(p local.IndexProgress) {
			switch p.Status {
			case "scan":
				fmt.Fprintf(os.Stderr, "index: %d indexable file(s)\n", p.Total)
			case "start":
				fmt.Fprintf(os.Stderr, "[%d/%d] indexing %s\n", p.Current, p.Total, p.Path)
			case "skip":
				fmt.Fprintf(os.Stderr, "[%d/%d] skip %s\n", p.Current, p.Total, p.Path)
			case "indexed":
				fmt.Fprintf(os.Stderr, "[%d/%d] ok (new) %s\n", p.Current, p.Total, p.Path)
			case "updated":
				fmt.Fprintf(os.Stderr, "[%d/%d] ok (updated) %s\n", p.Current, p.Total, p.Path)
			case "empty":
				fmt.Fprintf(os.Stderr, "[%d/%d] empty %s\n", p.Current, p.Total, p.Path)
			case "error":
				fmt.Fprintf(os.Stderr, "[%d/%d] error %s\n", p.Current, p.Total, p.Path)
			}
		},
	})
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

func runEmbed(args []string) {
	fs := flag.NewFlagSet("embed", flag.ExitOnError)
	force := fs.Bool("force", false, "re-embed chunks even when vectors already exist")
	file := fs.String("file", "", "single indexed file path (used by process embed worker)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: searchify embed [--force] [--file path] [path...]")
		fmt.Fprintln(os.Stderr, "  With no paths, embeds all indexed files missing vectors.")
		fmt.Fprintln(os.Stderr, "  Flags may appear before or after paths.")
	}
	args = rearrangeFlags(args, map[string]struct{}{"file": {}})
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}
	if err := rejectDanglingFlags(fs.Args()); err != nil {
		log.Fatal(err)
	}

	paths := append([]string{}, fs.Args()...)
	if strings.TrimSpace(*file) != "" {
		paths = append([]string{strings.TrimSpace(*file)}, paths...)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	// This command always embeds in-process; parent index uses process backend so RSS dies on exit.
	cfg.SkipEmbed = false
	cfg.EmbedBackend = config.EmbedBackendONNX

	svc, err := local.NewService(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer svc.Close()

	start := time.Now()
	report, err := svc.EmbedFiles(paths, local.EmbedOptions{
		Force: *force,
		Progress: func(p local.EmbedProgress) {
			switch p.Status {
			case "scan":
				fmt.Fprintf(os.Stderr, "embed: %d file(s)\n", p.Total)
			case "start":
				fmt.Fprintf(os.Stderr, "[%d/%d] embedding %s\n", p.Current, p.Total, p.Path)
			case "skip":
				fmt.Fprintf(os.Stderr, "[%d/%d] skip %s\n", p.Current, p.Total, p.Path)
			case "ok":
				fmt.Fprintf(os.Stderr, "[%d/%d] ok %s\n", p.Current, p.Total, p.Path)
			case "error":
				fmt.Fprintf(os.Stderr, "[%d/%d] error %s\n", p.Current, p.Total, p.Path)
			}
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("files=%d embedded=%d skipped=%d errors=%d duration_ms=%d\n",
		report.Files, report.Embedded, report.Skipped, report.Errors,
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
	args = rearrangeFlags(args, nil)
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}
	if err := rejectDanglingFlags(fs.Args()); err != nil {
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
		fmt.Fprintln(os.Stderr, "  Flags may appear before or after paths.")
	}
	args = rearrangeFlags(args, nil)
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}
	if err := rejectDanglingFlags(fs.Args()); err != nil {
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
  searchify index [--force] [--skip-embed] <paths...>
                               Build or refresh local keyword index (+ embed worker by default)
  searchify embed [--force] [--file path] [paths...]
                               Backfill chunk vectors (in-process ONNX; prefer after --skip-embed)
  searchify remove <paths...>  Remove files/dirs from the local index
  searchify prune [--dry-run] [paths...]
                               Drop index rows for missing files / out-of-root paths
  searchify serve http [--addr HOST:PORT] [--path /mcp]
                               Run MCP server over Streamable HTTP

environment:
  SEARCHIFY_ROOTS              Required comma-separated allowed search roots
  SEARCHIFY_INDEX_DIR          Index storage path (default: ~/.searchify/index)
  SEARCHIFY_PATH_BASE          Preferred base for relative paths (under a root)
  SEARCHIFY_WATCH_PATHS        Optional auto-index watch paths (under roots)
  SEARCHIFY_WATCH_DEBOUNCE     Watch debounce duration (default 1s)
  SEARCHIFY_WATCH_RESCAN       Optional periodic rescan (e.g. 5m; empty=off)
  SEARCHIFY_OCR                Enable OCR for images / scanned PDFs (1/true/on)
  SEARCHIFY_OCR_LANG           Tesseract language (default eng)
  SEARCHIFY_MAX_FILE_BYTES     Skip source files larger than this (default 2097152)
  SEARCHIFY_MAX_EXTRACT_BYTES  Truncate extracted text (default 524288)
  SEARCHIFY_MAX_CHUNKS_PER_FILE Max chunks per file (default 64)
  SEARCHIFY_EMBED_BATCH        Embedding batch size (default 1)
  SEARCHIFY_EMBED_BACKEND      none|onnx|process (default process: spawn embed worker)
  SEARCHIFY_SKIP_EMBED         1=FTS only, skip ONNX (low RAM; same idea as backend=none)
  SEARCHIFY_EMBED_RELOAD       Close embedder each file when backend=onnx (default on)
  SEARCHIFY_EXTRACT_TIMEOUT    Per-file extract deadline (default 30s)
  LANGSEARCH_API_KEY           LangSearch API key for web search and rerank
  SEARCHIFY_HTTP_TOKEN         Required Bearer token for HTTP transport
  SEARCHIFY_HTTP_ADDR          Default listen address for serve http (:8080)
  SEARCHIFY_EMBED_MODEL        Embedding model name (default: minilm-l6-v2)
`)
}
