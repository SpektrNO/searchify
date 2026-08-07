package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllowedPathAbsoluteUnderRoot(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "README.md")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{Roots: []string{root}}
	got, err := cfg.AllowedPath(file)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(file) {
		t.Fatalf("got %q want %q", got, file)
	}
}

func TestAllowedPathRelativeUnique(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "README.md")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{Roots: []string{root}}
	got, err := cfg.AllowedPath("README.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(file) {
		t.Fatalf("got %q want %q", got, file)
	}
}

func TestAllowedPathRelativeMissing(t *testing.T) {
	cfg := &Config{Roots: []string{t.TempDir()}}
	_, err := cfg.AllowedPath("nope.md")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAllowedPathRelativeAmbiguous(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	for _, root := range []string{a, b} {
		if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &Config{Roots: []string{a, b}}
	_, err := cfg.AllowedPath("README.md")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), a) || !strings.Contains(err.Error(), b) {
		t.Fatalf("error should list candidates: %v", err)
	}
}

func TestAllowedPathDotDotRejected(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "secret.txt")
	if err := os.WriteFile(outside, []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{Roots: []string{root}}
	_, err := cfg.AllowedPath("../secret.txt")
	if err == nil {
		t.Fatal("expected escape to be rejected")
	}
}

func TestAllowedPathBasePreferred(t *testing.T) {
	root := t.TempDir()
	projA := filepath.Join(root, "a")
	projB := filepath.Join(root, "b")
	for _, d := range []string{projA, projB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "README.md"), []byte(d), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &Config{Roots: []string{root}, PathBase: projB}
	got, err := cfg.AllowedPath("README.md")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(projB, "README.md")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestParsePathBaseMustBeUnderRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	_, err := parsePathBase(outside, []string{root})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAllowlistedCandidatesMissingOK(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "gone.md")
	cfg := &Config{Roots: []string{root}}

	got, err := cfg.AllowlistedCandidates(missing)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != filepath.Clean(missing) {
		t.Fatalf("got %v", got)
	}

	got, err = cfg.AllowlistedCandidates("gone.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != filepath.Clean(missing) {
		t.Fatalf("got %v", got)
	}
}

func TestParseWatchPathsMustBeUnderRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	_, err := parseWatchPaths(outside, []string{root})
	if err == nil {
		t.Fatal("expected error for outside watch path")
	}
}
