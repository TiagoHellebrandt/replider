package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// US-004 (review fixes), SQL level: exercises the real ledger schema —
// the 000134 constraint change (negative 'adjustment' amounts), the
// paid-sum query and the seller-agnostic bonus dedup — against
// Postgres, complementing the mock-level tests in core/usecases.

// seedProfit upserts the account's cycle profit row.
func seedProfit(t *testing.T, accountID, cycle string, netCents int64) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), `
		INSERT INTO account_monthly_profit (id, account_id, cycle, revenue_cents, net_profit_cents)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (account_id, cycle) DO UPDATE SET net_profit_cents = EXCLUDED.net_profit_cents
	`, ulid.Make().String(), accountID, cycle, netCents)
	require.NoError(t, err)
}

func TestE2E_CommissionPaidCycleRecomputeCreatesAdjustment(t *testing.T) {
	cleanDB(t)
	accountID, _, sellerID := setupSLAFixture(t, "Cliente", "us004-adj@test.com")
	_, err := testPool.Exec(context.Background(),
		"UPDATE accounts SET seller_user_id = $1, lead_source = 'inbound' WHERE id = $2", sellerID, accountID)
	require.NoError(t, err)

	repo := postgres.NewCommissionRepository(testPool)
	uc := usecases.NewComputeCommissionsUseCase(repo,
		postgres.NewUserRepository(testPool, nil), postgres.NewAuditLogRepository(testPool))
	ctx := context.Background()

	// Cycle computed (inbound 20% of 100000 = 20000) and settled.
	seedProfit(t, accountID, "2026-06", 100000)
	_, err = uc.ComputeCycle(ctx, "owner@test.com", "2026-06")
	require.NoError(t, err)
	_, _, err = repo.MarkPayoutPaid(ctx, sellerID, "2026-06", time.Now(), nil)
	require.NoError(t, err)

	// Profit is recomputed DOWN after the early payout: 20% of 50000 =
	// 10000 due vs 20000 paid -> pending adjustment of -10000. The
	// negative amount is what the 000134 constraint change permits.
	seedProfit(t, accountID, "2026-06", 50000)
	entries, err := uc.ComputeCycle(ctx, "owner@test.com", "2026-06")
	require.NoError(t, err)

	var paid, adjustment *domain.CommissionEntry
	for _, e := range entries {
		switch e.Reason {
		case domain.CommissionReasonStandard:
			paid = e
		case domain.CommissionReasonAdjustment:
			adjustment = e
		}
	}
	require.NotNil(t, paid, "settled row must survive the recompute")
	assert.Equal(t, domain.CommissionStatusPaid, paid.Status)
	assert.EqualValues(t, 20000, paid.AmountCents)
	require.NotNil(t, adjustment, "recompute on a paid cycle must record the delta")
	assert.Equal(t, domain.CommissionStatusPending, adjustment.Status)
	assert.EqualValues(t, -10000, adjustment.AmountCents)
	require.Len(t, entries, 2, "no second full profit-share row may appear")

	// SumPaidProfitShare sees the settled money under any seller.
	total, paidSeller, hasPaid, err := repo.SumPaidProfitShare(ctx, accountID, "2026-06")
	require.NoError(t, err)
	assert.True(t, hasPaid)
	assert.EqualValues(t, 20000, total)
	assert.Equal(t, sellerID, paidSeller, "paid seller is who the settle went to")
}

func TestE2E_CommissionReallocationAfterPaidCycle(t *testing.T) {
	cleanDB(t)
	accountID, _, sellerA := setupSLAFixture(t, "Cliente", "us004-realloc@test.com")
	registerUser(t, "Vendedor B", "us004-sellerb@test.com", "pass1234")
	var sellerB string
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT id FROM users WHERE email = $1", "us004-sellerb@test.com").Scan(&sellerB))

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

	// Owner reallocates the account to seller B and recomputes June:
	// same profit -> delta 0 -> B gets NOTHING for the settled cycle.
	_, err = testPool.Exec(ctx, "UPDATE accounts SET seller_user_id = $1 WHERE id = $2", sellerB, accountID)
	require.NoError(t, err)
	entries, err := uc.ComputeCycle(ctx, "owner@test.com", "2026-06")
	require.NoError(t, err)
	require.Len(t, entries, 1, "reallocation must not mint a full pending row for the new seller")
	assert.Equal(t, sellerA, entries[0].SellerUserID)
	assert.Equal(t, domain.CommissionStatusPaid, entries[0].Status)
}

func TestE2E_CommissionRetentionBonusDedupAcrossSellers(t *testing.T) {
	cleanDB(t)
	accountID, _, sellerA := setupSLAFixture(t, "Cliente", "us004-bonus@test.com")
	registerUser(t, "Vendedor B", "us004-bonusb@test.com", "pass1234")
	var sellerB string
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT id FROM users WHERE email = $1", "us004-bonusb@test.com").Scan(&sellerB))

	ctx := context.Background()
	// Account is 12+ months old with an active subscription (retention
	// bonus conditions) and already earned the bonus under seller A.
	_, err := testPool.Exec(ctx, `
		UPDATE accounts SET seller_user_id = $1, lead_source = 'inbound',
		       subscription_status = 'active', created_at = '2025-06-15'
		WHERE id = $2`, sellerB, accountID)
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, `
		INSERT INTO commission_ledger (id, seller_user_id, account_id, cycle, rate_applied, amount_cents, reason, status)
		VALUES ($1, $2, $3, '2026-06', 0, 10000, 'bonus_retention', 'pending')
	`, ulid.Make().String(), sellerA, accountID)
	require.NoError(t, err)

	repo := postgres.NewCommissionRepository(testPool)
	uc := usecases.NewComputeCommissionsUseCase(repo,
		postgres.NewUserRepository(testPool, nil), postgres.NewAuditLogRepository(testPool))

	seedProfit(t, accountID, "2026-06", 100000)
	entries, err := uc.ComputeCycle(ctx, "owner@test.com", "2026-06")
	require.NoError(t, err)

	retention := 0
	for _, e := range entries {
		if e.Reason == domain.CommissionReasonBonusRetention {
			retention++
			assert.Equal(t, sellerA, e.SellerUserID, "the original grant must be the one that survives")
		}
	}
	assert.Equal(t, 1, retention, "seller reallocation must not duplicate the retention bonus")
}
