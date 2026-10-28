package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// US-005 (review fixes), SQL level: a delta that appears AFTER the
// previous adjustment settled must land as a NEW pending adjustment.
// Before migration 000135 the table-level UNIQUE (seller, account,
// cycle, reason) made that insert collide with the PAID adjustment row
// and the pending-only upsert guard silently dropped the money.
func TestE2E_CommissionPaidAdjustmentDoesNotBlockNextDelta(t *testing.T) {
	cleanDB(t)
	accountID, _, sellerID := setupSLAFixture(t, "Cliente", "us005-adj@test.com")
	ctx := context.Background()
	_, err := testPool.Exec(ctx,
		"UPDATE accounts SET seller_user_id = $1, lead_source = 'inbound' WHERE id = $2", sellerID, accountID)
	require.NoError(t, err)

	repo := postgres.NewCommissionRepository(testPool)
	uc := usecases.NewComputeCommissionsUseCase(repo,
		postgres.NewUserRepository(testPool, nil), postgres.NewAuditLogRepository(testPool))
	// Commission audits land on the platform account (US-005) — the
	// fixture account doubles as it here so the write satisfies the FK.
	uc.SetPlatformAccountID(accountID)

	// Standard 20% of 100000 = 20000, settled early.
	seedProfit(t, accountID, "2026-06", 100000)
	_, err = uc.ComputeCycle(ctx, "owner@test.com", "2026-06")
	require.NoError(t, err)
	_, _, err = repo.MarkPayoutPaid(ctx, sellerID, "2026-06", time.Now(), nil)
	require.NoError(t, err)

	// First delta: due 30000 vs 20000 paid -> adjustment +10000, which
	// settles too (total paid 30000).
	seedProfit(t, accountID, "2026-06", 150000)
	_, err = uc.ComputeCycle(ctx, "owner@test.com", "2026-06")
	require.NoError(t, err)
	_, _, err = repo.MarkPayoutPaid(ctx, sellerID, "2026-06", time.Now(), nil)
	require.NoError(t, err)

	// Second delta: due 50000 vs 30000 paid. The INSERT of this pending
	// adjustment used to hit the UNIQUE against the PAID adjustment row
	// and vanish — the 000135 pending-only index lets it land.
	seedProfit(t, accountID, "2026-06", 250000)
	entries, err := uc.ComputeCycle(ctx, "owner@test.com", "2026-06")
	require.NoError(t, err)

	var paidAdj, pendingAdj []*domain.CommissionEntry
	for _, e := range entries {
		if e.Reason != domain.CommissionReasonAdjustment {
			continue
		}
		if e.Status == domain.CommissionStatusPaid {
			paidAdj = append(paidAdj, e)
		} else {
			pendingAdj = append(pendingAdj, e)
		}
	}
	require.Len(t, pendingAdj, 1, "the post-settlement delta must land as a new pending adjustment")
	assert.EqualValues(t, 20000, pendingAdj[0].AmountCents)
	require.Len(t, paidAdj, 1, "the settled adjustment is immutable history")
	assert.EqualValues(t, 10000, paidAdj[0].AmountCents)

	// SumPaidProfitShare sees the full settled total (standard + paid
	// adjustment) so the delta math stays anchored.
	total, paidSeller, hasPaid, err := repo.SumPaidProfitShare(ctx, accountID, "2026-06")
	require.NoError(t, err)
	assert.True(t, hasPaid)
	assert.EqualValues(t, 30000, total)
	assert.Equal(t, sellerID, paidSeller, "paid seller tracks the settled rows")

	// Idempotent: recomputing again rewrites the SAME pending adjustment
	// (pending-only upsert), no third adjustment row.
	entries, err = uc.ComputeCycle(ctx, "owner@test.com", "2026-06")
	require.NoError(t, err)
	adjustments := 0
	for _, e := range entries {
		if e.Reason == domain.CommissionReasonAdjustment {
			adjustments++
		}
	}
	assert.Equal(t, 2, adjustments, "1 paid + 1 pending adjustment, never more")

	// The audit trail carries the adjusted delta (US-005).
	var auditCount int
	require.NoError(t, testPool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE action = 'commission.adjustment'
		  AND (payload->>'delta_cents')::bigint = 20000
	`).Scan(&auditCount))
	assert.GreaterOrEqual(t, auditCount, 1, "adjustment audit entry must record the delta")
}
