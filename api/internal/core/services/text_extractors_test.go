package services

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractText_PlainTextUTF8(t *testing.T) {
	out, err := ExtractText([]byte("olá, mundo\ncom acentos"), "text/plain; charset=utf-8")
	assert.NoError(t, err)
	assert.Equal(t, "olá, mundo\ncom acentos", out)
}

func TestExtractText_PlainTextLatin1Fallback(t *testing.T) {
	// "canção" in latin-1 is 0x63 0x61 0x6E 0xE7 0xE3 0x6F — not valid
	// UTF-8. The fallback path should still produce a printable string.
	data := []byte{0x63, 0x61, 0x6E, 0xE7, 0xE3, 0x6F}
	out, err := ExtractText(data, "text/plain")
	assert.NoError(t, err)
	assert.NotEmpty(t, out)
}

func TestExtractText_UnsupportedType(t *testing.T) {
	_, err := ExtractText([]byte("foo"), "image/jpeg")
	assert.ErrorIs(t, err, ErrUnsupportedFileType)
}

func TestDetectMimeByMagic(t *testing.T) {
	// PDF header.
	assert.Equal(t, "application/pdf",
		DetectMimeByMagic([]byte("%PDF-1.4\n...")))

	// ZIP header (DOCX / any Office Open XML).
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		DetectMimeByMagic([]byte{'P', 'K', 3, 4, 'a', 'b'}))

	// Valid UTF-8 → plain text.
	assert.Equal(t, "text/plain; charset=utf-8",
		DetectMimeByMagic([]byte("hello world")))

	// Random binary bytes → empty (unknown).
	assert.Equal(t, "",
		DetectMimeByMagic([]byte{0xFF, 0xD8, 0xFF, 0xE0}))
}

// Builds the minimum .docx the extractor needs to care about — a ZIP
// with a single word/document.xml entry containing two runs across two
// paragraphs.
func buildMiniDOCX(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("create xml: %v", err)
	}
	xmlBody := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Clínica odontológica.</w:t></w:r></w:p>
    <w:p><w:r><w:t>Horário: segunda a sexta.</w:t></w:r></w:p>
  </w:body>
</w:document>`
	if _, err := f.Write([]byte(xmlBody)); err != nil {
		t.Fatalf("write xml: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func TestExtractText_DOCX(t *testing.T) {
	data := buildMiniDOCX(t)
	out, err := ExtractText(data,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	assert.NoError(t, err)
	assert.Contains(t, out, "Clínica odontológica")
	assert.Contains(t, out, "Horário: segunda a sexta")
	// Paragraph break preserved so the chunker can split cleanly.
	assert.True(t, strings.Contains(out, "\n"))
}

func TestExtractText_DOCXInvalidZip(t *testing.T) {
	_, err := ExtractText([]byte("not a zip"),
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	assert.Error(t, err)
}

func TestExtractText_PDFInvalidData(t *testing.T) {
	_, err := ExtractText([]byte("not a pdf"), "application/pdf")
	assert.Error(t, err)
}
