package domain

import (
	"context"
	"time"
)

// Transparent token metering (US-M015).
//
// The product's promise in "modo simples" is that the customer pays the
// provider's real price with no markup, visible on the invoice. That only
// holds if EVERY AI call is metered — including the ones that never produce
// a message (drafts, enrichment), which the per-message stamp misses.

// AICostEntry is one metered AI call.
type AICostEntry struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	// Operation is what was being done ('approach_draft', 'enrichment'…).
	Operation    string `json:"operation"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	// CachedTokens is the part of InputTokens the provider served from its
	// prefix cache (K3). A SUBSET of InputTokens, never an addition.
	CachedTokens int64 `json:"cached_tokens"`
	// CostUSD is stamped at call time and never recomputed — the price
	// table is editable, and a past month's invoice must not move.
	CostUSD    float64   `json:"cost_usd"`
	OccurredAt time.Time `json:"occurred_at"`
}

// AICostSummary is one period's spend, ready to show on an invoice.
type AICostSummary struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
	// TotalUSD is the raw provider cost.
	TotalUSD float64 `json:"total_usd"`
	// TotalBRLCents is TotalUSD converted at the platform rate, rounded
	// ONCE at the end. Rounding each entry first would drift by cents on a
	// busy account — and cents are the entire credibility of a
	// "no markup" line.
	TotalBRLCents int64 `json:"total_brl_cents"`
	// RateCents is the USD→BRL rate applied, exposed so the number can be
	// checked rather than trusted.
	RateCents int `json:"rate_cents"`
	// ByOperation breaks the spend down, because "IA: R$43" invites the
	// question "doing what?".
	ByOperation map[string]float64 `json:"by_operation_usd"`
	// CachedTokens and InputTokens make the prompt-caching saving visible
	// (K3): the ratio between them is how well the prompt is layered, and
	// without it "structured for caching" is an unverifiable claim.
	CachedTokens int64 `json:"cached_tokens"`
	InputTokens  int64 `json:"input_tokens"`
}

// AICostRepository persists and aggregates the ledger.
type AICostRepository interface {
	Record(ctx context.Context, e *AICostEntry) error
	// SumByAccount aggregates the ledger over [from, to). The ledger alone:
	// conversational replies land here as "agent_reply", and adding the
	// per-message stamp on top counted them twice (fixed 2026-09-01).
	SumByAccount(ctx context.Context, accountID string, from, to time.Time) (*AICostSummary, error)
	// CountOperations backs the per-plan spend caps (K4). The ledger is the
	// counter: a dedicated per-kind column would be a second source of truth
	// that drifts the first time a write is missed.
	CountOperations(ctx context.Context, accountID string, operations []string, since time.Time) (int64, error)
}
