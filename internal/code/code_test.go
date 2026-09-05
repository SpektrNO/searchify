package code_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/spektr/searchify/internal/code"
)

func TestPythonAnalyzeAndChunk(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		if _, err2 := exec.LookPath("python"); err2 != nil {
			t.Skip("python3 not on PATH")
		}
	}
	src := []byte("" +
		"import os\n\n" +
		"def hello(x):\n" +
		"    return x\n\n" +
		"class Greeter:\n" +
		"    def greet(self):\n" +
		"        hello(1)\n")
	a := code.PythonAnalyzer{}
	res, err := a.Analyze("sample.py", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Units) < 2 {
		t.Fatalf("units=%d %#v", len(res.Units), res.Units)
	}
	var sawHello, sawClass bool
	for _, u := range res.Units {
		if u.Name == "hello" {
			sawHello = true
		}
		if u.Name == "Greeter" {
			sawClass = true
		}
	}
	if !sawHello || !sawClass {
		t.Fatalf("missing units: %#v", res.Units)
	}
	var sawMethod bool
	for _, s := range res.Symbols {
		if s.QualName == "Greeter.greet" {
			sawMethod = true
		}
	}
	if !sawMethod {
		t.Fatalf("expected Greeter.greet symbol: %#v", res.Symbols)
	}
	chunks := code.ChunkFromUnits(src, res.Units, 3072, 0)
	if len(chunks) < 2 {
		t.Fatalf("chunks=%d", len(chunks))
	}
	joined := ""
	for _, c := range chunks {
		joined += c.Text
	}
	if !strings.Contains(joined, "def hello") {
		t.Fatalf("missing hello in chunks")
	}
}

func TestForPathPython(t *testing.T) {
	if code.ForPath("x.py") == nil {
		t.Fatal("expected python analyzer")
	}
	if code.ForPath("x.go") != nil {
		t.Fatal("go analyzer not in v1")
	}
}
