// Searchify TS/JS codeparse worker — stdout JSON {units,symbols,refs}
// Invoked as: node <script> <tempSourcePath> [originalPathForResolve]
// Prefers typescript from walk-up node_modules; else brace-aware heuristic (no deps).

import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const enc = new TextEncoder();

function lineStarts(src) {
  const starts = [0];
  for (let i = 0; i < src.length; i++) {
    if (src[i] === "\n") starts.push(i + 1);
  }
  return starts;
}

function lineCol(starts, offset) {
  let lo = 0;
  let hi = starts.length - 1;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if (starts[mid] <= offset) lo = mid;
    else hi = mid - 1;
  }
  return { line: lo + 1, col: offset - starts[lo] + 1 };
}

function byteAt(src, charIndex) {
  return enc.encode(src.slice(0, charIndex)).length;
}

function tryLoadTypescript(resolveFrom) {
  let dir = path.dirname(path.resolve(resolveFrom || "."));
  for (;;) {
    const candidate = path.join(dir, "node_modules", "typescript");
    try {
      return require(candidate);
    } catch {
      /* continue */
    }
    const parent = path.dirname(dir);
    if (parent === dir) break;
    dir = parent;
  }
  try {
    return require("typescript");
  } catch {
    return null;
  }
}

function scriptKind(ts, filePath) {
  const ext = path.extname(filePath).toLowerCase();
  if (ext === ".tsx") return ts.ScriptKind.TSX;
  if (ext === ".jsx") return ts.ScriptKind.JSX;
  if (ext === ".js") return ts.ScriptKind.JS;
  return ts.ScriptKind.TS;
}

function maskNoise(src) {
  let i = 0;
  let out = "";
  while (i < src.length) {
    if (src[i] === "/" && src[i + 1] === "/") {
      out += "  ";
      i += 2;
      while (i < src.length && src[i] !== "\n") {
        out += " ";
        i++;
      }
      continue;
    }
    if (src[i] === "/" && src[i + 1] === "*") {
      out += "  ";
      i += 2;
      while (i < src.length && !(src[i] === "*" && src[i + 1] === "/")) {
        out += src[i] === "\n" ? "\n" : " ";
        i++;
      }
      if (i < src.length) {
        out += "  ";
        i += 2;
      }
      continue;
    }
    const q = src[i];
    if (q === '"' || q === "'" || q === "`") {
      out += " ";
      i++;
      while (i < src.length) {
        if (src[i] === "\\") {
          out += "  ";
          i += 2;
          continue;
        }
        if (src[i] === q) {
          out += " ";
          i++;
          break;
        }
        out += src[i] === "\n" ? "\n" : " ";
        i++;
      }
      continue;
    }
    out += src[i];
    i++;
  }
  return out;
}

function matchBalanced(masked, from, open, close) {
  if (masked[from] !== open) return -1;
  let depth = 0;
  for (let i = from; i < masked.length; i++) {
    if (masked[i] === open) depth++;
    else if (masked[i] === close) {
      depth--;
      if (depth === 0) return i + 1;
    }
  }
  return masked.length;
}

function skipWS(masked, i) {
  while (i < masked.length && /\s/.test(masked[i])) i++;
  return i;
}

function analyzeWithTS(ts, filePath, src) {
  const starts = lineStarts(src);
  const sf = ts.createSourceFile(
    filePath,
    src,
    ts.ScriptTarget.Latest,
    true,
    scriptKind(ts, filePath),
  );

  const units = [];
  const symbols = [];
  const refs = [];

  const toBytes = (charOff) => byteAt(src, charOff);

  const addSymbol = (kind, name, qn, node) => {
    const start = node.getStart(sf);
    const end = node.getEnd();
    const a = lineCol(starts, start);
    const b = lineCol(starts, Math.max(start, end - 1));
    symbols.push({
      kind,
      name,
      qual_name: qn || name,
      line: a.line,
      end_line: b.line,
      col: a.col,
    });
  };

  const addUnit = (kind, name, qn, node) => {
    const start = node.getStart(sf);
    const end = node.getEnd();
    const a = lineCol(starts, start);
    const b = lineCol(starts, Math.max(start, end - 1));
    units.push({
      kind,
      name,
      qual_name: qn || name,
      line_start: a.line,
      line_end: b.line,
      byte_start: toBytes(start),
      byte_end: toBytes(end),
    });
    addSymbol(kind, name, qn || name, node);
  };

  let firstBody = src.length;
  for (const st of sf.statements) {
    if (
      ts.isFunctionDeclaration(st) ||
      ts.isClassDeclaration(st) ||
      ts.isInterfaceDeclaration(st) ||
      ts.isTypeAliasDeclaration(st) ||
      ts.isEnumDeclaration(st)
    ) {
      firstBody = Math.min(firstBody, st.getStart(sf));
    } else if (ts.isVariableStatement(st)) {
      for (const d of st.declarationList.declarations) {
        if (
          d.initializer &&
          (ts.isArrowFunction(d.initializer) ||
            ts.isFunctionExpression(d.initializer))
        ) {
          firstBody = Math.min(firstBody, st.getStart(sf));
        }
      }
    }
  }
  if (firstBody > 0 && src.slice(0, firstBody).trim()) {
    units.push({
      kind: "module",
      name: "",
      qual_name: "",
      line_start: 1,
      line_end: Math.max(1, lineCol(starts, firstBody).line),
      byte_start: 0,
      byte_end: toBytes(firstBody),
    });
  }

  const visitRefs = (node) => {
    if (ts.isImportDeclaration(node) && node.moduleSpecifier) {
      const mod = node.moduleSpecifier.text;
      let name = mod.split("/").pop() || mod;
      if (node.importClause) {
        if (node.importClause.name) name = node.importClause.name.text;
        else if (node.importClause.namedBindings) {
          const nb = node.importClause.namedBindings;
          if (ts.isNamespaceImport(nb)) name = nb.name.text;
          else if (ts.isNamedImports(nb) && nb.elements.length) {
            name = nb.elements[0].name.text;
          }
        }
      }
      const pos = lineCol(starts, node.getStart(sf));
      refs.push({
        kind: "import",
        name,
        qual_name: mod,
        line: pos.line,
        col: pos.col,
      });
    } else if (ts.isCallExpression(node)) {
      const expr = node.expression;
      let name = "";
      let qn = "";
      if (ts.isIdentifier(expr)) {
        name = expr.text;
        qn = expr.text;
      } else if (ts.isPropertyAccessExpression(expr)) {
        name = expr.name.text;
        qn = ts.isIdentifier(expr.expression)
          ? `${expr.expression.text}.${expr.name.text}`
          : name;
      }
      if (name) {
        const pos = lineCol(starts, node.getStart(sf));
        refs.push({
          kind: "call",
          name,
          qual_name: qn,
          line: pos.line,
          col: pos.col,
        });
      }
    }
    ts.forEachChild(node, visitRefs);
  };

  const isAsyncFn = (node) =>
    !!node.modifiers?.some((m) => m.kind === ts.SyntaxKind.AsyncKeyword);

  for (const st of sf.statements) {
    if (ts.isFunctionDeclaration(st) && st.name) {
      addUnit(
        isAsyncFn(st) ? "async_function" : "function",
        st.name.text,
        st.name.text,
        st,
      );
      visitRefs(st);
    } else if (ts.isClassDeclaration(st) && st.name) {
      addUnit("class", st.name.text, st.name.text, st);
      visitRefs(st);
      for (const mem of st.members) {
        if (ts.isMethodDeclaration(mem) && mem.name && ts.isIdentifier(mem.name)) {
          addSymbol("method", mem.name.text, `${st.name.text}.${mem.name.text}`, mem);
        } else if (ts.isConstructorDeclaration(mem)) {
          addSymbol("method", "constructor", `${st.name.text}.constructor`, mem);
        }
      }
    } else if (ts.isInterfaceDeclaration(st) && st.name) {
      addUnit("type", st.name.text, st.name.text, st);
    } else if (ts.isTypeAliasDeclaration(st) && st.name) {
      addUnit("type", st.name.text, st.name.text, st);
    } else if (ts.isEnumDeclaration(st) && st.name) {
      addUnit("type", st.name.text, st.name.text, st);
    } else if (ts.isVariableStatement(st)) {
      let sawFn = false;
      for (const d of st.declarationList.declarations) {
        if (
          d.name &&
          ts.isIdentifier(d.name) &&
          d.initializer &&
          (ts.isArrowFunction(d.initializer) ||
            ts.isFunctionExpression(d.initializer))
        ) {
          sawFn = true;
          addUnit(
            isAsyncFn(d.initializer) ? "async_function" : "function",
            d.name.text,
            d.name.text,
            st,
          );
        }
      }
      visitRefs(st);
      void sawFn;
    } else {
      visitRefs(st);
    }
  }

  return { units, symbols, refs };
}

function analyzeHeuristic(filePath, src) {
  const starts = lineStarts(src);
  const masked = maskNoise(src);
  const units = [];
  const symbols = [];
  const refs = [];

  const addSym = (kind, name, qn, start, end) => {
    const a = lineCol(starts, start);
    const b = lineCol(starts, Math.max(start, end - 1));
    symbols.push({
      kind,
      name,
      qual_name: qn || name,
      line: a.line,
      end_line: b.line,
      col: a.col,
    });
  };

  const addUnit = (kind, name, qn, start, end) => {
    units.push({
      kind,
      name,
      qual_name: qn || name,
      line_start: lineCol(starts, start).line,
      line_end: lineCol(starts, Math.max(start, end - 1)).line,
      byte_start: byteAt(src, start),
      byte_end: byteAt(src, end),
    });
    addSym(kind, name, qn || name, start, end);
  };

  function topLevelDepth(idx) {
    let d = 0;
    for (let i = 0; i < idx; i++) {
      if (masked[i] === "{") d++;
      else if (masked[i] === "}") d--;
    }
    return d;
  }

  function lineStartAt(idx) {
    return masked.lastIndexOf("\n", idx) + 1;
  }

  function skipTypeAnnotation(i) {
    i = skipWS(masked, i);
    if (masked[i] !== ":") return i;
    i++;
    // skip until `{`, `;`, or `=>` at depth 0 for <>()[]
    let angle = 0;
    let paren = 0;
    let bracket = 0;
    while (i < masked.length) {
      const c = masked[i];
      if (c === "<") angle++;
      else if (c === ">") angle--;
      else if (c === "(") paren++;
      else if (c === ")") paren--;
      else if (c === "[") bracket++;
      else if (c === "]") bracket--;
      else if (angle === 0 && paren === 0 && bracket === 0) {
        if (c === "{" || c === ";" || c === "=") return i;
        if (c === "=" && masked[i + 1] === ">") return i;
      }
      i++;
    }
    return i;
  }

  function functionEnd(sigStart) {
    // find '(' of params
    let i = masked.indexOf("(", sigStart);
    if (i < 0) return sigStart;
    i = matchBalanced(masked, i, "(", ")");
    i = skipTypeAnnotation(i);
    i = skipWS(masked, i);
    if (masked[i] === "{") return matchBalanced(masked, i, "{", "}");
    if (masked[i] === ";") return i + 1;
    return i;
  }

  const found = [];

  // function declarations
  const fnRe =
    /(?:^|[\n;{}])(\s*(?:export\s+(?:default\s+)?)?(?:async\s+)?function\s*\*?\s*)([A-Za-z_$][\w$]*)\s*(?:<[^>]*>)?\s*\(/g;
  let m;
  while ((m = fnRe.exec(masked)) !== null) {
    const name = m[2];
    const nameIdx = m.index + m[1].length + (m[0].startsWith("\n") || /[;{}]/.test(m[0][0]) ? 1 : 0);
    // more reliable: search name within match
    const absName = masked.indexOf(name, m.index);
    if (topLevelDepth(absName) !== 0) continue;
    const start = lineStartAt(absName);
    const isAsync = /async\s+function/.test(masked.slice(start, absName + name.length));
    found.push({
      kind: isAsync ? "async_function" : "function",
      name,
      start,
      end: functionEnd(absName),
    });
  }

  // class
  const classRe =
    /(?:^|[\n;{}])(\s*(?:export\s+(?:default\s+)?)?class\s+)([A-Za-z_$][\w$]*)\b/g;
  while ((m = classRe.exec(masked)) !== null) {
    const name = m[2];
    const absName = masked.indexOf(name, m.index);
    if (topLevelDepth(absName) !== 0) continue;
    const start = lineStartAt(absName);
    let i = absName + name.length;
    // skip extends/implements
    i = skipWS(masked, i);
    while (/^(extends|implements)\b/.test(masked.slice(i))) {
      while (i < masked.length && masked[i] !== "{") i++;
      break;
    }
    i = skipWS(masked, i);
    while (i < masked.length && masked[i] !== "{") i++;
    const end = i < masked.length ? matchBalanced(masked, i, "{", "}") : absName + name.length;
    found.push({ kind: "class", name, start, end });
  }

  // interface / type / enum
  const typeRe =
    /(?:^|[\n;{}])(\s*(?:export\s+)?(?:interface|type|enum)\s+)([A-Za-z_$][\w$]*)\b/g;
  while ((m = typeRe.exec(masked)) !== null) {
    const name = m[2];
    const absName = masked.indexOf(name, m.index);
    if (topLevelDepth(absName) !== 0) continue;
    const start = lineStartAt(absName);
    let i = absName + name.length;
    i = skipWS(masked, i);
    // type Alias = ...;
    if (/^\s*type\s/.test(masked.slice(start, absName))) {
      const eq = masked.indexOf("=", absName);
      if (eq >= 0) {
        let j = eq + 1;
        let depth = 0;
        while (j < masked.length) {
          if (masked[j] === "{") depth++;
          else if (masked[j] === "}") depth--;
          else if ((masked[j] === ";" || masked[j] === "\n") && depth === 0) {
            j++;
            break;
          }
          j++;
        }
        found.push({ kind: "type", name, start, end: j });
        continue;
      }
    }
    while (i < masked.length && masked[i] !== "{") i++;
    const end = i < masked.length ? matchBalanced(masked, i, "{", "}") : absName + name.length;
    found.push({ kind: "type", name, start, end });
  }

  // const/let/var name = (... ) => or = async () => or = function
  const varFnRe =
    /(?:^|[\n;{}])(\s*(?:export\s+)?(?:const|let|var)\s+)([A-Za-z_$][\w$]*)\s*=\s*(async\s*)?(?:function\b|\()/g;
  while ((m = varFnRe.exec(masked)) !== null) {
    const name = m[2];
    const absName = masked.indexOf(name, m.index);
    if (topLevelDepth(absName) !== 0) continue;
    const start = lineStartAt(absName);
    const assign = masked.indexOf("=", absName);
    let i = skipWS(masked, assign + 1);
    const isAsync = masked.startsWith("async", i);
    if (isAsync) i = skipWS(masked, i + 5);
    let end;
    if (masked.startsWith("function", i)) {
      end = functionEnd(i);
    } else if (masked[i] === "(") {
      i = matchBalanced(masked, i, "(", ")");
      i = skipWS(masked, i);
      if (masked[i] === "=" && masked[i + 1] === ">") {
        i = skipWS(masked, i + 2);
        if (masked[i] === "{") end = matchBalanced(masked, i, "{", "}");
        else {
          // expression body
          let j = i;
          let pd = 0;
          while (j < masked.length) {
            if (masked[j] === "(") pd++;
            else if (masked[j] === ")") pd--;
            else if ((masked[j] === ";" || masked[j] === ",") && pd === 0) break;
            else if (masked[j] === "\n" && pd === 0) break;
            j++;
          }
          end = j;
        }
      } else continue;
    } else continue;
    found.push({
      kind: isAsync ? "async_function" : "function",
      name,
      start,
      end,
    });
  }

  // dedupe by name+start
  found.sort((a, b) => a.start - b.start);

  let firstChar = src.length;
  for (const f of found) firstChar = Math.min(firstChar, f.start);
  if (firstChar > 0 && src.slice(0, firstChar).trim()) {
    units.push({
      kind: "module",
      name: "",
      qual_name: "",
      line_start: 1,
      line_end: lineCol(starts, firstChar).line,
      byte_start: 0,
      byte_end: byteAt(src, firstChar),
    });
  }

  for (const f of found) {
    addUnit(f.kind, f.name, f.name, f.start, f.end);
    if (f.kind === "class") {
      // Methods at class-body depth only (not nested call sites).
      const openBrace = masked.indexOf("{", f.start);
      const seenMeth = new Set();
      if (openBrace >= 0 && openBrace < f.end) {
        let depth = 0;
        for (let i = openBrace; i < f.end; i++) {
          if (masked[i] === "{") depth++;
          else if (masked[i] === "}") {
            depth--;
            continue;
          }
          if (depth !== 1) continue;
          if (!/[;\n{]/.test(masked[i]) && i !== openBrace) continue;
          const rest = masked.slice(i);
          const mm = rest.match(
            /^[;\n{]\s*(?:(?:public|private|protected|static|async|readonly|override|abstract|get|set)\s+)*(constructor|[A-Za-z_$][\w$]*)\s*(?:<[^>]*>)?\s*\(/,
          );
          if (!mm) continue;
          const meth = mm[1];
          if (["if", "for", "while", "switch", "catch", "function", "class"].includes(meth)) {
            continue;
          }
          const key = `${f.name}.${meth}`;
          if (seenMeth.has(key)) continue;
          seenMeth.add(key);
          const local = i + mm[0].lastIndexOf(meth);
          addSym("method", meth, key, local, local + meth.length);
        }
      }
    }
  }

  // Imports from original source (string masking would erase module paths).
  const importRe =
    /import\s+(?:type\s+)?(?:([\w$]+)|(?:\*\s+as\s+([\w$]+))|(?:\{[^}]*\}))\s+from\s+['"]([^'"]+)['"]|import\s+['"]([^'"]+)['"]/g;
  while ((m = importRe.exec(src)) !== null) {
    const mod = m[3] || m[4] || "";
    const name = m[1] || m[2] || mod.split("/").pop() || mod;
    const pos = lineCol(starts, m.index);
    refs.push({
      kind: "import",
      name,
      qual_name: mod,
      line: pos.line,
      col: pos.col,
    });
  }

  const callRe = /\b([A-Za-z_$][\w$]*)\s*(?:\.\s*([A-Za-z_$][\w$]*))?\s*\(/g;
  const keywords = new Set([
    "if",
    "for",
    "while",
    "switch",
    "catch",
    "function",
    "class",
    "return",
    "typeof",
    "new",
    "await",
  ]);
  while ((m = callRe.exec(masked)) !== null) {
    const before = masked.slice(Math.max(0, m.index - 16), m.index);
    if (/\bfunction\s*$/.test(before)) continue;
    if (m[2]) {
      if (keywords.has(m[1])) continue;
      const pos = lineCol(starts, m.index);
      refs.push({
        kind: "call",
        name: m[2],
        qual_name: `${m[1]}.${m[2]}`,
        line: pos.line,
        col: pos.col,
      });
    } else {
      if (keywords.has(m[1])) continue;
      const pos = lineCol(starts, m.index);
      refs.push({
        kind: "call",
        name: m[1],
        qual_name: m[1],
        line: pos.line,
        col: pos.col,
      });
    }
  }

  void filePath;
  return { units, symbols, refs };
}

function main() {
  const srcPath = process.argv[2];
  const resolveFrom = process.argv[3] || srcPath;
  if (!srcPath) {
    process.stdout.write(
      JSON.stringify({ error: "usage: codeparse_typescript.mjs <path> [resolveFrom]" }),
    );
    process.exit(1);
  }
  let src;
  try {
    src = fs.readFileSync(srcPath, "utf8");
  } catch (e) {
    process.stdout.write(JSON.stringify({ error: String(e.message || e) }));
    process.exit(1);
  }

  const displayPath = resolveFrom || srcPath;
  try {
    const ts = tryLoadTypescript(resolveFrom);
    const out = ts
      ? analyzeWithTS(ts, displayPath, src)
      : analyzeHeuristic(displayPath, src);
    process.stdout.write(JSON.stringify(out));
  } catch (e) {
    process.stdout.write(JSON.stringify({ error: String(e.message || e) }));
    process.exit(1);
  }
}

main();
