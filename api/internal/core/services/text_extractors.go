package services

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

// ErrUnsupportedFileType is returned by ExtractText when the caller
// passes a mime type we haven't implemented extraction for. The HTTP
// layer translates this into `INVALID_FILE_TYPE`.
var ErrUnsupportedFileType = errors.New("unsupported file type")

// ExtractText turns raw bytes into plain text for indexing. The mime
// argument is the detected content type — we match on prefix so
// `text/plain; charset=utf-8` also works. Returns the extracted text
// trimmed; empty string is treated by the caller as "no content".
//
// Supported types:
//   - text/plain (UTF-8, falls back to latin-1 for mixed encodings)
//   - application/pdf
//   - application/vnd.openxmlformats-officedocument.wordprocessingml.document (.docx)
func ExtractText(data []byte, mime string) (string, error) {
	switch {
	case strings.HasPrefix(mime, "text/plain"):
		return extractPlainText(data), nil
	case strings.HasPrefix(mime, "application/pdf"):
		return extractPDF(data)
	case strings.HasPrefix(mime, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"):
		return extractDOCX(data)
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedFileType, mime)
	}
}

// DetectMimeByMagic sniffs the leading bytes to pick the right
// extractor regardless of the client-supplied Content-Type — which is
// trivially spoofable. Returns empty string when we can't tell; caller
// should reject those uploads.
//
// The patterns cover just the three types we index; a richer sniff
// (like net/http.DetectContentType) would return image/* etc. that we
// can't handle anyway.
func DetectMimeByMagic(data []byte) string {
	if len(data) >= 4 && data[0] == '%' && data[1] == 'P' && data[2] == 'D' && data[3] == 'F' {
		return "application/pdf"
	}
	// DOCX (and every Office Open XML) is a ZIP container. We tighten
	// the check below by looking for the expected "word/document.xml"
	// entry inside extractDOCX; here we just return the generic type.
	if len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && data[2] == 3 && data[3] == 4 {
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
	if utf8.Valid(data) {
		return "text/plain; charset=utf-8"
	}
	return ""
}

func extractPlainText(data []byte) string {
	if utf8.Valid(data) {
		return strings.TrimSpace(string(data))
	}
	// Fallback: treat each byte as latin-1 (ISO-8859-1). Good enough
	// for Portuguese-language plain text saved by old editors on
	// Windows. Any bytes above 0xFF aren't representable anyway.
	var b strings.Builder
	b.Grow(len(data))
	for _, c := range data {
		b.WriteRune(rune(c))
	}
	return strings.TrimSpace(b.String())
}

// PDFPageCount reports how many pages a PDF has, without extracting anything.
// The product-brochure import uses it to enforce its page ceiling BEFORE the
// bytes reach a model — refusing after the model call would spend the tokens
// the ceiling exists to protect.
func PDFPageCount(data []byte) (int, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0, fmt.Errorf("pdf: failed to open: %w", err)
	}
	return reader.NumPage(), nil
}

func extractPDF(data []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("pdf: failed to open: %w", err)
	}
	var buf bytes.Buffer
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			// Some PDFs have malformed pages — skip and keep going
			// rather than fail the whole upload.
			continue
		}
		buf.WriteString(text)
		buf.WriteByte('\n')
	}
	out := strings.TrimSpace(buf.String())
	if out == "" {
		return "", errors.New("pdf: no extractable text (image-only or encrypted?)")
	}
	return out, nil
}

// extractDOCX pulls text from `word/document.xml` inside a .docx
// archive. We intentionally skip headers, footers, and tracked changes
// — they add noise and the body is what customers actually want to
// index. A DOCX is a ZIP by spec, so we don't need a full OOXML library.
func extractDOCX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("docx: not a valid zip: %w", err)
	}
	var doc *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			doc = f
			break
		}
	}
	if doc == nil {
		return "", errors.New("docx: missing word/document.xml")
	}
	rc, err := doc.Open()
	if err != nil {
		return "", fmt.Errorf("docx: failed to open document.xml: %w", err)
	}
	defer rc.Close()

	raw, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("docx: read failed: %w", err)
	}

	var buf bytes.Buffer
	dec := xml.NewDecoder(bytes.NewReader(raw))
	// DOCX wraps text in <w:t>...</w:t> elements. Paragraphs are <w:p>,
	// line breaks are <w:br/>. We flush a newline on each </w:p> so the
	// chunker can later break on paragraph boundaries the same way it
	// does for plain text input.
	inText := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Malformed XML deep inside the body is survivable — stop
			// here with whatever we have so far rather than 0'ing out.
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				inText = true
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inText = false
			}
			if t.Name.Local == "p" {
				buf.WriteByte('\n')
			}
		case xml.CharData:
			if inText {
				buf.Write(t)
			}
		}
	}
	out := strings.TrimSpace(buf.String())
	if out == "" {
		return "", errors.New("docx: empty body")
	}
	return out, nil
}
