package domain

import (
	"context"
	"time"
)

// AccountMonthlyProfit is one (account, cycle) row of the profit table
// (US-006, vendedores v3) — the base the commission ledger (US-007)
// applies its rates to. All money fields are integer BRL cents. Cycle
// is the calendar month "YYYY-MM" (UTC boundaries). Recomputes upsert
// on UNIQUE(account_id, cycle): idempotent, never duplicated.
type AccountMonthlyProfit struct {
	ID             string `json:"id"`
	AccountID      string `json:"account_id"`
	Cycle          string `json:"cycle"`
	RevenueCents   int64  `json:"revenue_cents"`
	StripeFeeCents int64  `json:"stripe_fee_cents"`
	TokenCostCents int64  `json:"token_cost_cents"`
	TaxCents       int64  `json:"tax_cents"`
	InfraCents     int64  `json:"infra_cents"`
	NetProfitCents int64  `json:"net_profit_cents"`
	// IsNegative is derived (net < 0), surfaced for the owner panel's
	// recurring-negative-margin flag. Never stored.
	IsNegative bool      `json:"is_negative"`
	ComputedAt time.Time `json:"computed_at"`
	// AccountName is joined on listing reads; empty elsewhere.
	AccountName string `json:"account_name,omitempty"`
}

// ProfitTokenCost is the cycle's AI cost summed from the per-message
// stamp (messages.provider_cost_usd, written at send time) — immutable
// attribution: deleting or repointing the conversation's agent never
// changes a past cycle's cost (US-007 review fix; the old aggregation
// JOINed the mutable agents row and zeroed retroactively). Unpriced
// tokens are message rows carrying tokens but no stamped cost; they
// cost zero and are surfaced so the computation can warn.
type ProfitTokenCost struct {
	CostUSD              float64
	UnpricedInputTokens  int64
	UnpricedOutputTokens int64
}

// ProfitComputeTarget is one account the cycle recompute iterates:
// accounts with a seller linked (the commission base — US-002).
type ProfitComputeTarget struct {
	AccountID        string
	StripeCustomerID *string
}

// MonthlyProfitRepository owns account_monthly_profit plus the token
// aggregation the computation reads.
type MonthlyProfitRepository interface {
	// Upsert overwrites the (account_id, cycle) row, refreshing
	// computed_at; the original id survives a recompute.
	Upsert(ctx context.Context, p *AccountMonthlyProfit) error
	// ListByCycle returns the cycle's rows with account names joined,
	// worst net first.
	ListByCycle(ctx context.Context, cycle string) ([]*AccountMonthlyProfit, error)
	// SumTokenCost sums the stamped per-message provider cost of
	// messages created in [from, to) for the account — no join with
	// the mutable agents row.
	SumTokenCost(ctx context.Context, accountID string, from, to time.Time) (ProfitTokenCost, error)
	// ListComputeTargets returns every account with a seller linked.
	ListComputeTargets(ctx context.Context) ([]ProfitComputeTarget, error)
}

// Platform-settings keys consumed by the profit computation, both
// owner-editable via PUT /owner/profit-settings.
const (
	// SettingProfitTaxBps: tax on revenue in basis points (600 = 6%).
	SettingProfitTaxBps = "profit_tax_bps"
	// SettingInfraCostCents: flat infra cost per account per month in
	// BRL cents (default 500 = R$5.00).
	SettingInfraCostCents = "infra_cost_cents"
)
