package local

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/search"
)

func TestIndexCSharpSymbols(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "src")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	src := "namespace Demo;\n\npublic class UniqueCsSymType {\n  public int UniqueCsSymFn() { return 1; }\n}\n"
	if err := os.WriteFile(filepath.Join(docs, "Mod.cs"), []byte(src), 0o644); err != nil {
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

	syms, err := svc.LookupSymbol(LookupSymbolParams{Query: "UniqueCsSymType", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) == 0 {
		t.Fatal("expected C# symbol hit")
	}
	if syms[0].Kind != "class" || syms[0].Lang != "csharp" {
		t.Fatalf("got kind=%q lang=%q", syms[0].Kind, syms[0].Lang)
	}

	out, err := svc.Search(SearchParams{Query: "UniqueCsSymType", Mode: search.ModeKeyword, Limit: 5})
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
