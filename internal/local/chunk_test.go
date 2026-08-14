package local

import (
	"strings"
	"testing"
)

func TestChunkSplitsMarkdownHeadings(t *testing.T) {
	text := "# Intro\n\nHello world.\n\n## Details\n\nMore text here.\n"
	chunks, err := chunkFile([]byte(text), ChunkParams{TargetBytes: 3072, OverlapBytes: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected >=2 chunks from headings, got %d %#v", len(chunks), chunks)
	}
	if !strings.Contains(chunks[0].Text, "# Intro") || !strings.Contains(chunks[0].Text, "Hello world") {
		t.Fatalf("chunk0=%q", chunks[0].Text)
	}
	if !strings.Contains(chunks[1].Text, "## Details") {
		t.Fatalf("chunk1=%q", chunks[1].Text)
	}
}

func TestChunkSplitsFormFeedPages(t *testing.T) {
	text := "Page one content.\fPage two content.\n"
	chunks, err := chunkFile([]byte(text), ChunkParams{TargetBytes: 3072, OverlapBytes: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected page split, got %d %#v", len(chunks), chunks)
	}
	if chunks[0].PageStart != 1 {
		t.Fatalf("chunk0 page=%d want 1", chunks[0].PageStart)
	}
	if chunks[1].PageStart != 2 {
		t.Fatalf("chunk1 page=%d want 2", chunks[1].PageStart)
	}
}

func TestChunkNoFormFeedHasNoPage(t *testing.T) {
	text := "# Intro\n\nHello world.\n"
	chunks, err := chunkFile([]byte(text), ChunkParams{TargetBytes: 3072, OverlapBytes: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}
	for _, c := range chunks {
		if c.PageStart != 0 {
			t.Fatalf("markdown should not set page, got %d", c.PageStart)
		}
	}
}

func TestChunkOverlapCarriesSuffix(t *testing.T) {
	// Two paragraphs each ~40 bytes; target forces separate chunks with overlap.
	p1 := strings.Repeat("alpha ", 20) // ~120
	p2 := strings.Repeat("bravo ", 20)
	text := p1 + "\n\n" + p2 + "\n"
	chunks, err := chunkFile([]byte(text), ChunkParams{TargetBytes: 100, OverlapBytes: 30})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	// Second chunk should include some carry from the first.
	if !strings.Contains(chunks[1].Text, "alpha") {
		t.Fatalf("expected overlap carry into chunk1, got %q", chunks[1].Text)
	}
}

func TestChunkSplitsOversizedParagraph(t *testing.T) {
	text := strings.Repeat("x", 5000) + "\n"
	chunks, err := chunkFile([]byte(text), ChunkParams{TargetBytes: 1000, OverlapBytes: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 5 {
		t.Fatalf("expected oversized split, got %d", len(chunks))
	}
	for _, c := range chunks {
		if len(c.Text) > 1100 {
			t.Fatalf("chunk too large: %d", len(c.Text))
		}
	}
}

func TestIsMarkdownHeading(t *testing.T) {
	cases := map[string]bool{
		"# H":       true,
		"##  x":     true,
		"###### x":  true,
		"####### x": false,
		"#nospace":  false,
		"not # h":   false,
		"":          false,
	}
	for in, want := range cases {
		if got := isMarkdownHeading(in); got != want {
			t.Fatalf("isMarkdownHeading(%q)=%v want %v", in, got, want)
		}
	}
}
