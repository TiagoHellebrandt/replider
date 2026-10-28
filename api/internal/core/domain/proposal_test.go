package domain

import "testing"

// A proposal is money. These pin the arithmetic and, more importantly, the
// refusals: a total nobody can defend is worse than an error.

func catalogue(products ...*Product) map[string]*Product {
	out := make(map[string]*Product, len(products))
	for _, p := range products {
		out[p.ID] = p
	}
	return out
}

func TestBuildItemsSnapshotsNameAndPrice(t *testing.T) {
	gauze := &Product{ID: "p1", Name: "Gaze estéril 10x10", PriceCents: 4990, Currency: "BRL"}
	items, currency, err := BuildItems(
		[]ProposalLine{{ProductID: "p1", Quantity: 3}},
		catalogue(gauze),
	)
	if err != nil {
		t.Fatalf("BuildItems: %v", err)
	}
	if currency != "BRL" {
		t.Fatalf("currency = %q, want BRL", currency)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Name != "Gaze estéril 10x10" || items[0].UnitPriceCents != 4990 {
		t.Fatalf("item did not snapshot the catalogue: %+v", items[0])
	}

	// The point of the snapshot: repricing the catalogue must not restate an
	// offer already made.
	gauze.PriceCents = 9990
	gauze.Name = "Gaze estéril 10x10 (novo fornecedor)"
	if items[0].UnitPriceCents != 4990 {
		t.Fatalf("a catalogue reprice reached an existing item: %d", items[0].UnitPriceCents)
	}
	if items[0].Name != "Gaze estéril 10x10" {
		t.Fatalf("a catalogue rename reached an existing item: %q", items[0].Name)
	}
	if got := items[0].TotalCents(); got != 14970 {
		t.Fatalf("TotalCents = %d, want 14970", got)
	}
}

// Summing BRL and USD would invent a number; picking one would misstate the
// rest. Neither is acceptable, so this refuses.
func TestBuildItemsRefusesMixedCurrency(t *testing.T) {
	_, _, err := BuildItems(
		[]ProposalLine{
			{ProductID: "p1", Quantity: 1},
			{ProductID: "p2", Quantity: 1},
		},
		catalogue(
			&Product{ID: "p1", Name: "Gaze", PriceCents: 1000, Currency: "BRL"},
			&Product{ID: "p2", Name: "Swab", PriceCents: 1000, Currency: "USD"},
		),
	)
	if err != ErrProposalMixedCurrency {
		t.Fatalf("err = %v, want ErrProposalMixedCurrency", err)
	}
}

// A product with no currency cannot be priced into an offer. Falling back to the
// account default would be right most of the time and wrong silently.
func TestBuildItemsRefusesProductWithoutCurrency(t *testing.T) {
	_, _, err := BuildItems(
		[]ProposalLine{{ProductID: "p1", Quantity: 1}},
		catalogue(&Product{ID: "p1", Name: "Gaze", PriceCents: 1000, Currency: "  "}),
	)
	if err != ErrProposalMixedCurrency {
		t.Fatalf("err = %v, want ErrProposalMixedCurrency", err)
	}
}

func TestBuildItemsNormalizesCurrencyCase(t *testing.T) {
	_, currency, err := BuildItems(
		[]ProposalLine{
			{ProductID: "p1", Quantity: 1},
			{ProductID: "p2", Quantity: 1},
		},
		catalogue(
			&Product{ID: "p1", Name: "Gaze", PriceCents: 1000, Currency: "brl"},
			&Product{ID: "p2", Name: "Swab", PriceCents: 2000, Currency: "BRL"},
		),
	)
	if err != nil {
		t.Fatalf("BuildItems: %v", err)
	}
	if currency != "BRL" {
		t.Fatalf("currency = %q, want BRL — case should not split a currency", currency)
	}
}

func TestBuildItemsRefusesEmptyAndBadQuantity(t *testing.T) {
	if _, _, err := BuildItems(nil, nil); err != ErrProposalNoItems {
		t.Fatalf("empty: err = %v, want ErrProposalNoItems", err)
	}

	c := catalogue(&Product{ID: "p1", Name: "Gaze", PriceCents: 1000, Currency: "BRL"})
	for _, qty := range []int{0, -2} {
		_, _, err := BuildItems([]ProposalLine{{ProductID: "p1", Quantity: qty}}, c)
		if err != ErrProposalQuantity {
			t.Fatalf("quantity %d: err = %v, want ErrProposalQuantity", qty, err)
		}
	}
}

func TestBuildItemsRefusesUnknownProduct(t *testing.T) {
	_, _, err := BuildItems(
		[]ProposalLine{{ProductID: "ghost", Quantity: 1}},
		catalogue(&Product{ID: "p1", Name: "Gaze", PriceCents: 1000, Currency: "BRL"}),
	)
	if err != ErrProductNotFound {
		t.Fatalf("err = %v, want ErrProductNotFound", err)
	}
}

func TestProposalTotalSumsLines(t *testing.T) {
	p := &Proposal{
		Currency: "BRL",
		Items: []ProposalItem{
			{Name: "Gaze", Quantity: 3, UnitPriceCents: 4990},
			{Name: "Luva", Quantity: 2, UnitPriceCents: 12550},
		},
	}
	if got := p.TotalCents(); got != 40070 {
		t.Fatalf("TotalCents = %d, want 40070", got)
	}
}

// A sent proposal is a statement somebody is holding. Editing it would make our
// record disagree with their copy.
func TestSentProposalIsNotEditable(t *testing.T) {
	if !(&Proposal{Status: ProposalDraft}).Editable() {
		t.Fatal("a draft must be editable")
	}
	if (&Proposal{Status: ProposalSent}).Editable() {
		t.Fatal("a sent proposal must not be editable")
	}
}

// The separator follows the currency. "1.234,56" and "1,234.56" are the same
// amount to a machine and a thousand times apart to a reader who assumes the
// other convention — and this string goes to a customer.
func TestFormatMoneyCentsFollowsTheCurrency(t *testing.T) {
	cases := []struct {
		cents    int64
		currency string
		want     string
	}{
		{123456, "BRL", "1.234,56 BRL"},
		{123456, "USD", "1,234.56 USD"},
		{123456789, "BRL", "1.234.567,89 BRL"},
		{123456789, "USD", "1,234,567.89 USD"},
		{4990, "BRL", "49,90 BRL"},
		{5, "BRL", "0,05 BRL"},
		{0, "USD", "0.00 USD"},
		{100, "brl", "1,00 BRL"},
		// Grouping must not fire at exactly three digits.
		{99900, "BRL", "999,00 BRL"},
		{100000, "BRL", "1.000,00 BRL"},
		// A credit or an adjustment can be negative; the sign belongs outside.
		{-4990, "BRL", "-49,90 BRL"},
		// No currency: the amount still reads, with no code invented for it.
		{4990, "", "49.90"},
	}

	for _, c := range cases {
		if got := FormatMoneyCents(c.cents, c.currency); got != c.want {
			t.Errorf("FormatMoneyCents(%d, %q) = %q, want %q", c.cents, c.currency, got, c.want)
		}
	}
}
