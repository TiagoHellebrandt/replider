package postgres

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// MonthlyProfitRepository is a raw-SQL repo (NOT sqlc) over the same
// pool, owning account_monthly_profit plus the token aggregation the
// profit computation reads (US-006, vendedores v3). Same rationale as
// ProviderCostRepository: isolated feature, keeps the expensive-to-
// extend sqlc repos untouched.
type MonthlyProfitRepository struct {
	db DBTX
}

func NewMonthlyProfitRepository(db DBTX) *MonthlyProfitRepository {
	return &MonthlyProfitRepository{db: db}
}

// Upsert overwrites the (account_id, cycle) row — recomputes are
// idempotent, never duplicated. The original id survives (DO UPDATE
// doesn't touch it); computed_at always refreshes.
func (r *MonthlyProfitRepository) Upsert(ctx context.Context, p *domain.AccountMonthlyProfit) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO account_monthly_profit
		     (id, account_id, cycle, revenue_cents, stripe_fee_cents, token_cost_cents, tax_cents, infra_cents, net_profit_cents, computed_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		 ON CONFLICT (account_id, cycle) DO UPDATE SET
		     revenue_cents    = EXCLUDED.revenue_cents,
		     stripe_fee_cents = EXCLUDED.stripe_fee_cents,
		     token_cost_cents = EXCLUDED.token_cost_cents,
		     tax_cents        = EXCLUDED.tax_cents,
		     infra_cents      = EXCLUDED.infra_cents,
		     net_profit_cents = EXCLUDED.net_profit_cents,
		     computed_at      = NOW()
		 RETURNING id, computed_at`,
		p.ID, p.AccountID, p.Cycle, p.RevenueCents, p.StripeFeeCents, p.TokenCostCents, p.TaxCents, p.InfraCents, p.NetProfitCents,
	).Scan(&p.ID, &p.ComputedAt)
}

func (r *MonthlyProfitRepository) ListByCycle(ctx context.Context, cycle string) ([]*domain.AccountMonthlyProfit, error) {
	rows, err := r.db.Query(ctx,
		`SELECT p.id, p.account_id, p.cycle, p.revenue_cents, p.stripe_fee_cents, p.token_cost_cents,
		        p.tax_cents, p.infra_cents, p.net_profit_cents, p.computed_at, a.name
		 FROM account_monthly_profit p
		 JOIN accounts a ON a.id = p.account_id
		 WHERE p.cycle = $1
		 ORDER BY p.net_profit_cents ASC`,
		cycle,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*domain.AccountMonthlyProfit{}
	for rows.Next() {
		var p domain.AccountMonthlyProfit
		if err := rows.Scan(&p.ID, &p.AccountID, &p.Cycle, &p.RevenueCents, &p.StripeFeeCents, &p.TokenCostCents,
			&p.TaxCents, &p.InfraCents, &p.NetProfitCents, &p.ComputedAt, &p.AccountName); err != nil {
			return nil, err
		}
		p.IsNegative = p.NetProfitCents < 0
		out = append(out, &p)
	}
	return out, rows.Err()
}

// SumTokenCost sums the cycle's AI cost from the per-message stamp
// (messages.provider_cost_usd, written at send time) — immutable
// attribution, no join with the mutable agents row: deleting or
// repointing an agent never rewrites a past cycle's cost (US-007
// review fix). Rows with tokens but no stamped cost are returned as
// unpriced token counts so the computation can warn.
func (r *MonthlyProfitRepository) SumTokenCost(ctx context.Context, accountID string, from, to time.Time) (domain.ProfitTokenCost, error) {
	var tc domain.ProfitTokenCost
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(m.provider_cost_usd), 0)::FLOAT8,
		        COALESCE(SUM(m.input_tokens)  FILTER (WHERE COALESCE(m.provider_cost_usd, 0) = 0), 0)::BIGINT,
		        COALESCE(SUM(m.output_tokens) FILTER (WHERE COALESCE(m.provider_cost_usd, 0) = 0), 0)::BIGINT
		 FROM messages m
		 JOIN conversations c ON c.id = m.conversation_id
		 WHERE c.account_id = $1
		   AND m.created_at >= $2 AND m.created_at < $3
		   AND (COALESCE(m.input_tokens, 0) > 0 OR COALESCE(m.output_tokens, 0) > 0)`,
		accountID, from, to,
	).Scan(&tc.CostUSD, &tc.UnpricedInputTokens, &tc.UnpricedOutputTokens)
	return tc, err
}

// ListComputeTargets returns every account with a seller linked
// (US-002) — the commission base the cycle recompute iterates.
func (r *MonthlyProfitRepository) ListComputeTargets(ctx context.Context) ([]domain.ProfitComputeTarget, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, stripe_customer_id FROM accounts WHERE seller_user_id IS NOT NULL ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ProfitComputeTarget{}
	for rows.Next() {
		var t domain.ProfitComputeTarget
		if err := rows.Scan(&t.AccountID, &t.StripeCustomerID); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
