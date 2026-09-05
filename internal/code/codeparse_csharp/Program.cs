// Searchify C# Roslyn worker — stdout JSON {units,symbols,refs}
// Invoked as: dotnet run --project <dir> -- <source.cs>
using System.Text;
using System.Text.Json;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;

if (args.Length < 1)
{
    Console.WriteLine(JsonSerializer.Serialize(new { error = "usage: Program <path.cs>" }));
    return 1;
}

var path = args[0];
string src;
try
{
    src = File.ReadAllText(path);
}
catch (Exception ex)
{
    Console.WriteLine(JsonSerializer.Serialize(new { error = ex.Message }));
    return 1;
}

try
{
    var tree = CSharpSyntaxTree.ParseText(src, path: path);
    var root = tree.GetCompilationUnitRoot();
    var starts = LineStarts(src);

    var units = new List<object>();
    var symbols = new List<object>();
    var refs = new List<object>();

    void AddSymbol(string kind, string name, string qn, SyntaxNode node)
    {
        var span = node.Span;
        var (ls, cs) = LineCol(starts, span.Start);
        var (le, _) = LineCol(starts, Math.Max(span.Start, span.End - 1));
        symbols.Add(new
        {
            kind,
            name,
            qual_name = string.IsNullOrEmpty(qn) ? name : qn,
            line = ls,
            end_line = le,
            col = cs
        });
    }

    void AddUnit(string kind, string name, string qn, SyntaxNode node)
    {
        var span = node.Span;
        var (ls, _) = LineCol(starts, span.Start);
        var (le, _) = LineCol(starts, Math.Max(span.Start, span.End - 1));
        units.Add(new
        {
            kind,
            name,
            qual_name = string.IsNullOrEmpty(qn) ? name : qn,
            line_start = ls,
            line_end = le,
            byte_start = ByteAt(src, span.Start),
            byte_end = ByteAt(src, span.End)
        });
        AddSymbol(kind, name, qn, node);
    }

    int firstBody = src.Length;
    foreach (var member in root.Members)
    {
        if (member is BaseTypeDeclarationSyntax or BaseMethodDeclarationSyntax or DelegateDeclarationSyntax)
            firstBody = Math.Min(firstBody, member.SpanStart);
        if (member is FileScopedNamespaceDeclarationSyntax fsn)
        {
            foreach (var m in fsn.Members)
            {
                if (m is BaseTypeDeclarationSyntax or BaseMethodDeclarationSyntax)
                    firstBody = Math.Min(firstBody, m.SpanStart);
            }
        }
        if (member is NamespaceDeclarationSyntax nd)
        {
            foreach (var m in nd.Members)
            {
                if (m is BaseTypeDeclarationSyntax or BaseMethodDeclarationSyntax)
                    firstBody = Math.Min(firstBody, m.SpanStart);
            }
        }
    }
    if (firstBody > 0 && src[..firstBody].Trim().Length > 0)
    {
        var (le, _) = LineCol(starts, firstBody);
        units.Add(new
        {
            kind = "module",
            name = "",
            qual_name = "",
            line_start = 1,
            line_end = Math.Max(1, le),
            byte_start = 0,
            byte_end = ByteAt(src, firstBody)
        });
    }

    void VisitRefs(SyntaxNode node)
    {
        foreach (var child in node.DescendantNodesAndSelf())
        {
            switch (child)
            {
                case UsingDirectiveSyntax u:
                {
                    var name = u.Name?.ToString() ?? "";
                    var shortName = name.Contains('.') ? name[(name.LastIndexOf('.') + 1)..] : name;
                    var (line, col) = LineCol(starts, u.SpanStart);
                    refs.Add(new { kind = "import", name = shortName, qual_name = name, line, col });
                    break;
                }
                case InvocationExpressionSyntax inv:
                {
                    string name = "", qn = "";
                    switch (inv.Expression)
                    {
                        case IdentifierNameSyntax id:
                            name = id.Identifier.Text;
                            qn = name;
                            break;
                        case MemberAccessExpressionSyntax ma:
                            name = ma.Name.Identifier.Text;
                            qn = ma.Expression is IdentifierNameSyntax left
                                ? $"{left.Identifier.Text}.{name}"
                                : name;
                            break;
                    }
                    if (name.Length > 0)
                    {
                        var (line, col) = LineCol(starts, inv.SpanStart);
                        refs.Add(new { kind = "call", name, qual_name = qn, line, col });
                    }
                    break;
                }
            }
        }
    }

    void WalkMembers(IEnumerable<MemberDeclarationSyntax> members, string prefix)
    {
        foreach (var m in members)
        {
            switch (m)
            {
                case ClassDeclarationSyntax c when c.Identifier.Text.Length > 0:
                {
                    var qn = Qual(prefix, c.Identifier.Text);
                    AddUnit("class", c.Identifier.Text, qn, c);
                    WalkTypeMembers(c.Members, qn);
                    VisitRefs(c);
                    break;
                }
                case StructDeclarationSyntax s when s.Identifier.Text.Length > 0:
                {
                    var qn = Qual(prefix, s.Identifier.Text);
                    AddUnit("type", s.Identifier.Text, qn, s);
                    WalkTypeMembers(s.Members, qn);
                    VisitRefs(s);
                    break;
                }
                case InterfaceDeclarationSyntax i when i.Identifier.Text.Length > 0:
                {
                    var qn = Qual(prefix, i.Identifier.Text);
                    AddUnit("type", i.Identifier.Text, qn, i);
                    WalkTypeMembers(i.Members, qn);
                    break;
                }
                case RecordDeclarationSyntax r when r.Identifier.Text.Length > 0:
                {
                    var qn = Qual(prefix, r.Identifier.Text);
                    AddUnit("type", r.Identifier.Text, qn, r);
                    WalkTypeMembers(r.Members, qn);
                    VisitRefs(r);
                    break;
                }
                case EnumDeclarationSyntax e when e.Identifier.Text.Length > 0:
                {
                    var qn = Qual(prefix, e.Identifier.Text);
                    AddUnit("type", e.Identifier.Text, qn, e);
                    break;
                }
                case MethodDeclarationSyntax method when method.Identifier.Text.Length > 0:
                {
                    // Top-level or nested function-like
                    var qn = Qual(prefix, method.Identifier.Text);
                    var kind = method.Modifiers.Any(x => x.IsKind(SyntaxKind.AsyncKeyword))
                        ? "async_function"
                        : "function";
                    AddUnit(kind, method.Identifier.Text, qn, method);
                    VisitRefs(method);
                    break;
                }
                case NamespaceDeclarationSyntax ns:
                    WalkMembers(ns.Members, Qual(prefix, ns.Name.ToString()));
                    VisitRefs(ns);
                    break;
                case FileScopedNamespaceDeclarationSyntax fns:
                    WalkMembers(fns.Members, Qual(prefix, fns.Name.ToString()));
                    VisitRefs(fns);
                    break;
                default:
                    VisitRefs(m);
                    break;
            }
        }
    }

    void WalkTypeMembers(SyntaxList<MemberDeclarationSyntax> members, string typeQn)
    {
        foreach (var mem in members)
        {
            switch (mem)
            {
                case MethodDeclarationSyntax method when method.Identifier.Text.Length > 0:
                    AddSymbol("method", method.Identifier.Text, Qual(typeQn, method.Identifier.Text), method);
                    break;
                case ConstructorDeclarationSyntax ctor:
                    AddSymbol("method", "constructor", Qual(typeQn, "constructor"), ctor);
                    break;
                case PropertyDeclarationSyntax prop when prop.Identifier.Text.Length > 0:
                    AddSymbol("method", prop.Identifier.Text, Qual(typeQn, prop.Identifier.Text), prop);
                    break;
            }
        }
    }

    // Top-level usings
    foreach (var u in root.Usings)
        VisitRefs(u);

    WalkMembers(root.Members, "");

    Console.WriteLine(JsonSerializer.Serialize(new { units, symbols, refs }));
    return 0;
}
catch (Exception ex)
{
    Console.WriteLine(JsonSerializer.Serialize(new { error = ex.Message }));
    return 1;
}

static string Qual(string prefix, string name) =>
    string.IsNullOrEmpty(prefix) ? name : $"{prefix}.{name}";

static List<int> LineStarts(string src)
{
    var starts = new List<int> { 0 };
    for (var i = 0; i < src.Length; i++)
        if (src[i] == '\n') starts.Add(i + 1);
    return starts;
}

static (int line, int col) LineCol(List<int> starts, int offset)
{
    var lo = 0;
    var hi = starts.Count - 1;
    while (lo < hi)
    {
        var mid = (lo + hi + 1) / 2;
        if (starts[mid] <= offset) lo = mid;
        else hi = mid - 1;
    }
    return (lo + 1, offset - starts[lo] + 1);
}

static int ByteAt(string src, int charIndex)
{
    if (charIndex <= 0) return 0;
    if (charIndex >= src.Length) return Encoding.UTF8.GetByteCount(src);
    return Encoding.UTF8.GetByteCount(src.AsSpan(0, charIndex));
}
