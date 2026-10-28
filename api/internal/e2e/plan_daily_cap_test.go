package e2e

import (
	"context"
	"testing"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/require"
)

// TestE2E_PlanDailyTokenCapScalesWithPlan locks in review-fixes US-016
// against the REAL schema + seed data: plans.max_daily_ai_tokens exists,
// is populated per plan, and the repository maps it back — so paid tiers
// no longer fall to the 100K platform default that capped USD Pro (105M
// tokens/month marketed) at ~3M/month in practice.
//
// Fails on the pre-US-016 schema: the column doesn't exist, the repo
// maps nothing, EffectiveMaxDailyAITokens() == 100K for every plan.
func TestE2E_PlanDailyTokenCapScalesWithPlan(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewPlanRepository(testPool)

	// AC scenario: a Pro USD account consuming >100K tokens in a single
	// day, well inside its 105M monthly franchise, must NOT trip the
	// daily gate (process_message blocks when dailyUsed >= dailyCap).
	pro, err := repo.GetByID(ctx, "01PLAN_PRO_USD_00000000")
	require.NoError(t, err)
	require.NotNil(t, pro, "USD Pro seed row missing")
	dailyUsed := int64(150_000)
	require.Less(t, dailyUsed, pro.EffectiveMaxDailyAITokens(),
		"a >100K-token day within the monthly franchise must not be blocked on Pro USD")

	// Every plan with a monthly franchise must be able to actually reach
	// it: 30 × daily cap >= monthly allowance, and no paid tier below
	// the old effective default.
	plans, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, plans)
	for _, p := range plans {
		if p.MaxAITokensPerMonth <= 0 {
			continue // full-custom contract (e.g. Partner) keeps the default
		}
		daily := p.EffectiveMaxDailyAITokens()
		require.GreaterOrEqualf(t, daily*30, p.MaxAITokensPerMonth,
			"plan %s (%s): daily cap %d throttles below the marketed monthly allowance %d",
			p.Name, p.Currency, daily, p.MaxAITokensPerMonth)
		require.GreaterOrEqualf(t, daily, domain.DefaultMaxDailyAITokens,
			"plan %s (%s): daily cap regressed below the platform default", p.Name, p.Currency)
	}

	// GetByStripePriceID maps the column too (checkout activation path
	// hands this plan to limit checks).
	priceID := "price_daily_cap_test"
	_, err = testPool.Exec(ctx,
		"UPDATE plans SET stripe_price_id = $1 WHERE id = '01PLAN_CORE_USD_0000000'", priceID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(),
			"UPDATE plans SET stripe_price_id = NULL WHERE id = '01PLAN_CORE_USD_0000000'")
	})
	core, err := repo.GetByStripePriceID(ctx, priceID)
	require.NoError(t, err)
	require.NotNil(t, core)
	require.Greater(t, core.MaxDailyAITokens, domain.DefaultMaxDailyAITokens,
		"GetByStripePriceID must map max_daily_ai_tokens")

	// Trial keeps the deliberately low in-code default.
	require.Equal(t, domain.DefaultMaxDailyAITokens,
		domain.TrialPlan().EffectiveMaxDailyAITokens())
}
