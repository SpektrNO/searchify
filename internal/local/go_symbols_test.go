package local

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/search"
)

func TestIndexGoSymbols(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "pkg")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	src := "package pkg\n\nfunc UniqueGoSymFn() int { return 1 }\n\ntype UniqueGoSymType struct{}\n"
	if err := os.WriteFile(filepath.Join(docs, "mod.go"), []byte(src), 0o644); err != nil {
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

	if _, err := svc.IndexPaths([]string{docs}, false); err != nil {
		t.Fatal(err)
	}

	syms, err := svc.LookupSymbol(LookupSymbolParams{Query: "UniqueGoSymFn", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) == 0 {
		t.Fatal("expected Go symbol hit")
	}
	if syms[0].Kind != "function" || syms[0].Lang != "go" {
		t.Fatalf("got kind=%q lang=%q", syms[0].Kind, syms[0].Lang)
	}

	out, err := svc.Search(SearchParams{Query: "UniqueGoSymFn", Mode: search.ModeKeyword, Limit: 5})
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
