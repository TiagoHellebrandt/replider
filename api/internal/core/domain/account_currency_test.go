package domain

import "testing"

func TestCurrencyFromLocale(t *testing.T) {
	cases := []struct {
		locale string
		want   string
	}{
		// Only a real US-English device maps to USD (US-017 review
		// fixes). The old any-"en*" rule force-locked every
		// non-Portuguese device into USD, because the app's Slang
		// fallback reports bare "en" for German/Spanish/French/… users.
		{"en-US", CurrencyUSD},
		{"en_US", CurrencyUSD},
		{"EN-us", CurrencyUSD},
		{" en-US ", CurrencyUSD},
		// Ambiguous / non-US English defaults to BRL.
		{"en", CurrencyBRL},
		{"en-GB", CurrencyBRL},
		{"en_GB", CurrencyBRL},
		{"de-DE", CurrencyBRL},
		{"es", CurrencyBRL},
		{"pt", CurrencyBRL},
		{"pt-BR", CurrencyBRL},
		{"", CurrencyBRL},
		{"enx", CurrencyBRL},
		{"garbage", CurrencyBRL},
	}
	for _, tc := range cases {
		if got := CurrencyFromLocale(tc.locale); got != tc.want {
			t.Errorf("CurrencyFromLocale(%q) = %q, want %q", tc.locale, got, tc.want)
		}
	}
}

func TestConvertCents(t *testing.T) {
	cases := []struct {
		name   string
		cents  int64
		from   string
		to     string
		rate   int64
		want   int64
		wantOK bool
	}{
		{"same brl passes through", 19900, "brl", "brl", 550, 19900, true},
		{"same usd passes through", 9900, "usd", "usd", 550, 9900, true},
		{"usd to brl multiplies by rate", 9900, "usd", "brl", 550, 54450, true},
		{"brl to usd divides by rate", 19900, "brl", "usd", 550, 3618, true},
		{"case and space insensitive", 9900, " USD ", "BRL", 550, 54450, true},
		{"rounds once at the end", 101, "brl", "usd", 550, 18, true}, // 18.36 → 18
		{"unknown target refused", 9900, "usd", "eur", 550, 0, false},
		{"unknown source refused", 9900, "eur", "brl", 550, 0, false},
		{"same unknown currency refused", 9900, "eur", "eur", 550, 0, false},
		{"non-positive rate refused", 9900, "usd", "brl", 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ConvertCents(tc.cents, tc.from, tc.to, tc.rate)
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("ConvertCents(%d, %q, %q, %d) = (%d, %v), want (%d, %v)",
					tc.cents, tc.from, tc.to, tc.rate, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// US-018 review fixes: transactional emails resolve their locale from
// the account's billing currency — USD accounts get English copy.
func TestEmailLocaleForCurrency(t *testing.T) {
	cases := []struct {
		currency string
		want     string
	}{
		{"usd", "en"},
		{"USD", "en"},
		{" usd ", "en"},
		{"brl", "pt-BR"},
		{"", "pt-BR"},    // legacy pre-currency accounts were BRL
		{"eur", "pt-BR"}, // unknown → safe default
	}
	for _, c := range cases {
		if got := EmailLocaleForCurrency(c.currency); got != c.want {
			t.Errorf("EmailLocaleForCurrency(%q) = %q, want %q", c.currency, got, c.want)
		}
	}
}
