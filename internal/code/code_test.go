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

func TestForPathAnalyzers(t *testing.T) {
	if code.ForPath("x.py") == nil {
		t.Fatal("expected python analyzer")
	}
	if code.ForPath("x.go") == nil {
		t.Fatal("expected go analyzer")
	}
	if code.ForPath("x.ts") == nil || code.ForPath("x.tsx") == nil {
		t.Fatal("expected typescript analyzer")
	}
	if code.ForPath("x.js") == nil || code.ForPath("x.jsx") == nil {
		t.Fatal("expected js analyzer")
	}
	if code.ForPath("x.cs") == nil {
		t.Fatal("expected csharp analyzer")
	}
	if code.ForPath("x.rs") != nil {
		t.Fatal("rust analyzer not in v1")
	}
}

func TestGoAnalyze(t *testing.T) {
	src := []byte(`package sample

import "fmt"

func Hello(x int) int {
	return fmt.Sprintf("%d", x)
}

type Greeter struct{}

func (g *Greeter) Greet() {
	Hello(1)
}
`)
	a := code.GoAnalyzer{}
	res, err := a.Analyze("sample.go", src)
	if err != nil {
		t.Fatal(err)
	}
	var sawHello, sawType, sawMethod bool
	for _, u := range res.Units {
		switch u.QualName {
		case "Hello":
			sawHello = true
			if u.Kind != "function" {
				t.Fatalf("Hello kind=%s", u.Kind)
			}
		case "Greeter":
			sawType = true
		case "Greeter.Greet":
			sawMethod = true
			if u.Kind != "method" {
				t.Fatalf("Greet kind=%s", u.Kind)
			}
		}
	}
	if !sawHello || !sawType || !sawMethod {
		t.Fatalf("units=%#v", res.Units)
	}
	var sawImport, sawCall bool
	for _, r := range res.Refs {
		if r.Kind == "import" && strings.Contains(r.QualName, "fmt") {
			sawImport = true
		}
		if r.Kind == "call" && (r.Name == "Hello" || r.QualName == "fmt.Sprintf") {
			sawCall = true
		}
	}
	if !sawImport || !sawCall {
		t.Fatalf("refs=%#v", res.Refs)
	}
}

func TestTSAnalyze(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not on PATH")
	}
	src := []byte(`import { readFile } from "fs";

export function hello(x: number): number {
  return x;
}

export class Greeter {
  greet() {
    hello(1);
  }
}

export const arrowFn = (n: number) => n + 1;
`)
	a := code.TSAnalyzer{}
	res, err := a.Analyze("sample.ts", src)
	if err != nil {
		t.Fatal(err)
	}
	var sawHello, sawClass, sawArrow bool
	for _, u := range res.Units {
		switch u.Name {
		case "hello":
			sawHello = true
		case "Greeter":
			sawClass = true
		case "arrowFn":
			sawArrow = true
		}
	}
	if !sawHello || !sawClass || !sawArrow {
		t.Fatalf("units=%#v", res.Units)
	}
	var sawMethod bool
	for _, s := range res.Symbols {
		if s.QualName == "Greeter.greet" {
			sawMethod = true
		}
	}
	if !sawMethod {
		t.Fatalf("expected Greeter.greet: %#v", res.Symbols)
	}
	var sawImport, sawCall bool
	for _, r := range res.Refs {
		if r.Kind == "import" && r.QualName == "fs" {
			sawImport = true
		}
		if r.Kind == "call" && r.Name == "hello" {
			sawCall = true
		}
	}
	if !sawImport || !sawCall {
		t.Fatalf("refs=%#v", res.Refs)
	}
	chunks := code.ChunkFromUnits(src, res.Units, 3072, 0)
	if len(chunks) < 2 {
		t.Fatalf("chunks=%d", len(chunks))
	}
}

func TestCSharpAnalyze(t *testing.T) {
	src := []byte(`using System;
using System.IO;

namespace Demo;

public class Greeter {
  public int Hello(int x) {
    Console.WriteLine(x);
    return x;
  }
}

public struct Point { }
`)
	a := code.CSharpAnalyzer{}
	res, err := a.Analyze("sample.cs", src)
	if err != nil {
		t.Fatal(err)
	}
	var sawClass, sawStruct bool
	for _, u := range res.Units {
		switch u.Name {
		case "Greeter":
			sawClass = true
			if u.Kind != "class" {
				t.Fatalf("Greeter kind=%s", u.Kind)
			}
		case "Point":
			sawStruct = true
		}
	}
	if !sawClass || !sawStruct {
		t.Fatalf("units=%#v", res.Units)
	}
	var sawMethod bool
	for _, s := range res.Symbols {
		if s.QualName == "Greeter.Hello" {
			sawMethod = true
		}
	}
	if !sawMethod {
		t.Fatalf("expected Greeter.Hello: %#v", res.Symbols)
	}
	var sawImport, sawCall bool
	for _, r := range res.Refs {
		if r.Kind == "import" && (r.QualName == "System" || strings.Contains(r.QualName, "System")) {
			sawImport = true
		}
		if r.Kind == "call" && (r.Name == "WriteLine" || r.QualName == "Console.WriteLine") {
			sawCall = true
		}
	}
	if !sawImport || !sawCall {
		t.Fatalf("refs=%#v", res.Refs)
	}
}
