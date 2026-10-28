package domain

import "testing"

// The money parser is the reason this file exists. "1.234,56" and "1,234.56" are
// the same amount and "1,234" is two different amounts a THOUSAND apart; this
// codebase has already shipped that bug in both directions.

func TestParseMoney_BothNotations(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		// Brazilian: dot groups, comma decides.
		{"1.234,56", 123456},
		{"1.234.567,89", 123456789},
		{"49,90", 4990},
		{"0,05", 5},
		// US: comma groups, dot decides.
		{"1,234.56", 123456},
		{"1,234,567.89", 123456789},
		{"49.90", 4990},
		// No separator at all: whole units.
		{"1234", 123400},
		{"0", 0},
		// One decimal digit is TENTHS. "10,5" is ten and fifty cents, and reading
		// it as ten-oh-five would shave 45 cents off every such row.
		{"10,5", 1050},
		{"10.5", 1050},
		// Symbols and the spaces Excel emits are noise around the number.
		{"R$ 1.234,56", 123456},
		{" R$1.234,56 ", 123456},
		{"US$ 1,234.56", 123456},
		{"1.234,56 BRL", 123456},
		// A negative belongs to the amount, not to a currency called minus.
		{"-49,90", -4990},
	}

	for _, c := range cases {
		got, ok, ambiguous := ParseMoney(c.in)
		if !ok || ambiguous {
			t.Errorf("ParseMoney(%q) = (%d, ok=%v, ambiguous=%v), want a clean parse",
				c.in, got, ok, ambiguous)
			continue
		}
		if got != c.want {
			t.Errorf("ParseMoney(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// The refusal that pays for the whole function. "1,234" is 1234 or 1.234 and no
// rule recovers which — one look at one row beats a price list wrong by 1000x.
func TestParseMoney_RefusesTheAmbiguousShape(t *testing.T) {
	for _, in := range []string{"1,234", "1.234", "12,345", "999.999"} {
		cents, ok, ambiguous := ParseMoney(in)
		if ok || !ambiguous {
			t.Errorf("ParseMoney(%q) = (%d, ok=%v, ambiguous=%v), want ambiguous",
				in, cents, ok, ambiguous)
		}
	}
}

func TestParseMoney_RejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "  ", "abc", "1,2345", "R$", "1..2,3,4"} {
		cents, ok, ambiguous := ParseMoney(in)
		if ok {
			t.Errorf("ParseMoney(%q) = %d, want a refusal", in, cents)
		}
		if ambiguous {
			t.Errorf("ParseMoney(%q) reported ambiguity; it is simply invalid", in)
		}
	}
}

func TestBuildImportRows_ReadsAPortugueseSpreadsheet(t *testing.T) {
	records := [][]string{
		{"Nome", "Descrição", "Preço", "Moeda"},
		{"Gaze estéril 10x10", "Pacote com 500", "R$ 49,90", "BRL"},
		{"Luva cirúrgica", "", "125,50", ""},
	}

	rows, err := BuildImportRows(records, "BRL")
	if err != nil {
		t.Fatalf("BuildImportRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].Name != "Gaze estéril 10x10" || rows[0].PriceCents != 4990 {
		t.Fatalf("row 0 wrong: %+v", rows[0])
	}
	if rows[0].Description != "Pacote com 500" {
		t.Fatalf("description not read: %+v", rows[0])
	}
	// The line number points at the row the person sees in Excel, header included.
	if rows[0].Line != 2 || rows[1].Line != 3 {
		t.Fatalf("line numbers wrong: %d, %d", rows[0].Line, rows[1].Line)
	}
	// An empty currency cell falls back to the ACCOUNT's, never to a literal.
	if rows[1].Currency != "BRL" {
		t.Fatalf("currency fallback wrong: %q", rows[1].Currency)
	}
}

// A USD account's spreadsheet with no currency column must not come back BRL.
func TestBuildImportRows_FallbackIsTheAccountsCurrency(t *testing.T) {
	rows, err := BuildImportRows([][]string{
		{"name", "price"},
		{"Swab", "12.50"},
	}, "USD")
	if err != nil {
		t.Fatalf("BuildImportRows: %v", err)
	}
	if rows[0].Currency != "USD" || rows[0].PriceCents != 1250 {
		t.Fatalf("row wrong: %+v", rows[0])
	}
}

// A problem row still travels. A silently dropped line is a product the person
// believes they imported.
func TestBuildImportRows_KeepsProblemRowsWithTheirLine(t *testing.T) {
	rows, err := BuildImportRows([][]string{
		{"Nome", "Preço"},
		{"Gaze", "49,90"},
		{"Ambígua", "1,234"},
		{"", "10,00"},
		{"Gaze", "49,90"},
		{"Sem preço", "abc"},
	}, "BRL")
	if err != nil {
		t.Fatalf("BuildImportRows: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("len(rows) = %d, want 5 — a problem row must not disappear", len(rows))
	}

	if !rows[0].Importable() {
		t.Fatalf("the good row is not importable: %+v", rows[0])
	}
	if rows[1].Problem != ProblemAmbiguousPrice || rows[1].Line != 3 {
		t.Fatalf("ambiguous row wrong: %+v", rows[1])
	}
	if rows[2].Problem != ProblemNoName {
		t.Fatalf("nameless row wrong: %+v", rows[2])
	}
	// Two lines with one name would create two products nobody can tell apart.
	if rows[3].Problem != ProblemDuplicateInFile {
		t.Fatalf("duplicate row wrong: %+v", rows[3])
	}
	if rows[4].Problem != ProblemBadPrice {
		t.Fatalf("bad-price row wrong: %+v", rows[4])
	}
	for i, r := range rows[1:] {
		if r.Importable() {
			t.Fatalf("row %d has a problem and is still importable: %+v", i+1, r)
		}
	}
}

// A trailing blank line is what every spreadsheet export looks like. Reporting it
// as a problem trains the person to ignore problems.
func TestBuildImportRows_SkipsBlankTrailingLines(t *testing.T) {
	rows, err := BuildImportRows([][]string{
		{"Nome", "Preço"},
		{"Gaze", "49,90"},
		{"", ""},
		{"  ", ""},
	}, "BRL")
	if err != nil {
		t.Fatalf("BuildImportRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
}

// The wrong file is not an empty import, and both need saying differently.
func TestBuildImportRows_RefusalsAreDistinct(t *testing.T) {
	if _, err := BuildImportRows(nil, "BRL"); err != ErrProductImportEmpty {
		t.Fatalf("nil: err = %v", err)
	}
	if _, err := BuildImportRows([][]string{{"Nome", "Preço"}}, "BRL"); err != ErrProductImportEmpty {
		t.Fatalf("header only: err = %v", err)
	}
	_, err := BuildImportRows([][]string{
		{"coluna a", "coluna b"},
		{"x", "y"},
	}, "BRL")
	if err != ErrProductImportNoHeader {
		t.Fatalf("unrecognisable header: err = %v, want ErrProductImportNoHeader", err)
	}
}

// A price column is optional: a catalogue whose prices are negotiated per client
// is still worth importing as a list of what she sells.
func TestBuildImportRows_PriceColumnIsOptional(t *testing.T) {
	rows, err := BuildImportRows([][]string{
		{"Produto"},
		{"Gaze estéril"},
	}, "BRL")
	if err != nil {
		t.Fatalf("BuildImportRows: %v", err)
	}
	if !rows[0].Importable() || rows[0].PriceCents != 0 {
		t.Fatalf("row wrong: %+v", rows[0])
	}
}

// Header matching ignores case and accents: a real export writes "Preço", "PRECO"
// and "preco" depending on who saved it.
func TestMatchHeaderIgnoresCaseAndAccents(t *testing.T) {
	for _, header := range [][]string{
		{"PREÇO"}, {"preco"}, {" Preço "}, {"Valor"},
	} {
		if got := MatchHeader(header, priceHeaders); got != 0 {
			t.Errorf("MatchHeader(%v) = %d, want 0", header, got)
		}
	}
	if got := MatchHeader([]string{"outra"}, priceHeaders); got != -1 {
		t.Errorf("MatchHeader(outra) = %d, want -1", got)
	}
}
