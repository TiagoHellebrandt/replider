package services

// DefaultPDFExtractor is the production implementation of the
// usecases.PDFExtractor port. It's a thin wrapper around ExtractText
// so the use case stays decoupled from the ledongthuc/pdf import and
// tests can substitute a stub that returns canned strings.
type DefaultPDFExtractor struct{}

func NewDefaultPDFExtractor() *DefaultPDFExtractor { return &DefaultPDFExtractor{} }

// Extract routes through ExtractText with the PDF MIME hardcoded.
// Scanned / image-only PDFs surface inside extractPDF as "no
// extractable text" — we turn that into an empty-string success so
// the use case's unified scanned-PDF guard (<100 chars) can own the
// error mapping in one place instead of two.
func (e *DefaultPDFExtractor) Extract(data []byte) (string, error) {
	text, err := ExtractText(data, "application/pdf")
	if err != nil {
		// "no extractable text" is the extractPDF sentinel for
		// image-only PDFs. Everything else is a genuine parse
		// failure (bad header, truncated, encrypted) and should
		// propagate.
		if err.Error() == "pdf: no extractable text (image-only or encrypted?)" {
			return "", nil
		}
		return "", err
	}
	return text, nil
}
