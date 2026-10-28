package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
)

// US-014 (review fixes): being a payable seller requires the EXPLICIT
// seller_settings.is_seller flag. Before this, ANY platform-account user
// with a minted ref code attributed signups to themselves — every
// attendant was silently payable.

// TestSellerRefResolve_RequiresExplicitFlag FAILS on the pre-US-014 code:
// ResolveSellerRef resolved any active platform user's code with no flag
// required.
func TestSellerRefResolve_RequiresExplicitFlag(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	registerUser(t, "Replider Platform", "platform-admin@replider.com", "Password!123")
	var platformAccID, adminUserID string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT account_id, id FROM users WHERE email = $1`,
		"platform-admin@replider.com").Scan(&platformAccID, &adminUserID))

	repo := postgres.NewSellerRefRepository(testPool)

	// Mint a code for a user WITHOUT the seller flag.
	code, err := repo.GetOrCreateSellerRefCode(ctx, adminUserID)
	require.NoError(t, err)
	require.NotEmpty(t, code)

	// Not flagged → the code must NOT resolve (no signup attribution).
	gotID, gotEmail, err := repo.ResolveSellerRef(ctx, code, platformAccID)
	require.NoError(t, err)
	assert.Empty(t, gotID, "unflagged user's code resolved — every platform user would be payable")
	assert.Empty(t, gotEmail)

	isSeller, err := repo.IsSeller(ctx, adminUserID)
	require.NoError(t, err)
	assert.False(t, isSeller, "user without seller_settings row reported as seller")

	// Owner grants the flag → same code now resolves.
	commissionRepo := postgres.NewCommissionRepository(testPool)
	require.NoError(t, commissionRepo.SetSellerFlag(ctx, adminUserID, true))

	gotID, gotEmail, err = repo.ResolveSellerRef(ctx, code, platformAccID)
	require.NoError(t, err)
	assert.Equal(t, adminUserID, gotID)
	assert.Equal(t, "platform-admin@replider.com", gotEmail)

	isSeller, err = repo.IsSeller(ctx, adminUserID)
	require.NoError(t, err)
	assert.True(t, isSeller)

	// Revoking flips it back off — a revoked seller's leftover code stops
	// attributing.
	require.NoError(t, commissionRepo.SetSellerFlag(ctx, adminUserID, false))
	gotID, _, err = repo.ResolveSellerRef(ctx, code, platformAccID)
	require.NoError(t, err)
	assert.Empty(t, gotID)
}

// TestSellerLeadAccount_PaymentHistoryAndEmailLookup pins the two repo
// reads the convert-lead tie-down (US-014) depends on.
func TestSellerLeadAccount_PaymentHistoryAndEmailLookup(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	registerUser(t, "Cliente Trial", "trial-owner@cliente.com", "Password!123")
	var trialAccID string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT account_id FROM users WHERE email = $1`, "trial-owner@cliente.com").Scan(&trialAccID))

	repo := postgres.NewSellerRefRepository(testPool)

	// Fresh trial account: no payment history.
	acc, err := repo.GetLeadAccount(ctx, trialAccID)
	require.NoError(t, err)
	require.NotNil(t, acc)
	assert.False(t, acc.HasPaymentHistory, "fresh trial account must have no payment history")

	// Give it a subscription → payment history flips on.
	_, err = testPool.Exec(ctx,
		`UPDATE accounts SET stripe_subscription_id = 'sub_test_123', subscription_status = 'active' WHERE id = $1`,
		trialAccID)
	require.NoError(t, err)

	acc, err = repo.GetLeadAccount(ctx, trialAccID)
	require.NoError(t, err)
	require.NotNil(t, acc)
	assert.True(t, acc.HasPaymentHistory)

	// Email relation lookup: exact user email matches case-insensitively,
	// foreign email doesn't.
	has, err := repo.AccountHasUserEmail(ctx, trialAccID, "TRIAL-OWNER@cliente.com")
	require.NoError(t, err)
	assert.True(t, has)

	has, err = repo.AccountHasUserEmail(ctx, trialAccID, "outra@pessoa.com")
	require.NoError(t, err)
	assert.False(t, has)
}
