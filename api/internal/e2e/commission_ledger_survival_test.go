package e2e

import (
	"context"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/usecases"
)

// US-008 (review fixes), SQL level: the commission ledger is settled
// money and audit trail — migration 000136 dropped the ON DELETE
// CASCADE FKs so a GDPR account deletion (or removing a seller user)
// can never erase paid commission history. Both tests FAIL on the old
// schema: the cascade silently deleted the rows.

// insertLedgerRow seeds one commission_ledger row directly.
func insertLedgerRow(t *testing.T, sellerID, accountID, cycle, reason, status string, amountCents int64) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(), `
		INSERT INTO commission_ledger (id, seller_user_id, account_id, cycle, rate_applied, amount_cents, reason, status, paid_at)
		VALUES ($1, $2, $3, $4, 20, $5, $6, $7, CASE WHEN $7 = 'paid' THEN NOW() ELSE NULL END)
	`, id, sellerID, accountID, cycle, amountCents, reason, status)
	require.NoError(t, err)
	return id
}

func countLedgerRows(t *testing.T, accountID string) int {
	t.Helper()
	var n int
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM commission_ledger WHERE account_id = $1", accountID).Scan(&n))
	return n
}

func TestE2E_CommissionLedgerSurvivesAccountDeletion(t *testing.T) {
	cleanDB(t)
	accountID, _, sellerID := setupSLAFixture(t, "Cliente", "us008-cascade@test.com")
	// The seller must live on a DIFFERENT account so deleting the
	// customer account doesn't also delete the seller user.
	registerUser(t, "Vendedor", "us008-cascade-seller@test.com", "pass1234")
	var platformSellerID string
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT id FROM users WHERE email = $1", "us008-cascade-seller@test.com").Scan(&platformSellerID))
	_ = sellerID

	insertLedgerRow(t, platformSellerID, accountID, "2026-06", "standard", "paid", 20000)
	insertLedgerRow(t, platformSellerID, accountID, "2026-06", "bonus_nps", "pending", 10000)
	require.Equal(t, 2, countLedgerRows(t, accountID))

	// GDPR-style hard delete of the customer account.
	_, err := testPool.Exec(context.Background(), "DELETE FROM accounts WHERE id = $1", accountID)
	require.NoError(t, err)

	assert.Equal(t, 2, countLedgerRows(t, accountID),
		"account deletion must not erase the commission trail (old CASCADE did)")

	// Deleting the seller USER must not erase it either.
	_, err = testPool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", platformSellerID)
	require.NoError(t, err)
	assert.Equal(t, 2, countLedgerRows(t, accountID),
		"seller user deletion must not erase the commission trail (old CASCADE did)")

	// The owner listing still returns the rows (LEFT JOINs degrade to
	// empty names instead of dropping settled money).
	repo := postgres.NewCommissionRepository(testPool)
	entries, err := repo.ListEntriesByCycle(context.Background(), "2026-06")
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

// US-008: unlinking a seller flags the account's PENDING rows for owner
// reconciliation (note visible in the ledger listing + accounting CSV)
// — it never settles or deletes them, and paid history stays untouched.
func TestE2E_SellerUnlinkFlagsOrphanedPending(t *testing.T) {
	cleanDB(t)
	accountID, _, _ := setupSLAFixture(t, "Cliente", "us008-unlink@test.com")
	registerUser(t, "Vendedor", "us008-unlink-seller@test.com", "pass1234")
	var sellerID string
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT id FROM users WHERE email = $1", "us008-unlink-seller@test.com").Scan(&sellerID))
	ctx := context.Background()
	_, err := testPool.Exec(ctx,
		"UPDATE accounts SET seller_user_id = $1, lead_source = 'inbound' WHERE id = $2", sellerID, accountID)
	require.NoError(t, err)

	pendingID := insertLedgerRow(t, sellerID, accountID, "2026-06", "standard", "pending", 20000)
	paidID := insertLedgerRow(t, sellerID, accountID, "2026-05", "standard", "paid", 15000)

	accountRepo := postgres.NewAccountRepository(testPool, nil)
	commissionRepo := postgres.NewCommissionRepository(testPool)
	userRepo := postgres.NewUserRepository(testPool, nil)
	uc := usecases.NewOwnerAdminUseCase(accountRepo, accountRepo, userRepo, nil, nil,
		postgres.NewAuditLogRepository(testPool))
	uc.SetCommissionFlagger(commissionRepo)

	// Empty seller email = unlink.
	_, err = uc.SetSeller(ctx, "owner@test.com", accountID, "", "")
	require.NoError(t, err)

	var status string
	var note *string
	require.NoError(t, testPool.QueryRow(ctx,
		"SELECT status, note FROM commission_ledger WHERE id = $1", pendingID).Scan(&status, &note))
	assert.Equal(t, "pending", status, "flagging must not settle the row")
	require.NotNil(t, note, "pending row must be flagged for reconciliation")
	assert.Contains(t, *note, "orphaned_by_unlink")

	require.NoError(t, testPool.QueryRow(ctx,
		"SELECT status, note FROM commission_ledger WHERE id = $1", paidID).Scan(&status, &note))
	assert.Equal(t, "paid", status)
	assert.Nil(t, note, "paid history must stay untouched")

	// Idempotent: a second flag pass doesn't stack the marker.
	n, err := commissionRepo.FlagOrphanedPending(ctx, accountID)
	require.NoError(t, err)
	assert.EqualValues(t, 0, n)

	// The account is actually unlinked.
	var sellerUserID *string
	require.NoError(t, testPool.QueryRow(ctx,
		"SELECT seller_user_id FROM accounts WHERE id = $1", accountID).Scan(&sellerUserID))
	assert.Nil(t, sellerUserID)
}
