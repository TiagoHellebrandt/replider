package services

import (
	"strings"
	"time"
)

// geminiPromoEndsAt is when gemini-3.7-flash's introductory price ends: the
// rate doubles to $1.50/$7.50 per 1M. `provider_costs` flips on its own
// (it carries both rows with effective_from dates); the static fallback in
// modelRates does not. TestGeminiPromoPricingExpiry fails from this date
// until BOTH this constant and the modelRates row are bumped.
var geminiPromoEndsAt = time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)

// geminiPromoRate is the introductory row as seeded; the expiry test
// compares modelRates against it so a bump that forgets the table is caught.
var geminiPromoRate = tokenRate{inputPerM: 0.75, outputPerM: 3.75}

// US-030: coarse provider-cost estimation for cost attribution analytics
// (top-expensive-conversation dashboard, overage signals). These are
// list-price approximations in USD per 1,000,000 tokens — NOT an
// authoritative billing ledger. Stripe metering still runs off the
// normalized-credit accounting in ProcessMessageUseCase.

// tokenRate is the list price for a provider/model in USD per 1M tokens.
type tokenRate struct {
	inputPerM  float64
	outputPerM float64
}

// modelRates is keyed by a lowercase model-name prefix. Lookup matches the
// longest prefix so "gemini-1.5-pro-002" resolves to the gemini-1.5-pro
// entry before the shorter gemini- default. Rates reflect published list
// prices as of 2026 and should be revisited when providers change pricing.
var modelRates = map[string]tokenRate{
	"gpt-4o-mini":      {inputPerM: 0.15, outputPerM: 0.60},
	"gpt-4o":           {inputPerM: 2.50, outputPerM: 10.00},
	"gpt-4.1-mini":     {inputPerM: 0.40, outputPerM: 1.60},
	"gpt-4.1":          {inputPerM: 2.00, outputPerM: 8.00},
	"gemini-1.5-pro":   {inputPerM: 1.25, outputPerM: 5.00},
	"gemini-2.5-pro":   {inputPerM: 1.25, outputPerM: 10.00},
	"gemini-1.5-flash": {inputPerM: 0.075, outputPerM: 0.30},
	"gemini-2.0-flash": {inputPerM: 0.10, outputPerM: 0.40},
	"gemini-2.5-flash": {inputPerM: 0.30, outputPerM: 2.50},
	// flash-lite was the SDR plan tier before migration 000206.
	"gemini-3.1-flash-lite": {inputPerM: 0.25, outputPerM: 1.50},
	// Introductory price — DOUBLES to $1.50/$7.50 on geminiPromoEndsAt
	// (2027-01-01). The provider_costs table carries both rows; this static
	// fallback must be bumped by hand, and TestGeminiPromoPricingExpiry
	// goes red on that date to make sure it is.
	"gemini-3.7-flash": geminiPromoRate,
}

// providerDefault is the fallback rate when the specific model isn't in the
// table (new model id, custom deployment). Keyed by provider name.
var providerDefault = map[string]tokenRate{
	"openai": {inputPerM: 0.15, outputPerM: 0.60}, // ~gpt-4o-mini
	"google": {inputPerM: 0.10, outputPerM: 0.40}, // ~gemini flash
}

// unknownRate is the last-resort fallback so cost is never silently zero
// for an unrecognized provider+model.
var unknownRate = tokenRate{inputPerM: 0.50, outputPerM: 1.50}

func resolveRate(provider, model string) tokenRate {
	m := strings.ToLower(strings.TrimSpace(model))
	var best string
	for prefix := range modelRates {
		if strings.HasPrefix(m, prefix) && len(prefix) > len(best) {
			best = prefix
		}
	}
	if best != "" {
		return modelRates[best]
	}
	if r, ok := providerDefault[strings.ToLower(strings.TrimSpace(provider))]; ok {
		return r
	}
	return unknownRate
}

// CachedInputDiscount is the fraction of the input rate a CACHED prompt token
// costs (§6.16, K3).
//
// 0.25 — a quarter of list — which is the conservative end of what both providers
// publish for a prefix cache hit (Google bills cached context at 25% of the input
// rate; OpenAI discounts by 50%). Deliberately the same number for both: this
// table is a coarse ESTIMATE feeding a margin dashboard, and a per-provider
// discount here would imply a precision the rest of the table does not have.
// Erring toward the smaller discount means the estimate never flatters the saving.
const CachedInputDiscount = 0.25

// ProviderCostUSD estimates the USD cost of a single AI call from its token
// counts and the agent's provider/model. Returns 0 when both counts are 0.
//
// Cached prompt tokens are NOT priced here — see ProviderCostUSDCached. This
// signature is kept so the call sites that have no cached count (streaming paths
// where the provider reports none) stay unchanged and keep pricing everything at
// full rate, which is the safe direction.
func ProviderCostUSD(provider, model string, inputTokens, outputTokens int64) float64 {
	if inputTokens <= 0 && outputTokens <= 0 {
		return 0
	}
	r := resolveRate(provider, model)
	cost := float64(inputTokens)/1_000_000*r.inputPerM + float64(outputTokens)/1_000_000*r.outputPerM
	if cost < 0 {
		return 0
	}
	return cost
}

// ProviderCostUSDCached prices a call whose prompt partly hit the provider's
// prefix cache.
//
// `cachedTokens` is a SUBSET of inputTokens, not an addition to it: every provider
// reports the cached count as part of the prompt total. Adding them would
// double-bill the cached half, which is the opposite of the point.
//
// A cached count larger than the input total (a provider bug, a mis-mapped field)
// is clamped rather than trusted: it would otherwise produce a negative
// full-rate remainder and an under-stated cost, and this number feeds a margin
// figure that has to err upward.
func ProviderCostUSDCached(provider, model string, inputTokens, cachedTokens, outputTokens int64) float64 {
	if cachedTokens < 0 {
		cachedTokens = 0
	}
	if cachedTokens > inputTokens {
		cachedTokens = inputTokens
	}
	full := inputTokens - cachedTokens
	if inputTokens <= 0 && outputTokens <= 0 {
		return 0
	}
	r := resolveRate(provider, model)
	cost := float64(full)/1_000_000*r.inputPerM +
		float64(cachedTokens)/1_000_000*r.inputPerM*CachedInputDiscount +
		float64(outputTokens)/1_000_000*r.outputPerM
	if cost < 0 {
		return 0
	}
	return cost
}

// EstimateTokens is the len(text)/4 fallback used when the provider omits a
// usage report (some streaming responses don't surface token counts). It is
// the standard rough heuristic of ~4 characters per token.
func EstimateTokens(text string) int64 {
	if text == "" {
		return 0
	}
	return int64(len(text) / 4)
}
