package local

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/search"
)

func BenchmarkSearchKeyword(b *testing.B) {
	svc := benchService(b, 1200)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.Search(SearchParams{
			Query: "shard realm",
			Mode:  search.ModeKeyword,
			Limit: 10,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearchHybrid(b *testing.B) {
	svc := benchService(b, 1200)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.Search(SearchParams{
			Query: "how are realms partitioned",
			Mode:  search.ModeHybrid,
			Limit: 10,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchService(b *testing.B, files int) *Service {
	b.Helper()
	root := b.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < files; i++ {
		name := filepath.Join(docs, fmt.Sprintf("doc-%04d.md", i))
		content := fmt.Sprintf("# Doc %d\n\nThe shard_id partitions realms for isolation.\n\nExtra filler text for chunking volume %d.\n", i, i)
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			b.Fatal(err)
		}
	}

	cfg := &config.Config{
		Roots:      []string{root},
		IndexDir:   filepath.Join(b.TempDir(), "index"),
		EmbedModel: "stub",
	}
	svc, err := NewService(cfg)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = svc.Close() })
	svc.embedForTest = &stubEmbedder{}

	if _, err := svc.IndexPaths([]string{docs}, false); err != nil {
		b.Fatal(err)
	}
	return svc
}
