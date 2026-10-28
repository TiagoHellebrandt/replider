package domain

import "testing"

func TestNormalizeBillingInterval(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"", BillingIntervalMonthly, true},
		{"monthly", BillingIntervalMonthly, true},
		{"annual", BillingIntervalAnnual, true},
		{"yearly", BillingIntervalMonthly, false},
		{"ANNUAL", BillingIntervalMonthly, false}, // case-sensitive wire value
		{"garbage", BillingIntervalMonthly, false},
	}
	for _, c := range cases {
		got, ok := NormalizeBillingInterval(c.in)
		if got != c.want || ok != c.wantOK {
			t.Errorf("NormalizeBillingInterval(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.wantOK)
		}
	}
}

func TestResolveStripePriceID(t *testing.T) {
	monthly := "price_monthly"
	annual := "price_annual"

	withBoth := &Plan{StripePriceID: &monthly, StripePriceIDAnnual: &annual}
	if got := withBoth.ResolveStripePriceID(BillingIntervalAnnual); got != annual {
		t.Errorf("annual interval should pick annual price, got %q", got)
	}
	if got := withBoth.ResolveStripePriceID(BillingIntervalMonthly); got != monthly {
		t.Errorf("monthly interval should pick monthly price, got %q", got)
	}
	if got := withBoth.ResolveStripePriceID(""); got != monthly {
		t.Errorf("empty interval should default to monthly price, got %q", got)
	}

	// Annual requested but unconfigured → fall back to monthly.
	monthlyOnly := &Plan{StripePriceID: &monthly}
	if got := monthlyOnly.ResolveStripePriceID(BillingIntervalAnnual); got != monthly {
		t.Errorf("annual with no annual price should fall back to monthly, got %q", got)
	}

	// Empty annual string is treated as unconfigured.
	empty := ""
	emptyAnnual := &Plan{StripePriceID: &monthly, StripePriceIDAnnual: &empty}
	if got := emptyAnnual.ResolveStripePriceID(BillingIntervalAnnual); got != monthly {
		t.Errorf("empty annual price should fall back to monthly, got %q", got)
	}

	// Contract plan (no monthly price) resolves to "".
	contract := &Plan{}
	if got := contract.ResolveStripePriceID(BillingIntervalMonthly); got != "" {
		t.Errorf("contract plan should resolve to empty, got %q", got)
	}

	// Nil receiver is safe.
	var nilPlan *Plan
	if got := nilPlan.ResolveStripePriceID(BillingIntervalAnnual); got != "" {
		t.Errorf("nil plan should resolve to empty, got %q", got)
	}
}

func TestAllowsBrandingRemoval(t *testing.T) {
	cases := []struct {
		name     string
		currency string
		want     bool
	}{
		{"Trial", "", false},
		{"", "", false},
		// BRL ladder (empty currency = pre-currency rows, all BRL).
		{"Solo", "", true},
		{"Starter", "brl", true},
		{"Pro", "brl", true},
		{"Business", "brl", true},
		{"Scale", "brl", true},
		{"Partner", "brl", true},
		{"Enterprise", "brl", true},
		// USD ladder — every paid tier removes the badge.
		{"Solo", "usd", true},
		{"Core", "usd", true},
		{"Pro", "usd", true},
	}
	for _, c := range cases {
		p := &Plan{Name: c.name, Currency: c.currency}
		if got := p.AllowsBrandingRemoval(); got != c.want {
			t.Errorf("plan %q/%q: AllowsBrandingRemoval() = %v, want %v", c.name, c.currency, got, c.want)
		}
	}
	var nilPlan *Plan
	if nilPlan.AllowsBrandingRemoval() {
		t.Errorf("nil plan should not allow branding removal")
	}
	if TrialPlan().AllowsBrandingRemoval() {
		t.Errorf("TrialPlan should show branding (not allow removal)")
	}
}

// TestFeatureGates_USDLadder pins the (currency, name) qualification of
// the name-keyed feature gates (US-017 review fixes). The load-bearing
// case is Pro USD → AllowsAutoSuggest: with the old name-only switch
// ("Business"+) the top USD tier — and therefore EVERY USD customer —
// could never reach the auto-suggest feature. The Solo/Pro name
// collisions between the BRL and USD ladders must not couple the gates.
func TestFeatureGates_USDLadder(t *testing.T) {
	gates := func(p *Plan) (bool, bool) { return p.AllowsExperiments(), p.AllowsAutoSuggest() }

	cases := []struct {
		name            string
		currency        string
		wantExperiments bool
		wantAutoSuggest bool
	}{
		// USD ladder: Pro is the top tier and gets both.
		{"Pro", "usd", true, true},
		{"Core", "usd", false, false},
		{"Solo", "usd", false, false},
		// BRL ladder unchanged: Pro gets experiments but NOT
		// auto-suggest (Business+); Business gets both.
		{"Pro", "brl", true, false},
		{"Business", "brl", true, true},
		{"Solo", "brl", false, false},
		// Pre-currency rows behave as BRL.
		{"Pro", "", true, false},
	}
	for _, c := range cases {
		p := &Plan{Name: c.name, Currency: c.currency}
		exp, sug := gates(p)
		if exp != c.wantExperiments || sug != c.wantAutoSuggest {
			t.Errorf("plan %q/%q: (experiments, autosuggest) = (%v, %v), want (%v, %v)",
				c.name, c.currency, exp, sug, c.wantExperiments, c.wantAutoSuggest)
		}
	}

	var nilPlan *Plan
	if nilPlan.AllowsExperiments() || nilPlan.AllowsAutoSuggest() {
		t.Errorf("nil plan should gate everything off")
	}
}
