package domain

import "testing"

// Four of six production agents carried google + gpt-4o. Nothing on the way
// in said no.
func TestValidateModelForProvider_RefusesTheProductionMismatch(t *testing.T) {
	if err := ValidateModelForProvider("google", "gpt-4o"); err == nil {
		t.Fatal("google + gpt-4o must be refused")
	}
	if err := ValidateModelForProvider("openai", "gemini-3.7-flash"); err == nil {
		t.Fatal("openai + gemini must be refused")
	}
}

func TestValidateModelForProvider_AcceptsMatchingPairs(t *testing.T) {
	cases := []struct{ provider, model string }{
		{"google", "gemini-3.7-flash"},
		{"gemini", "gemini-2.5-flash"},
		{"openai", "gpt-4o"},
		{"openai", "gpt-4o-2024-08-06"},
		{"openai", "o3-mini"},
	}
	for _, c := range cases {
		if err := ValidateModelForProvider(c.provider, c.model); err != nil {
			t.Fatalf("%s + %s = %v, want accepted", c.provider, c.model, err)
		}
	}
}

// The check catches what is provably wrong; it is not an allow-list. A model
// this file has never heard of must still save, or the next vendor release
// becomes an outage.
func TestValidateModelForProvider_LetsUnknownNamesThrough(t *testing.T) {
	cases := []struct{ provider, model string }{
		{"google", "some-new-model-2027"},
		{"openai", ""},
		{"", "gpt-4o"},
		{"anthropic", "claude-opus"},
	}
	for _, c := range cases {
		if err := ValidateModelForProvider(c.provider, c.model); err != nil {
			t.Fatalf("%q + %q = %v, want accepted", c.provider, c.model, err)
		}
	}
}
