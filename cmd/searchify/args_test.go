package main

import (
	"reflect"
	"testing"
)

func TestRearrangeFlagsPutsOptionsFirst(t *testing.T) {
	got := rearrangeFlags([]string{`C:\vault`, "--skip-embed", "--force"}, nil)
	want := []string{"--skip-embed", "--force", `C:\vault`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestRearrangeFlagsValueFlag(t *testing.T) {
	vf := map[string]struct{}{"file": {}, "addr": {}}
	got := rearrangeFlags([]string{"docs", "--file", "a.md", "--force"}, vf)
	want := []string{"--file", "a.md", "--force", "docs"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestRearrangeFlagsEqualsForm(t *testing.T) {
	vf := map[string]struct{}{"file": {}}
	got := rearrangeFlags([]string{"docs", "--file=a.md"}, vf)
	want := []string{"--file=a.md", "docs"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestRearrangeFlagsDoubleDash(t *testing.T) {
	got := rearrangeFlags([]string{"--skip-embed", "--", "-weird"}, nil)
	want := []string{"--skip-embed", "-weird"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}
