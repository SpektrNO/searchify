package local

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/search"
)

func TestIndexPythonSymbols(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "pkg")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	src := "def unique_sym_fn():\n    return 1\n\nclass UniqueSymCls:\n    pass\n"
	if err := os.WriteFile(filepath.Join(docs, "mod.py"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Roots:      []string{root},
		IndexDir:   t.TempDir(),
		EmbedModel: "stub",
		SkipEmbed:  true,
	}
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	report, err := svc.IndexPaths([]string{docs}, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Indexed < 1 && report.Updated < 1 {
		t.Fatalf("report=%+v", report)
	}

	syms, err := svc.LookupSymbol(LookupSymbolParams{Query: "unique_sym_fn", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) == 0 {
		// Analyzer may have fallen back if python missing.
		if strings.Contains(strings.Join(report.Messages, " "), "code analyze failed") ||
			strings.Contains(strings.Join(report.Messages, " "), "python") {
			t.Skip("python analyze unavailable")
		}
		t.Fatal("expected symbol hit")
	}
	if syms[0].Kind != "function" {
		t.Fatalf("kind=%q", syms[0].Kind)
	}

	out, err := svc.Search(SearchParams{Query: "unique_sym_fn", Mode: search.ModeKeyword, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) == 0 {
		t.Fatal("expected search hit")
	}
	if out.Results[0].Symbol == "" {
		t.Fatalf("expected symbol on hit, title=%q", out.Results[0].Title)
	}
}
