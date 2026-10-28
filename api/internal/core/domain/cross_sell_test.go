package domain

import "testing"

func pair(product, partner string, both, total int) CoPurchasePair {
	return CoPurchasePair{
		Product: product, Partner: partner,
		Customers: both, ProductCustomers: total,
	}
}

func TestCoPurchasePair_ConfidenceIsUnknownWithNoDenominator(t *testing.T) {
	// Zero out of zero is unknown, not zero.
	if c := pair("luva", "mascara", 0, 0).Confidence(); c != nil {
		t.Fatalf("expected nil confidence, got %v", *c)
	}
}

// Two customers is a coincidence somebody will act on, and acting on a
// coincidence in front of a customer costs more credibility than the
// suggestion was worth.
func TestCoPurchasePair_TwoCustomersIsNotEnough(t *testing.T) {
	if pair("luva", "mascara", 2, 2).Strong() {
		t.Fatal("two customers must not be a pattern")
	}
	if !pair("luva", "mascara", 3, 4).Strong() {
		t.Fatal("three of four customers is a pattern")
	}
}

// Everybody who buys anything also buys gloves. Without a confidence floor
// the suggestion for every customer is "gloves".
func TestCoPurchasePair_LowConfidenceIsNoise(t *testing.T) {
	if pair("luva", "mascara", 5, 100).Strong() {
		t.Fatal("5% co-purchase is noise, not a pattern")
	}
}

func TestSuggestCrossSell_SuggestsWhatTheyDoNotBuy(t *testing.T) {
	out := SuggestCrossSell(
		[]string{"Luvas de procedimento cx 100"},
		[]CoPurchasePair{pair("luva de procedimento", "mascara cirurgica", 8, 10)},
		3)

	if len(out) != 1 {
		t.Fatalf("expected one suggestion, got %d", len(out))
	}
	if out[0].Suggest != "mascara cirurgica" {
		t.Fatalf("unexpected suggestion %q", out[0].Suggest)
	}
	// The reason is what lets a salesperson say why. A suggestion without one
	// is a guess the customer can hear.
	if out[0].Because != "luva de procedimento" {
		t.Fatalf("expected the anchor as the reason, got %q", out[0].Because)
	}
}

func TestSuggestCrossSell_NeverSuggestsWhatTheyAlreadyBuy(t *testing.T) {
	out := SuggestCrossSell(
		[]string{"luva de procedimento", "Máscara Cirúrgica cx 50"},
		[]CoPurchasePair{pair("luva de procedimento", "mascara cirurgica", 8, 10)},
		3)

	if len(out) != 0 {
		t.Fatalf("expected nothing to suggest, got %+v", out)
	}
}

// A pattern anchored on something this customer does not buy says nothing
// about this customer.
func TestSuggestCrossSell_IgnoresUnanchoredPatterns(t *testing.T) {
	out := SuggestCrossSell(
		[]string{"seringa"},
		[]CoPurchasePair{pair("luva de procedimento", "mascara cirurgica", 8, 10)},
		3)

	if len(out) != 0 {
		t.Fatalf("expected nothing, got %+v", out)
	}
}

// A panel listing "máscara (porque luva)" and "máscara (porque avental)" is
// the same idea twice.
func TestSuggestCrossSell_OneReasonPerSuggestion(t *testing.T) {
	out := SuggestCrossSell(
		[]string{"luva", "avental"},
		[]CoPurchasePair{
			pair("luva", "mascara", 4, 10),
			pair("avental", "mascara", 9, 10),
		}, 3)

	if len(out) != 1 {
		t.Fatalf("expected one suggestion, got %+v", out)
	}
	// And it keeps the STRONGER reason.
	if out[0].Because != "avental" {
		t.Fatalf("expected the strongest anchor, got %q", out[0].Because)
	}
}

func TestSuggestCrossSell_StrongestFirstAndStable(t *testing.T) {
	pairs := []CoPurchasePair{
		pair("luva", "avental", 4, 10),
		pair("luva", "mascara", 9, 10),
		pair("luva", "touca", 4, 10),
	}
	first := SuggestCrossSell([]string{"luva"}, pairs, 3)
	if first[0].Suggest != "mascara" {
		t.Fatalf("expected the strongest first, got %q", first[0].Suggest)
	}
	// A panel that reshuffles on refresh looks broken.
	second := SuggestCrossSell([]string{"luva"}, pairs, 3)
	for i := range first {
		if first[i].Suggest != second[i].Suggest {
			t.Fatalf("order is not stable: %v vs %v", first, second)
		}
	}
}

func TestNormalizeItemLabel_CollapsesPackagingAndAccents(t *testing.T) {
	cases := map[string]string{
		"Luvas de procedimento cx 100":      "luva de procedimento",
		"LUVAS DE PROCEDIMENTO caixa c/100": "luva de procedimento",
		"Máscara Cirúrgica":                 "mascara cirurgica",
		// The plural fold: two spellings of one product are one product.
		"Luva":                      "luva",
		"Luvas":                     "luva",
		"  Avental   descartável  ": "avental descartavel",
	}
	for in, want := range cases {
		if got := NormalizeItemLabel(in); got != want {
			t.Errorf("NormalizeItemLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
