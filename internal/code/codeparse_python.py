# Searchify Python AST worker — stdout JSON {units,symbols,refs}; stdin unused; argv: path
# Invoked as: python3 <script> <file>

from __future__ import annotations

import ast
import json
import sys
from typing import Any


def _qual(parts: list[str]) -> str:
    return ".".join(p for p in parts if p)


def analyze(path: str, src: str) -> dict[str, Any]:
    try:
        tree = ast.parse(src, filename=path)
    except SyntaxError as e:
        raise SystemExit(json.dumps({"error": f"syntax: {e}"})) from e

    lines = src.splitlines(keepends=True)
    # byte offsets per 1-based line start
    line_starts = [0]
    for line in lines:
        line_starts.append(line_starts[-1] + len(line.encode("utf-8")))

    def span(node: ast.AST) -> tuple[int, int, int, int]:
        ls = getattr(node, "lineno", 1) or 1
        le = getattr(node, "end_lineno", ls) or ls
        bs = line_starts[ls - 1] if ls - 1 < len(line_starts) else 0
        be = line_starts[le] if le < len(line_starts) else len(src.encode("utf-8"))
        return ls, le, bs, be

    units: list[dict[str, Any]] = []
    symbols: list[dict[str, Any]] = []
    refs: list[dict[str, Any]] = []

    # Module preamble: bytes before first top-level def/class
    body = list(tree.body)
    first_def_byte = None
    for n in body:
        if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
            _, _, bs, _ = span(n)
            first_def_byte = bs
            break
    if first_def_byte is None:
        first_def_byte = len(src.encode("utf-8"))
    if first_def_byte > 0:
        preamble = src.encode("utf-8")[:first_def_byte].decode("utf-8", errors="replace").strip()
        if preamble:
            # line end of preamble
            le = 1
            for i, start in enumerate(line_starts):
                if start >= first_def_byte:
                    le = max(1, i)
                    break
            else:
                le = max(1, len(lines))
            units.append(
                {
                    "kind": "module",
                    "name": "",
                    "qual_name": "",
                    "line_start": 1,
                    "line_end": le,
                    "byte_start": 0,
                    "byte_end": first_def_byte,
                }
            )

    def add_symbol(kind: str, name: str, qn: str, node: ast.AST) -> None:
        ls, le, _, _ = span(node)
        col = getattr(node, "col_offset", 0) or 0
        symbols.append(
            {
                "kind": kind,
                "name": name,
                "qual_name": qn,
                "line": ls,
                "end_line": le,
                "col": col + 1,
            }
        )

    def add_unit(kind: str, name: str, qn: str, node: ast.AST) -> None:
        ls, le, bs, be = span(node)
        units.append(
            {
                "kind": kind,
                "name": name,
                "qual_name": qn,
                "line_start": ls,
                "line_end": le,
                "byte_start": bs,
                "byte_end": be,
            }
        )
        add_symbol(kind, name, qn, node)

    def walk_refs(node: ast.AST, prefix: list[str]) -> None:
        for child in ast.walk(node):
            if isinstance(child, ast.Import):
                for alias in child.names:
                    name = alias.asname or alias.name
                    refs.append(
                        {
                            "kind": "import",
                            "name": name.split(".")[-1],
                            "qual_name": alias.name,
                            "line": getattr(child, "lineno", 1) or 1,
                            "col": (getattr(child, "col_offset", 0) or 0) + 1,
                        }
                    )
            elif isinstance(child, ast.ImportFrom):
                mod = child.module or ""
                for alias in child.names:
                    if alias.name == "*":
                        continue
                    qn = f"{mod}.{alias.name}" if mod else alias.name
                    refs.append(
                        {
                            "kind": "import",
                            "name": alias.asname or alias.name,
                            "qual_name": qn,
                            "line": getattr(child, "lineno", 1) or 1,
                            "col": (getattr(child, "col_offset", 0) or 0) + 1,
                        }
                    )
            elif isinstance(child, ast.Call):
                fn = child.func
                name, qn = "", ""
                if isinstance(fn, ast.Name):
                    name = fn.id
                    qn = fn.id
                elif isinstance(fn, ast.Attribute):
                    name = fn.attr
                    qn = fn.attr
                    if isinstance(fn.value, ast.Name):
                        qn = f"{fn.value.id}.{fn.attr}"
                if name:
                    refs.append(
                        {
                            "kind": "call",
                            "name": name,
                            "qual_name": qn,
                            "line": getattr(child, "lineno", 1) or 1,
                            "col": (getattr(child, "col_offset", 0) or 0) + 1,
                        }
                    )

    for n in body:
        if isinstance(n, ast.FunctionDef):
            add_unit("function", n.name, n.name, n)
            walk_refs(n, [n.name])
        elif isinstance(n, ast.AsyncFunctionDef):
            add_unit("async_function", n.name, n.name, n)
            walk_refs(n, [n.name])
        elif isinstance(n, ast.ClassDef):
            add_unit("class", n.name, n.name, n)
            walk_refs(n, [n.name])
            for item in n.body:
                if isinstance(item, ast.FunctionDef):
                    qn = _qual([n.name, item.name])
                    add_symbol("method", item.name, qn, item)
                elif isinstance(item, ast.AsyncFunctionDef):
                    qn = _qual([n.name, item.name])
                    add_symbol("method", item.name, qn, item)
        else:
            walk_refs(n, [])

    # Top-level imports already walked via else; also walk module-level once more for imports only
    return {"units": units, "symbols": symbols, "refs": refs}


def main() -> None:
    if len(sys.argv) < 2:
        print(json.dumps({"error": "usage: codeparse_python.py <path>"}), flush=True)
        sys.exit(1)
    path = sys.argv[1]
    with open(path, "r", encoding="utf-8", errors="replace") as f:
        src = f.read()
    out = analyze(path, src)
    print(json.dumps(out), flush=True)


if __name__ == "__main__":
    main()
