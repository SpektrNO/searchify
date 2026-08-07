package local

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spektr/searchify/internal/config"
	"github.com/spektr/searchify/internal/search"
	"github.com/xuri/excelize/v2"
)

func TestIndexRicherFileTypes(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "corpus")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}

	mustWrite := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(docs, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("note.md", []byte("passthrough keep\n"))
	mustWrite("sheet.csv", []byte("col\nUniqueCsvHitXYZ\n"))
	mustWrite("page.html", []byte(`<html><body><p>UniqueHtmlHitXYZ</p></body></html>`))
	mustWrite("doc.docx", richerMinimalDOCX("UniqueDocxHitXYZ"))
	mustWrite("book.pdf", richerMinimalPDF("UniquePdfHitXYZ"))
	mustWrite("grid.xlsx", richerMinimalXLSX(t, "UniqueXlsxHitXYZ"))
	mustWrite("photo.png", []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}) // PNG header only
	mustWrite("broken.pdf", []byte("%PDF-not-really"))
	mustWrite("ignore.bin", []byte("should be ignored"))

	cfg := &config.Config{
		Roots:          []string{root},
		IndexDir:       filepath.Join(t.TempDir(), "index"),
		EmbedModel:     "stub",
		MaxFileBytes:   32 * 1024 * 1024,
		ExtractTimeout: 30 * time.Second,
		OCREnabled:     false,
	}
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	svc.embedForTest = &stubEmbedder{}

	report, err := svc.IndexPaths([]string{docs}, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Indexed < 5 {
		t.Fatalf("expected >=5 indexed, got indexed=%d errors=%d msgs=%v", report.Indexed, report.Errors, report.Messages)
	}
	if report.Errors < 1 {
		t.Fatalf("expected corrupt pdf to count as error, msgs=%v", report.Messages)
	}

	pngSkipped := false
	for _, m := range report.Messages {
		if strings.Contains(m, "photo.png") && strings.Contains(strings.ToLower(m), "ocr") {
			pngSkipped = true
		}
	}
	if !pngSkipped {
		t.Fatalf("expected OCR-off skip message for png, msgs=%v", report.Messages)
	}

	for _, q := range []string{"UniquePdfHitXYZ", "UniqueDocxHitXYZ", "UniqueXlsxHitXYZ", "UniqueCsvHitXYZ", "UniqueHtmlHitXYZ"} {
		res, err := svc.Search(SearchParams{Query: q, Mode: search.ModeKeyword, Limit: 5})
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Results) == 0 {
			t.Fatalf("expected keyword hit for %q", q)
		}
	}

	status, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.OCREnabled {
		t.Fatal("ocr should be off")
	}
	if len(status.IndexExtensions) < 10 {
		t.Fatalf("expected index_extensions, got %v", status.IndexExtensions)
	}
}

func TestMaxFileBytesSkip(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "big.md")
	if err := os.WriteFile(path, bytes.Repeat([]byte("a"), 100), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Roots:          []string{root},
		IndexDir:       filepath.Join(t.TempDir(), "index"),
		EmbedModel:     "stub",
		MaxFileBytes:   50,
		ExtractTimeout: time.Second,
	}
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	svc.embedForTest = &stubEmbedder{}

	report, err := svc.IndexPaths([]string{root}, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Indexed != 0 {
		t.Fatalf("expected skip, indexed=%d", report.Indexed)
	}
	found := false
	for _, m := range report.Messages {
		if strings.Contains(m, "larger than") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected size skip message, got %v", report.Messages)
	}
}

func TestTextOnlySkipsPDF(t *testing.T) {
	root := t.TempDir()
	md := filepath.Join(root, "a.md")
	if err := os.WriteFile(md, []byte("plain text only please\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pdf := filepath.Join(root, "b.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.1 fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Roots:            []string{root},
		IndexDir:         filepath.Join(t.TempDir(), "index"),
		EmbedModel:       "stub",
		SkipEmbed:        true,
		TextOnly:         true,
		MaxFileBytes:     1024 * 1024,
		MaxExtractBytes:  1024 * 1024,
		MaxChunksPerFile: 64,
	}
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	report, err := svc.IndexPaths([]string{root}, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Indexed != 1 {
		t.Fatalf("indexed=%d want 1 (md only); msgs=%v", report.Indexed, report.Messages)
	}
}

func richerMinimalDOCX(phrase string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	_ = writeZipFile(zw, "[Content_Types].xml", `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`)
	_ = writeZipFile(zw, "word/document.xml", `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>`+phrase+`</w:t></w:r></w:p></w:body></w:document>`)
	_ = zw.Close()
	return buf.Bytes()
}

func writeZipFile(zw *zip.Writer, name, body string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(body))
	return err
}

func richerMinimalXLSX(t *testing.T, phrase string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetCellValue("Sheet1", "A1", phrase)
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func richerMinimalPDF(phrase string) []byte {
	content := "BT /F1 24 Tf 100 700 Td (" + phrase + ") Tj ET"
	var body strings.Builder
	objs := []string{
		"1 0 obj<< /Type /Catalog /Pages 2 0 R >>endobj\n",
		"2 0 obj<< /Type /Pages /Kids [3 0 R] /Count 1 >>endobj\n",
		"3 0 obj<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources<< /Font<< /F1 5 0 R >> >> >>endobj\n",
		"4 0 obj<< /Length " + itoaLocal(len(content)) + " >>stream\n" + content + "\nendstream\nendobj\n",
		"5 0 obj<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>endobj\n",
	}
	body.WriteString("%PDF-1.4\n")
	off := make([]int, len(objs)+1)
	for i, o := range objs {
		off[i+1] = body.Len()
		body.WriteString(o)
	}
	xref := body.Len()
	body.WriteString("xref\n0 " + itoaLocal(len(objs)+1) + "\n0000000000 65535 f \n")
	for i := 1; i <= len(objs); i++ {
		s := itoaLocal(off[i])
		for len(s) < 10 {
			s = "0" + s
		}
		body.WriteString(s + " 00000 n \n")
	}
	body.WriteString("trailer<< /Size " + itoaLocal(len(objs)+1) + " /Root 1 0 R >>\nstartxref\n" + itoaLocal(xref) + "\n%%EOF\n")
	return []byte(body.String())
}

func itoaLocal(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
