package usecases

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Universal export (I3).
//
// The file goes to a spreadsheet on somebody's laptop, so the tests are about
// what happens when it is opened: the accents survive, the phone numbers keep
// their digits, and a contact name typed by a stranger does not execute.

func exportTable() domain.ExportTable {
	return domain.ExportTable{
		Columns: []string{"id", "contato", "telefone"},
		Rows: [][]string{
			{"l1", "Clínica Bem Viver", "5565999990000"},
			{"l2", "Hospital São José", "5565988880000"},
		},
	}
}

func TestRenderCSV_OpensCorrectlyInBrazilianExcel(t *testing.T) {
	out, err := renderCSV(exportTable())
	if err != nil {
		t.Fatalf("renderCSV: %v", err)
	}

	// The BOM is what stops "Clínica" arriving as "ClÃ­nica" for every user on a
	// default Windows Excel — which is most of the people this file is for.
	if !bytes.HasPrefix(out, utf8BOM) {
		t.Fatal("missing UTF-8 BOM")
	}
	text := string(out)
	if !strings.Contains(text, "id;contato;telefone") {
		t.Fatalf("expected semicolon separators, got:\n%s", text)
	}
	// A comma separator would put an entire row into one cell in a pt-BR locale.
	if strings.Contains(text, "id,contato") {
		t.Fatal("comma-separated: unreadable in the locale this file is made for")
	}
	if !strings.Contains(text, "Clínica Bem Viver") {
		t.Fatal("the accents did not survive")
	}
}

func TestRenderXLSX_ProducesARealSpreadsheet(t *testing.T) {
	out, err := renderXLSX(exportTable())
	if err != nil {
		t.Fatalf("renderXLSX: %v", err)
	}
	// XLSX is a zip; "PK" is its magic number. Enough to prove a real file came
	// out rather than a CSV with a different extension.
	if len(out) < 4 || out[0] != 'P' || out[1] != 'K' {
		t.Fatalf("not a zip container: % x", out[:4])
	}
}

func TestSanitizeExportCell_DefusesFormulaInjection(t *testing.T) {
	// Contact names in this file are typed by strangers on WhatsApp. Without
	// this, "=HYPERLINK(...)" in a contact name is a path from an inbound
	// message to code running on a salesperson's laptop.
	for _, dangerous := range []string{
		"=HYPERLINK(\"http://evil\",\"clique\")",
		"+1+1",
		"-2+3",
		"@SUM(A1:A9)",
	} {
		got := sanitizeExportCell(dangerous)
		if !strings.HasPrefix(got, "'") {
			t.Fatalf("%q was not defused: %q", dangerous, got)
		}
		// And the value is still readable — defusing must not destroy the data.
		if !strings.Contains(got, strings.TrimLeft(dangerous, "=+-@")[:3]) {
			t.Fatalf("%q lost its content: %q", dangerous, got)
		}
	}

	// Ordinary text is untouched: a leading apostrophe on every cell would be
	// visible noise in the file people actually read.
	for _, safe := range []string{"Clínica Bem Viver", "5565999990000", "", "R$ 100,00"} {
		if got := sanitizeExportCell(safe); got != safe {
			t.Fatalf("%q was altered to %q", safe, got)
		}
	}
}

func TestExportRequest_NormalizeRefusesAndClamps(t *testing.T) {
	bad := domain.ExportRequest{Entity: "everything"}
	if err := bad.Normalize(); err == nil {
		t.Fatal("an unknown entity must be refused, not resolved into 'everything'")
	}

	badFormat := domain.ExportRequest{Entity: domain.ExportLeads, Format: "pdf"}
	if err := badFormat.Normalize(); err == nil {
		t.Fatal("an unknown format must be refused")
	}

	defaults := domain.ExportRequest{Entity: domain.ExportLeads}
	if err := defaults.Normalize(); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if defaults.Format != domain.ExportCSV {
		t.Fatalf("format = %q, want the CSV default", defaults.Format)
	}
	if defaults.Limit != domain.DefaultExportLimit {
		t.Fatalf("limit = %d, want the default", defaults.Limit)
	}

	huge := domain.ExportRequest{Entity: domain.ExportLeads, Limit: 999999}
	_ = huge.Normalize()
	if huge.Limit != domain.MaxExportLimit {
		t.Fatalf("limit = %d, want the ceiling", huge.Limit)
	}
}

func TestExportTable_TruncationIsVisible(t *testing.T) {
	table := domain.ExportTable{Rows: make([][]string, 50)}

	// Somebody who asked for "all leads" and got exactly the ceiling has to
	// know the file is not all of them, or they will reconcile it against a
	// dashboard and conclude the dashboard is wrong.
	if !table.Truncated(50) {
		t.Fatal("a full page must report as truncated")
	}
	if table.Truncated(51) {
		t.Fatal("a table under the limit is complete")
	}
}

func TestFilterSummary_IsDeterministicAndDropsEmpties(t *testing.T) {
	first := domain.FilterSummary(map[string]string{
		"status": "responded", "funnel_id": "f1", "source": "",
	})
	second := domain.FilterSummary(map[string]string{
		"source": "", "funnel_id": "f1", "status": "responded",
	})

	// An audit line whose text changes between two identical exports is an
	// audit line nobody trusts — and Go's map order is not stable.
	if first != second {
		t.Fatalf("summary drifted: %q vs %q", first, second)
	}
	if strings.Contains(first, "source") {
		t.Fatalf("an empty filter was recorded as applied: %q", first)
	}
	if first != "funnel_id=f1, status=responded" {
		t.Fatalf("summary = %q", first)
	}
}

func TestExportRequest_FilenameCarriesTheEntityAndTheDate(t *testing.T) {
	req := domain.ExportRequest{Entity: domain.ExportSales, Format: domain.ExportXLSX}
	_ = req.Normalize()

	name := req.Filename(fixedExportTime())
	// A downloads folder with four files called "export.csv" is a downloads
	// folder nobody can use.
	if name != "sales_2026-07-30.xlsx" {
		t.Fatalf("filename = %q", name)
	}
}

func fixedExportTime() time.Time {
	return time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
}
