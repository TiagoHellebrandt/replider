package services

import "testing"

func TestProviderCostUSD(t *testing.T) {
	// gpt-4o-mini: 0.15/M in, 0.60/M out. 1M in + 1M out = 0.75.
	got := ProviderCostUSD("openai", "gpt-4o-mini", 1_000_000, 1_000_000)
	if got < 0.7499 || got > 0.7501 {
		t.Fatalf("gpt-4o-mini cost = %v, want ~0.75", got)
	}

	// Longest-prefix match: a versioned model id resolves to its base rate.
	versioned := ProviderCostUSD("google", "gemini-1.5-pro-002", 1_000_000, 0)
	if versioned < 1.2499 || versioned > 1.2501 {
		t.Fatalf("gemini-1.5-pro-002 input cost = %v, want ~1.25", versioned)
	}

	// Unknown model falls back to the provider default (not zero).
	fallback := ProviderCostUSD("google", "totally-new-model", 1_000_000, 0)
	if fallback <= 0 {
		t.Fatalf("provider fallback cost = %v, want > 0", fallback)
	}

	// Zero tokens => zero cost.
	if z := ProviderCostUSD("openai", "gpt-4o", 0, 0); z != 0 {
		t.Fatalf("zero-token cost = %v, want 0", z)
	}
}

func TestEstimateTokens(t *testing.T) {
	if got := EstimateTokens("12345678"); got != 2 {
		t.Fatalf("EstimateTokens(8 chars) = %d, want 2", got)
	}
	if got := EstimateTokens(""); got != 0 {
		t.Fatalf("EstimateTokens(empty) = %d, want 0", got)
	}
}

// Cached-prompt pricing (K3).
//
// The cached count is a SUBSET of the prompt total, and this is a margin figure —
// so the only acceptable direction to be wrong in is upward.

func TestProviderCostUSDCached_CachedTokensAreDiscountedNotAdded(t *testing.T) {
	const provider, model = "google", "gemini-2.0-flash"

	full := ProviderCostUSD(provider, model, 10_000, 500)
	// The same call, with 8k of the same 10k prompt served from the cache.
	cached := ProviderCostUSDCached(provider, model, 10_000, 8_000, 500)

	if !(cached < full) {
		t.Fatalf("cached %v is not cheaper than full %v", cached, full)
	}
	// 2k at full rate + 8k at a quarter = the equivalent of 4k, so the input
	// half costs 40% of what it did. Computed rather than hardcoded so the
	// assertion survives a rate-table edit.
	rate := resolveRate(provider, model)
	wantInput := (2_000 + 8_000*CachedInputDiscount) / 1_000_000 * rate.inputPerM
	wantOutput := 500.0 / 1_000_000 * rate.outputPerM
	if diff := cached - (wantInput + wantOutput); diff > 1e-12 || diff < -1e-12 {
		t.Fatalf("cached = %v, want %v", cached, wantInput+wantOutput)
	}
}

func TestProviderCostUSDCached_ZeroCachedPricesExactlyLikeBefore(t *testing.T) {
	// A provider that reports no cached count (every streaming path today) must
	// be billed identically to before this function existed.
	full := ProviderCostUSD("openai", "gpt-4o-mini", 10_000, 500)
	none := ProviderCostUSDCached("openai", "gpt-4o-mini", 10_000, 0, 500)

	if full != none {
		t.Fatalf("cached-aware pricing changed an uncached call: %v vs %v", none, full)
	}
}

func TestProviderCostUSDCached_ClampsAnImpossibleCachedCount(t *testing.T) {
	// A provider bug or a mis-mapped field reporting more cached than prompt
	// would otherwise produce a NEGATIVE full-rate remainder and an
	// under-stated cost — the wrong direction for a margin figure.
	clamped := ProviderCostUSDCached("google", "gemini-2.0-flash", 1_000, 9_999, 100)
	allCached := ProviderCostUSDCached("google", "gemini-2.0-flash", 1_000, 1_000, 100)

	if clamped != allCached {
		t.Fatalf("over-reported cache = %v, want the all-cached price %v", clamped, allCached)
	}
	if clamped <= 0 {
		t.Fatalf("cost = %v, must never go to zero or negative", clamped)
	}

	negative := ProviderCostUSDCached("google", "gemini-2.0-flash", 1_000, -50, 100)
	if negative != ProviderCostUSD("google", "gemini-2.0-flash", 1_000, 100) {
		t.Fatalf("a negative cached count must be treated as zero, got %v", negative)
	}
}

func TestProviderCostUSDCached_NoTokensIsFree(t *testing.T) {
	if got := ProviderCostUSDCached("google", "gemini-2.0-flash", 0, 0, 0); got != 0 {
		t.Fatalf("cost = %v, want 0", got)
	}
}
