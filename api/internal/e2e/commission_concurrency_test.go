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

// US-006 (review fixes): ComputeCycle and MarkPayoutPaid serialize on
// the cycle's transaction-scoped advisory lock. These tests pin the two
// possible commit orders deterministically: an external transaction
// holds the lock, the racing operation is proven to WAIT on it (a
// pg_locks waiter appears — the old lock-free code sails through and
// fails here), the "first committer"'s effect is applied inside the
// lock-holding transaction, and only then does the waiter proceed.
// Neither order may double-pay or leave settled money unaccounted.

const cycleLockSQL = `SELECT pg_advisory_xact_lock(hashtext('commission_cycle'), hashtext($1))`

// waitForAdvisoryWaiter blocks until some backend is queued on an
// advisory lock. done closing first means the operation never waited —
// the exact US-006 bug.
func waitForAdvisoryWaiter(t *testing.T, done <-chan struct{}) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-done:
			t.Fatal("operation completed without waiting on the cycle advisory lock (US-006: recompute/payout must serialize)")
		case <-deadline:
			t.Fatal("timed out waiting for the advisory-lock waiter to appear")
		default:
		}
		var waiters int
		require.NoError(t, testPool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM pg_locks WHERE locktype = 'advisory' AND NOT granted`).Scan(&waiters))
		if waiters > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Commit order 1: the payout settles and commits WHILE a recompute of
// the same cycle is already waiting on the lock. The recompute must
// then observe the paid money and record only the delta adjustment —
// never a second full pending profit-share row (double-pay).
func TestE2E_CommissionRecomputeWaitsForPayoutCommit(t *testing.T) {
	cleanDB(t)
	accountID, _, sellerID := setupSLAFixture(t, "Cliente", "us006-order1@test.com")
	ctx := context.Background()
	_, err := testPool.Exec(ctx,
		"UPDATE accounts SET seller_user_id = $1, lead_source = 'inbound' WHERE id = $2", sellerID, accountID)
	require.NoError(t, err)

	repo := postgres.NewCommissionRepository(testPool)
	uc := usecases.NewComputeCommissionsUseCase(repo,
		postgres.NewUserRepository(testPool, nil), postgres.NewAuditLogRepository(testPool))

	// Cycle computed: inbound 20% of 100000 = 20000 pending.
	seedProfit(t, accountID, "2026-06", 100000)
	_, err = uc.ComputeCycle(ctx, "owner@test.com", "2026-06")
	require.NoError(t, err)

	// Profit is recomputed UP; the racing recompute will price 30000.
	seedProfit(t, accountID, "2026-06", 150000)

	// tx1 plays the payout side and grabs the cycle lock first.
	tx1, err := testPool.Begin(ctx)
	require.NoError(t, err)
	defer tx1.Rollback(ctx) //nolint:errcheck
	_, err = tx1.Exec(ctx, cycleLockSQL, "2026-06")
	require.NoError(t, err)

	recomputeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	done := make(chan struct{})
	var entries []*domain.CommissionEntry
	var recomputeErr error
	go func() {
		defer close(done)
		entries, recomputeErr = uc.ComputeCycle(recomputeCtx, "owner@test.com", "2026-06")
	}()

	// The recompute must queue behind the lock BEFORE its
	// SumPaidProfitShare check — that ordering is the whole fix.
	waitForAdvisoryWaiter(t, done)

	// Payout commits first: settle the pending 20000 inside tx1.
	_, err = tx1.Exec(ctx,
		`UPDATE commission_ledger SET status = 'paid', paid_at = NOW(), updated_at = NOW()
		 WHERE seller_user_id = $1 AND cycle = '2026-06' AND status = 'pending'`, sellerID)
	require.NoError(t, err)
	require.NoError(t, tx1.Commit(ctx))

	<-done
	require.NoError(t, recomputeErr)

	// Serialized outcome: the paid 20000 is immutable history; the
	// recomputed 30000 lands as a +10000 pending adjustment. The
	// lock-free interleave would leave a paid 20000 AND a full pending
	// 30000 standard row — the same profit paid twice.
	var paidStandard, pendingAdjustment, other int
	for _, e := range entries {
		switch {
		case e.Reason == domain.CommissionReasonStandard && e.Status == domain.CommissionStatusPaid:
			paidStandard++
			assert.EqualValues(t, 20000, e.AmountCents)
		case e.Reason == domain.CommissionReasonAdjustment && e.Status == domain.CommissionStatusPending:
			pendingAdjustment++
			assert.EqualValues(t, 10000, e.AmountCents)
		default:
			other++
			t.Errorf("unexpected ledger row after serialized recompute: reason=%s status=%s amount=%d",
				e.Reason, e.Status, e.AmountCents)
		}
	}
	assert.Equal(t, 1, paidStandard)
	assert.Equal(t, 1, pendingAdjustment)
	assert.Zero(t, other)
}

// Commit order 2: a recompute holds the lock and rewrites the pending
// amount WHILE the payout is already waiting. The payout must settle
// the post-recompute total (revalidated from the live rows under the
// lock), never the stale snapshot the owner had on screen.
func TestE2E_CommissionPayoutWaitsForRecomputeCommit(t *testing.T) {
	cleanDB(t)
	accountID, _, sellerID := setupSLAFixture(t, "Cliente", "us006-order2@test.com")
	ctx := context.Background()
	_, err := testPool.Exec(ctx,
		"UPDATE accounts SET seller_user_id = $1, lead_source = 'inbound' WHERE id = $2", sellerID, accountID)
	require.NoError(t, err)

	repo := postgres.NewCommissionRepository(testPool)
	uc := usecases.NewComputeCommissionsUseCase(repo,
		postgres.NewUserRepository(testPool, nil), postgres.NewAuditLogRepository(testPool))

	// Owner's screen shows 20000 pending.
	seedProfit(t, accountID, "2026-06", 100000)
	_, err = uc.ComputeCycle(ctx, "owner@test.com", "2026-06")
	require.NoError(t, err)

	// tx1 plays the recompute side and grabs the cycle lock first.
	tx1, err := testPool.Begin(ctx)
	require.NoError(t, err)
	defer tx1.Rollback(ctx) //nolint:errcheck
	_, err = tx1.Exec(ctx, cycleLockSQL, "2026-06")
	require.NoError(t, err)

	payoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	done := make(chan struct{})
	var settledEntries, settledCents int64
	var payoutErr error
	go func() {
		defer close(done)
		settledEntries, settledCents, payoutErr = repo.MarkPayoutPaid(payoutCtx, sellerID, "2026-06", time.Now(), nil)
	}()

	waitForAdvisoryWaiter(t, done)

	// Recompute commits first: the pending row grows to 30000.
	_, err = tx1.Exec(ctx,
		`UPDATE commission_ledger SET amount_cents = 30000, base_net_profit_cents = 150000, updated_at = NOW()
		 WHERE seller_user_id = $1 AND cycle = '2026-06' AND status = 'pending'`, sellerID)
	require.NoError(t, err)
	require.NoError(t, tx1.Commit(ctx))

	<-done
	require.NoError(t, payoutErr)

	// The payout settled the LIVE amount, not the pre-recompute 20000
	// snapshot, and nothing is left half-paid.
	assert.EqualValues(t, 1, settledEntries)
	assert.EqualValues(t, 30000, settledCents, "payout must revalidate the pending total under the lock")

	var pending int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM commission_ledger WHERE seller_user_id = $1 AND status = 'pending'`,
		sellerID).Scan(&pending))
	assert.Zero(t, pending, "no orphan pending row may survive the serialized settle")
}
