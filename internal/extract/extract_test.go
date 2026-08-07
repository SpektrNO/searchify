package extract_test

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spektr/searchify/internal/extract"
	"github.com/xuri/excelize/v2"
)

func TestRegistryExtensionsIncludeP0P1(t *testing.T) {
	reg := extract.NewRegistry(extract.Options{})
	need := []string{
		".md", ".pdf", ".docx", ".xlsx", ".csv", ".png", ".html",
		".xml", ".toml", ".pptx", ".odt", ".rtf", ".eml",
	}
	have := map[string]bool{}
	for _, e := range reg.Extensions() {
		have[e] = true
	}
	for _, e := range need {
		if !have[e] {
			t.Fatalf("missing extension %s in registry", e)
		}
	}
	if reg.HasExtension("nope.bin") {
		t.Fatal(".bin should not be indexed")
	}
}

func TestPassthroughAndHTML(t *testing.T) {
	reg := extract.NewRegistry(extract.Options{})
	ctx := context.Background()

	text, _, err := reg.Extract(ctx, "a.md", strings.NewReader("# hello\n"), 8)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "hello") {
		t.Fatalf("got %q", text)
	}

	html := `<html><head><style>x{}</style><script>1</script></head><body><p>VisibleToken</p></body></html>`
	text, _, err = reg.Extract(ctx, "a.html", strings.NewReader(html), int64(len(html)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "VisibleToken") {
		t.Fatalf("missing visible text: %q", text)
	}
	if strings.Contains(text, "script") || strings.Contains(text, "x{}") {
		t.Fatalf("leaked script/style: %q", text)
	}
}

func TestDOCXAndXLSXAndCSV(t *testing.T) {
	reg := extract.NewRegistry(extract.Options{})
	ctx := context.Background()

	docx := minimalDOCX("UniqueDocxPhrase")
	text, _, err := reg.Extract(ctx, "a.docx", bytes.NewReader(docx), int64(len(docx)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "UniqueDocxPhrase") {
		t.Fatalf("docx text=%q", text)
	}

	xlsxBytes := minimalXLSX(t, "UniqueXlsxPhrase")
	text, _, err = reg.Extract(ctx, "a.xlsx", bytes.NewReader(xlsxBytes), int64(len(xlsxBytes)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "UniqueXlsxPhrase") {
		t.Fatalf("xlsx text=%q", text)
	}

	csv := "a,b\nUniqueCsvPhrase,2\n"
	text, _, err = reg.Extract(ctx, "a.csv", strings.NewReader(csv), int64(len(csv)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "UniqueCsvPhrase") {
		t.Fatalf("csv text=%q", text)
	}
}

func TestPDFPlainText(t *testing.T) {
	reg := extract.NewRegistry(extract.Options{})
	pdf := minimalPDF("UniquePdfPhrase")
	text, _, err := reg.Extract(context.Background(), "a.pdf", bytes.NewReader(pdf), int64(len(pdf)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "UniquePdfPhrase") {
		t.Fatalf("pdf text=%q", text)
	}
}

func TestImageOCROffIsSkip(t *testing.T) {
	reg := extract.NewRegistry(extract.Options{OCREnabled: false})
	_, _, err := reg.Extract(context.Background(), "a.png", bytes.NewReader([]byte("notanimage")), 10)
	var skip *extract.SkipError
	if err == nil {
		t.Fatal("expected skip")
	}
	if !asSkip(err, &skip) {
		t.Fatalf("want SkipError, got %T %v", err, err)
	}
}

func TestExtractTimeout(t *testing.T) {
	reg := extract.NewRegistry(extract.Options{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)
	_, _, err := reg.Extract(ctx, "a.md", strings.NewReader("hi"), 2)
	if err == nil {
		t.Fatal("expected context error")
	}
}

func TestCorruptPDFFailsSoft(t *testing.T) {
	reg := extract.NewRegistry(extract.Options{})
	_, _, err := reg.Extract(context.Background(), "bad.pdf", strings.NewReader("not a pdf"), 9)
	if err == nil {
		t.Fatal("expected error")
	}
}

func asSkip(err error, target **extract.SkipError) bool {
	if err == nil {
		return false
	}
	if s, ok := err.(*extract.SkipError); ok {
		*target = s
		return true
	}
	return false
}

func minimalDOCX(phrase string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>` + phrase + `</w:t></w:r></w:p></w:body>
</w:document>`,
	}
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			panic(err)
		}
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func minimalXLSX(t *testing.T, phrase string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetCellValue("Sheet1", "A1", phrase); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func minimalPDF(phrase string) []byte {
	// Minimal PDF with a single Helvetica text show operator.
	content := "BT /F1 24 Tf 100 700 Td (" + phrase + ") Tj ET"
	objects := []string{
		"1 0 obj<< /Type /Catalog /Pages 2 0 R >>endobj\n",
		"2 0 obj<< /Type /Pages /Kids [3 0 R] /Count 1 >>endobj\n",
		"3 0 obj<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources<< /Font<< /F1 5 0 R >> >> >>endobj\n",
		"4 0 obj<< /Length " + itoa(len(content)) + " >>stream\n" + content + "\nendstream\nendobj\n",
		"5 0 obj<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>endobj\n",
	}
	var body strings.Builder
	body.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for i, obj := range objects {
		offsets[i+1] = body.Len()
		body.WriteString(obj)
	}
	xrefStart := body.Len()
	body.WriteString("xref\n0 ")
	body.WriteString(itoa(len(objects) + 1))
	body.WriteString("\n")
	body.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objects); i++ {
		body.WriteString(pad10(offsets[i]) + " 00000 n \n")
	}
	body.WriteString("trailer<< /Size " + itoa(len(objects)+1) + " /Root 1 0 R >>\n")
	body.WriteString("startxref\n")
	body.WriteString(itoa(xrefStart))
	body.WriteString("\n%%EOF\n")
	return []byte(body.String())
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [32]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func pad10(n int) string {
	s := itoa(n)
	for len(s) < 10 {
		s = "0" + s
	}
	return s
}

func TestWriteFixturesHelper(t *testing.T) {
	// Smoke: ensure fixtures can be written for integration tests.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.pdf"), minimalPDF("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}
