package code

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// GoAnalyzer uses go/parser + go/ast in-process (no worker).
type GoAnalyzer struct{}

func (GoAnalyzer) Lang() string   { return "go" }
func (GoAnalyzer) Exts() []string { return []string{".go"} }

func (GoAnalyzer) Analyze(path string, src []byte) (Result, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		return Result{}, fmt.Errorf("go parse: %w", err)
	}

	file := fset.File(f.Pos())
	if file == nil {
		return Result{}, fmt.Errorf("go parse: missing file set entry")
	}
	srcLen := len(src)

	posSpan := func(n ast.Node) (lineStart, lineEnd, byteStart, byteEnd, col int) {
		start := fset.Position(n.Pos())
		end := fset.Position(n.End())
		lineStart = start.Line
		lineEnd = end.Line
		if lineEnd < lineStart {
			lineEnd = lineStart
		}
		byteStart = start.Offset
		byteEnd = end.Offset
		if byteStart < 0 {
			byteStart = 0
		}
		if byteEnd > srcLen {
			byteEnd = srcLen
		}
		if byteEnd < byteStart {
			byteEnd = byteStart
		}
		col = start.Column
		if col < 1 {
			col = 1
		}
		return
	}

	var units []Unit
	var symbols []Symbol
	var refs []Ref

	addSymbol := func(kind, name, qn string, n ast.Node) {
		ls, le, _, _, col := posSpan(n)
		if qn == "" {
			qn = name
		}
		symbols = append(symbols, Symbol{
			Kind: kind, Name: name, QualName: qn,
			Line: ls, EndLine: le, Col: col,
		})
	}
	addUnit := func(kind, name, qn string, n ast.Node) {
		ls, le, bs, be, col := posSpan(n)
		if qn == "" {
			qn = name
		}
		units = append(units, Unit{
			Kind: kind, Name: name, QualName: qn,
			LineStart: ls, LineEnd: le, ByteStart: bs, ByteEnd: be,
		})
		_ = col
		addSymbol(kind, name, qn, n)
	}

	// Module preamble: package + imports + file docs before first func/type.
	firstBody := srcLen
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			_, _, bs, _, _ := posSpan(d)
			if bs < firstBody {
				firstBody = bs
			}
		case *ast.GenDecl:
			if d.Tok == token.TYPE {
				_, _, bs, _, _ := posSpan(d)
				if bs < firstBody {
					firstBody = bs
				}
			}
		}
	}
	if firstBody > 0 {
		preamble := strings.TrimSpace(string(src[:firstBody]))
		if preamble != "" {
			le := 1
			if firstBody <= srcLen {
				le = fset.Position(file.Pos(firstBody)).Line
				if le < 1 {
					le = 1
				}
			}
			units = append(units, Unit{
				Kind: "module", Name: "", QualName: "",
				LineStart: 1, LineEnd: le, ByteStart: 0, ByteEnd: firstBody,
			})
		}
	}

	recvTypeName := func(fn *ast.FuncDecl) string {
		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			return ""
		}
		return exprTypeName(fn.Recv.List[0].Type)
	}

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			name := d.Name.Name
			if recv := recvTypeName(d); recv != "" {
				addUnit("method", name, recv+"."+name, d)
			} else {
				addUnit("function", name, name, d)
			}
			collectGoRefs(fset, d, &refs)
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				for _, spec := range d.Specs {
					is, ok := spec.(*ast.ImportSpec)
					if !ok || is.Path == nil {
						continue
					}
					pathLit := strings.Trim(is.Path.Value, `"`)
					name := pathLit
					if i := strings.LastIndex(pathLit, "/"); i >= 0 {
						name = pathLit[i+1:]
					}
					if is.Name != nil && is.Name.Name != "" && is.Name.Name != "_" && is.Name.Name != "." {
						name = is.Name.Name
					}
					pos := fset.Position(is.Pos())
					refs = append(refs, Ref{
						Kind: "import", Name: name, QualName: pathLit,
						Line: pos.Line, Col: max1(pos.Column),
					})
				}
				continue
			}
			if d.Tok == token.TYPE {
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok || ts.Name == nil {
						continue
					}
					addUnit("type", ts.Name.Name, ts.Name.Name, ts)
				}
			}
			collectGoRefs(fset, d, &refs)
		default:
			collectGoRefs(fset, decl, &refs)
		}
	}

	return Result{Units: units, Symbols: symbols, Refs: refs}, nil
}

func exprTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return exprTypeName(t.X)
	case *ast.IndexExpr:
		return exprTypeName(t.X)
	case *ast.IndexListExpr:
		return exprTypeName(t.X)
	default:
		return ""
	}
}

func collectGoRefs(fset *token.FileSet, root ast.Node, refs *[]Ref) {
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		switch c := n.(type) {
		case *ast.CallExpr:
			name, qn := callName(c.Fun)
			if name == "" {
				return true
			}
			pos := fset.Position(c.Pos())
			*refs = append(*refs, Ref{
				Kind: "call", Name: name, QualName: qn,
				Line: pos.Line, Col: max1(pos.Column),
			})
		case *ast.SelectorExpr:
			// Skip bare selectors that are not calls; calls handled above.
		case *ast.Ident:
			// Skip — too noisy for v1 name refs.
		}
		return true
	})
}

func callName(fun ast.Expr) (name, qual string) {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name, f.Name
	case *ast.SelectorExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			return f.Sel.Name, id.Name + "." + f.Sel.Name
		}
		return f.Sel.Name, f.Sel.Name
	default:
		return "", ""
	}
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

func init() {
	Register(GoAnalyzer{})
}
