package rank

import (
	"context"
	"testing"
)

func TestRerankRequiresAPIKey(t *testing.T) {
	_, err := Rerank(context.Background(), "", "query", []string{"doc"}, 1)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}
