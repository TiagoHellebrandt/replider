package domain

import "testing"

func TestCSVSafeCell_PrefixesFormulaTriggers(t *testing.T) {
	cases := map[string]string{
		"=HYPERLINK(\"http://evil\")": "'=HYPERLINK(\"http://evil\")",
		"+1+1":                        "'+1+1",
		"-2":                          "'-2",
		"@SUM(A1)":                    "'@SUM(A1)",
		"\t=cmd":                      "'\t=cmd",
		"\r=cmd":                      "'\r=cmd",
		"":                            "",
		"Clínica Aurora":              "Clínica Aurora",
		"contato@clinica.com.br":      "contato@clinica.com.br", // '@' only matters at the start
		"5565999990000":               "5565999990000",
	}
	for in, want := range cases {
		if got := CSVSafeCell(in); got != want {
			t.Errorf("CSVSafeCell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCSVSafeRow_DoesNotMutateInput(t *testing.T) {
	in := []string{"=1", "ok"}
	out := CSVSafeRow(in)
	if in[0] != "=1" {
		t.Fatalf("input mutated: %q", in[0])
	}
	if out[0] != "'=1" || out[1] != "ok" {
		t.Fatalf("unexpected row: %v", out)
	}
}
