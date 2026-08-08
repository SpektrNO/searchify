package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/extract"
	"github.com/spektr/searchify/internal/local"
)

func runExtract(args []string) {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	file := fs.String("file", "", "absolute path under SEARCHIFY_ROOTS to extract")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: searchify extract --file <path>")
		fmt.Fprintln(os.Stderr, "  Prints one JSON object on stdout (used by index extract worker).")
	}
	args = rearrangeFlags(args, map[string]struct{}{"file": {}})
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	path := strings.TrimSpace(*file)
	if path == "" && fs.NArg() == 1 {
		path = fs.Arg(0)
	}
	if path == "" {
		fs.Usage()
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		writeExtractFail(err.Error(), nil, false)
		os.Exit(1)
	}
	cfg.ExtractInProcess = true
	cfg.SkipEmbed = true

	allowed, err := cfg.AllowedPath(path)
	if err != nil {
		writeExtractFail(err.Error(), nil, false)
		os.Exit(1)
	}

	svc, err := local.NewService(cfg)
	if err != nil {
		writeExtractFail(err.Error(), nil, false)
		os.Exit(1)
	}
	defer svc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), svc.ExtractTimeoutOrDefault())
	defer cancel()

	text, warn, err := svc.ExtractFileText(ctx, allowed)
	if err != nil {
		var skip *extract.SkipError
		if errors.As(err, &skip) {
			writeExtractFail(skip.Error(), warn, true)
			os.Exit(0)
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			writeExtractFail("extract timed out", warn, true)
			os.Exit(0)
		}
		writeExtractFail(err.Error(), warn, false)
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"ok":   true,
		"text": text,
		"warn": warn,
	})
}

func writeExtractFail(msg string, warn []string, skip bool) {
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"ok":    false,
		"error": msg,
		"warn":  warn,
		"skip":  skip,
	})
}
