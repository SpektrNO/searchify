package config

import "testing"

func TestValidateEmbedModel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", defaultEmbedModel, false},
		{"  minilm-l6-v2 ", EmbedModelMiniLML6V2, false},
		{"MPNet-Base-V2", EmbedModelMPNetBaseV2, false},
		{"distilbert-base", EmbedModelDistilBERT, false},
		{"stub", "", true},
		{"e5-large", "", true},
	}
	for _, tc := range cases {
		got, err := ValidateEmbedModel(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("ValidateEmbedModel(%q) = %q, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ValidateEmbedModel(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ValidateEmbedModel(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
