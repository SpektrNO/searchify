package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/extract"
)

func collectIndexablePaths(cfg *config.Config, reg *extract.Registry, roots []string) ([]string, []string) {
	var files, messages []string
	for path := range indexablePaths(context.Background(), cfg, reg, roots, func(msg string) {
		if len(messages) < maxIndexMessages {
			messages = append(messages, msg)
		}
	}) {
		files = append(files, path)
	}
	return files, messages
}

func TestIndexablePathsStreamsAcrossDirectoryBatches(t *testing.T) {
	w, _, root := testQueueWatcher(t)
	for i := range directoryBatchSize*2 + 1 {
		path := filepath.Join(root, fmt.Sprintf("%03d.md", i))
		if err := os.WriteFile(path, []byte("text\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for range indexablePaths(context.Background(), w.cfg, w.svc.extract, []string{root}, func(msg string) {
		t.Error(msg)
	}) {
		count++
	}
	if count != directoryBatchSize*2+1 {
		t.Fatalf("count=%d", count)
	}
	count = 0
	for range indexablePaths(context.Background(), w.cfg, w.svc.extract, []string{root}, func(msg string) {
		t.Error(msg)
	}) {
		count++
		if count == 3 {
			break
		}
	}
	if count != 3 {
		t.Fatalf("early stop count=%d", count)
	}
}

func TestIndexScanDiagnosticsAreBounded(t *testing.T) {
	w, _, root := testQueueWatcher(t)
	for i := range maxIndexMessages * 3 {
		path := filepath.Join(root, fmt.Sprintf("%d.md", i))
		if err := os.WriteFile(path, make([]byte, 1025), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	report, err := w.svc.IndexPaths([]string{root}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Messages) != maxIndexMessages || report.Indexed != 0 {
		t.Fatalf("report=%+v", report)
	}
}

func TestIndexCancellationAfterScanAndExtraction(t *testing.T) {
	for _, stage := range []string{"scan", "extract"} {
		t.Run(stage, func(t *testing.T) {
			w, _, root := testQueueWatcher(t)
			path := filepath.Join(root, "cancel.docx")
			if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			w.svc.spawnExtractForTest = func(string) (string, []string, error) {
				if stage == "extract" {
					cancel()
				}
				return "cancelled contents\n", nil, nil
			}
			report, err := w.svc.IndexPathsOpts([]string{root}, IndexPathsOptions{
				Context: ctx,
				Progress: func(p IndexProgress) {
					if stage == "scan" && p.Status == "scan" {
						cancel()
					}
				},
			})
			if !errors.Is(err, context.Canceled) || report.Indexed != 0 {
				t.Fatalf("report=%+v err=%v", report, err)
			}
			n, err := w.svc.DocumentCount()
			if err != nil || n != 0 {
				t.Fatalf("cancelled work persisted: count=%d err=%v", n, err)
			}
		})
	}
}

func TestIndexProgressPreservesTotals(t *testing.T) {
	w, _, root := testQueueWatcher(t)
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("content\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var scan, completed int
	report, err := w.svc.IndexPathsOpts([]string{root}, IndexPathsOptions{
		Progress: func(p IndexProgress) {
			if p.Total != 3 {
				t.Errorf("total=%d, want 3", p.Total)
			}
			if p.Status == "scan" {
				scan++
			}
			if p.Status == "indexed" {
				completed++
				if p.Current != completed {
					t.Errorf("current=%d want %d", p.Current, completed)
				}
			}
		},
	})
	if err != nil || report.Indexed != 3 || scan != 1 || completed != 3 {
		t.Fatalf("report=%+v scan=%d completed=%d err=%v", report, scan, completed, err)
	}
}

func TestIndexCancellationDuringEmbeddingCanRetry(t *testing.T) {
	w, _, root := testQueueWatcher(t)
	w.cfg.SkipEmbed = false
	w.cfg.EmbedBackend = config.EmbedBackendProcess
	path := filepath.Join(root, "retry.md")
	if err := os.WriteFile(path, []byte("retry contents\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.svc.spawnEmbedForTest = func(string) error {
		cancel()
		return ctx.Err()
	}
	_, err := w.svc.IndexPathsOpts([]string{path}, IndexPathsOptions{Context: ctx})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	n, err := w.svc.DocumentCount()
	if err != nil || n != 0 {
		t.Fatalf("interrupted index not cleaned: count=%d err=%v", n, err)
	}
	w.svc.spawnEmbedForTest = func(string) error { return nil }
	report, err := w.svc.IndexPaths([]string{path}, false)
	if err != nil || report.Indexed != 1 || report.Skipped != 0 {
		t.Fatalf("retry report=%+v err=%v", report, err)
	}
}
