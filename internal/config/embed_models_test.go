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

func TestResolveEmbedSettings(t *testing.T) {
	t.Parallel()
	eng, model, url, err := ResolveEmbedSettings("", "", "")
	if err != nil || eng != EmbedEngineKjarni || model != defaultEmbedModel || url != "" {
		t.Fatalf("kjarni default: eng=%q model=%q url=%q err=%v", eng, model, url, err)
	}

	eng, model, url, err = ResolveEmbedSettings("ollama", "nomic-embed-text", "")
	if err != nil || eng != EmbedEngineOllama || model != "nomic-embed-text" || url != defaultOllamaURL {
		t.Fatalf("ollama: eng=%q model=%q url=%q err=%v", eng, model, url, err)
	}

	_, _, _, err = ResolveEmbedSettings("ollama", "", "")
	if err == nil {
		t.Fatal("ollama without model should fail")
	}

	_, _, _, err = ResolveEmbedSettings("http", "my-model", "")
	if err == nil {
		t.Fatal("http without URL should fail")
	}

	eng, model, url, err = ResolveEmbedSettings("http", "my-model", "http://127.0.0.1:8081/v1/embeddings")
	if err != nil || eng != EmbedEngineHTTP || model != "my-model" || url != "http://127.0.0.1:8081/v1/embeddings" {
		t.Fatalf("http: eng=%q model=%q url=%q err=%v", eng, model, url, err)
	}

	_, _, _, err = ResolveEmbedSettings("onnx", "x", "")
	if err == nil {
		t.Fatal("invalid engine should fail")
	}
}
