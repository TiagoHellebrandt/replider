package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-023 follow-up of the ledger audit — three concerns pinned against
// the real schema:
//   (1) a clawback (downward recompute of a PAID cycle) debits the
//       seller who received the money, not the account's current one;
//   (2) GetPlanByStripePriceID maps monthly AND annual price ids to the
//       right tier and refuses to guess on a shared/misconfigured id;
//   (3) migration 000134's DROP CONSTRAINT hit the real auto-generated
//       CHECK names — no silent no-op leaving the strict >= 0 rail up.

// A paid R$200 (20% of 100000) for June -> account reallocated A→B ->
// profit recomputed down to 50000 (R$100 due) -> the -R$100 adjustment
// lands under A, and B's payout settles nothing of it. FAILS on the
// pre-US-023 code: the negative adjustment was minted under B.
func TestE2E_ClawbackDebitsPaidSellerAfterReallocation(t *testing.T) {
	cleanDB(t)
	accountID, _, sellerA := setupSLAFixture(t, "Cliente", "us023-claw@test.com")
	registerUser(t, "Vendedor B", "us023-clawb@test.com", "pass1234")
	var sellerB string
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT id FROM users WHERE email = $1", "us023-clawb@test.com").Scan(&sellerB))

	ctx := context.Background()
	_, err := testPool.Exec(ctx,
		"UPDATE accounts SET seller_user_id = $1, lead_source = 'inbound' WHERE id = $2", sellerA, accountID)
	require.NoError(t, err)

	repo := postgres.NewCommissionRepository(testPool)
	uc := usecases.NewComputeCommissionsUseCase(repo,
		postgres.NewUserRepository(testPool, nil), postgres.NewAuditLogRepository(testPool))

	seedProfit(t, accountID, "2026-06", 100000)
	_, err = uc.ComputeCycle(ctx, "owner@test.com", "2026-06")
	require.NoError(t, err)
	_, _, err = repo.MarkPayoutPaid(ctx, sellerA, "2026-06", time.Now(), nil)
	require.NoError(t, err)

	// Reallocate to B, then the cycle's profit drops.
	_, err = testPool.Exec(ctx, "UPDATE accounts SET seller_user_id = $1 WHERE id = $2", sellerB, accountID)
	require.NoError(t, err)
	seedProfit(t, accountID, "2026-06", 50000)
	entries, err := uc.ComputeCycle(ctx, "owner@test.com", "2026-06")
	require.NoError(t, err)

	var adjustment *domain.CommissionEntry
	for _, e := range entries {
		if e.Reason == domain.CommissionReasonAdjustment {
			require.Nil(t, adjustment, "exactly one adjustment row expected")
			adjustment = e
		}
	}
	require.NotNil(t, adjustment, "downward recompute of a paid cycle must record the delta")
	assert.EqualValues(t, -10000, adjustment.AmountCents)
	assert.Equal(t, sellerA, adjustment.SellerUserID,
		"the clawback debits the seller who was PAID, not the current one")

	// B's payout is untouched by money A received.
	settled, cents, err := repo.MarkPayoutPaid(ctx, sellerB, "2026-06", time.Now(), nil)
	require.NoError(t, err)
	assert.Zero(t, settled)
	assert.Zero(t, cents)

	// A's next payout absorbs the saldo.
	settled, cents, err = repo.MarkPayoutPaid(ctx, sellerA, "2026-06", time.Now(), nil)
	require.NoError(t, err)
	assert.EqualValues(t, 1, settled)
	assert.EqualValues(t, -10000, cents)
}

// Concern #2: GetPlanByStripePriceID against the real seed rows — a
// monthly id and an annual id both resolve to their own tier, an
// unknown id resolves to nothing, and a price id misconfigured onto TWO
// plans resolves to nothing (loudly unmapped -> the webhook alerts)
// instead of silently activating an arbitrary tier.
func TestE2E_PlanStripePriceIDMapsToRightTier(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewPlanRepository(testPool)

	const coreUSD, proUSD = "01PLAN_CORE_USD_0000000", "01PLAN_PRO_USD_00000000"
	stampPrice := func(planID, monthly, annual string) {
		var m, a *string
		if monthly != "" {
			m = &monthly
		}
		if annual != "" {
			a = &annual
		}
		_, err := testPool.Exec(ctx,
			"UPDATE plans SET stripe_price_id = $1, stripe_price_id_annual = $2 WHERE id = $3",
			m, a, planID)
		require.NoError(t, err)
	}
	// plans is seeded by migrations and NOT truncated by cleanDB —
	// restore the seed state so other tests never see these ids.
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(),
			"UPDATE plans SET stripe_price_id = NULL, stripe_price_id_annual = NULL WHERE id IN ($1, $2)",
			coreUSD, proUSD)
	})

	stampPrice(coreUSD, "price_core_monthly", "price_core_annual")
	stampPrice(proUSD, "price_pro_monthly", "")

	plan, err := repo.GetByStripePriceID(ctx, "price_core_monthly")
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, coreUSD, strings.TrimSpace(plan.ID), "monthly price id maps to its own tier")

	plan, err = repo.GetByStripePriceID(ctx, "price_core_annual")
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, coreUSD, strings.TrimSpace(plan.ID), "annual price id maps to the same tier")

	plan, err = repo.GetByStripePriceID(ctx, "price_pro_monthly")
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, proUSD, strings.TrimSpace(plan.ID), "a different tier's price never bleeds over")

	plan, err = repo.GetByStripePriceID(ctx, "price_never_seeded")
	require.NoError(t, err)
	assert.Nil(t, plan, "unknown price id is unmapped, not an arbitrary plan")

	// Misconfiguration: the same price id lands on a SECOND plan (as
	// Pro's annual). The lookup must refuse to guess — nil routes the
	// webhook to its plan_activation_failed alert, never a wrong tier.
	stampPrice(proUSD, "price_pro_monthly", "price_core_monthly")
	plan, err = repo.GetByStripePriceID(ctx, "price_core_monthly")
	require.NoError(t, err)
	assert.Nil(t, plan, "a price id shared by two plans must not silently map to either")
}

// Concern #3: migration 000134 replaced the ledger's reason and
// amount_cents CHECKs by dropping them BY NAME. Postgres names an
// inline column CHECK <table>_<column>_check — this pins that the drop
// actually matched (exactly ONE constraint per column, carrying the
// adjustment exemption). If the drop had been a silent no-op, the old
// strict `amount_cents >= 0` rail would coexist with the new one and
// every negative adjustment insert would fail.
func TestE2E_CommissionLedgerChecksWereReplacedNotDuplicated(t *testing.T) {
	ctx := context.Background()
	rows, err := testPool.Query(ctx, `
		SELECT conname, pg_get_constraintdef(oid)
		FROM pg_constraint
		WHERE conrelid = 'commission_ledger'::regclass AND contype = 'c'`)
	require.NoError(t, err)
	defer rows.Close()

	amountChecks := map[string]string{}
	reasonChecks := map[string]string{}
	for rows.Next() {
		var name, def string
		require.NoError(t, rows.Scan(&name, &def))
		switch {
		case strings.Contains(def, "amount_cents"):
			amountChecks[name] = def
		case strings.Contains(def, "reason"):
			reasonChecks[name] = def
		}
	}
	require.NoError(t, rows.Err())

	require.Len(t, amountChecks, 1,
		"exactly one amount_cents CHECK — a second one means 000134's DROP was a no-op: %v", amountChecks)
	amountDef, ok := amountChecks["commission_ledger_amount_cents_check"]
	require.True(t, ok, "the replacement kept the auto-generated name, got %v", amountChecks)
	assert.Contains(t, amountDef, "adjustment", "the >= 0 rail must exempt adjustments")

	require.Len(t, reasonChecks, 1,
		"exactly one reason CHECK, got %v", reasonChecks)
	reasonDef, ok := reasonChecks["commission_ledger_reason_check"]
	require.True(t, ok, "the replacement kept the auto-generated name, got %v", reasonChecks)
	assert.Contains(t, reasonDef, "'adjustment'", "reason whitelist must include adjustment")
}
