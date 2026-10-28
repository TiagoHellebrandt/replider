package services

import (
	"os"
	"strings"
	"testing"
)

// The brochure fixture is only useful if the real reader can read it: page
// count feeds the import's ceiling, text presence proves it is a document and
// not just bytes with a %PDF header.
func TestPDFPageCount_ReadsTheBrochureFixture(t *testing.T) {
	data, err := os.ReadFile("../usecases/testdata/folheto_produtos.pdf")
	if err != nil {
		t.Fatalf("fixture missing: %v", err)
	}
	pages, err := PDFPageCount(data)
	if err != nil || pages != 2 {
		t.Fatalf("pages = %d, err = %v; want 2 pages", pages, err)
	}
	text, err := ExtractText(data, "application/pdf")
	if err != nil || !strings.Contains(text, "Tomografo") {
		t.Fatalf("fixture text unreadable: err=%v text=%q", err, text)
	}
}
