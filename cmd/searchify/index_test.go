package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndexFileErrorsDoNotFailCommand(t *testing.T) {
	root := t.TempDir()
	index := t.TempDir()
	for i := range 25 {
		path := filepath.Join(root, fmt.Sprintf("%02d.png", i))
		if err := os.WriteFile(path, []byte("invalid image"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	good := filepath.Join(root, "z.txt")
	if err := os.WriteFile(good, []byte("searchable text\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	setIndexTestEnv(t, root, index)
	output, err := indexTestCommand(root).CombinedOutput()
	if err != nil {
		t.Fatalf("file errors should not fail command: %v\n%s", err, output)
	}
	text := string(output)
	for _, want := range []string{
		"indexed=1 updated=0 skipped=0 errors=25",
		"completed with 25 file error(s)",
		"ok (new) " + good,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in output:\n%s", want, text)
		}
	}
	for i := range 25 {
		want := "error (skipping file) " + filepath.Join(root, fmt.Sprintf("%02d.png", i)) + ": tesseract not available:"
		if !strings.Contains(text, want) {
			t.Fatalf("missing uncapped file error %q:\n%s", want, text)
		}
	}
}

func TestIndexStartupErrorsFailCommand(t *testing.T) {
	for _, mode := range []string{"configuration", "database"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			index := filepath.Join(t.TempDir(), "index")
			setIndexTestEnv(t, root, index)
			if mode == "configuration" {
				t.Setenv("SEARCHIFY_ROOTS", "")
			} else if err := os.WriteFile(index, []byte("not a directory"), 0o600); err != nil {
				t.Fatal(err)
			}
			output, err := indexTestCommand(root).CombinedOutput()
			if err == nil {
				t.Fatalf("expected startup failure:\n%s", output)
			}
		})
	}
}

func setIndexTestEnv(t *testing.T, root, index string) {
	t.Helper()
	for key, value := range map[string]string{
		"SEARCHIFY_ROOTS":              root,
		"SEARCHIFY_INDEX_DIR":          index,
		"SEARCHIFY_SKIP_EMBED":         "1",
		"SEARCHIFY_EMBED_ENGINE":       "kjarni",
		"SEARCHIFY_EMBED_MODEL":        "minilm-l6-v2",
		"SEARCHIFY_OCR":                "1",
		"SEARCHIFY_EXTRACT_INPROCESS":  "1",
		"SEARCHIFY_TEXT_ONLY":          "0",
		"SEARCHIFY_EXCLUDE_FILES":      "",
		"SEARCHIFY_EXCLUDE_DIRS":       "",
		"SEARCHIFY_WATCH_PATHS":        "",
		"SEARCHIFY_PATH_BASE":          "",
		"SEARCHIFY_TEST_INDEX_PROCESS": "1",
		"PATH":                         "",
	} {
		t.Setenv(key, value)
	}
}

func indexTestCommand(root string) *exec.Cmd {
	return exec.Command(os.Args[0], "-test.run=^TestIndexCommandProcess$", "--", root)
}

func TestIndexCommandProcess(t *testing.T) {
	if os.Getenv("SEARCHIFY_TEST_INDEX_PROCESS") != "1" {
		return
	}
	runIndex([]string{os.Args[len(os.Args)-1]})
	os.Exit(0)
}
