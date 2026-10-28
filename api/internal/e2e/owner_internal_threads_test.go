package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/stretchr/testify/require"
)

// Canal-interno US-010 (owner panel): CountActiveInternalThreads counts
// only OPEN conversations stamped with the customer account — a closed
// thread and another account's thread don't leak into the figure.
func TestE2E_OwnerCountActiveInternalThreads(t *testing.T) {
	cleanDB(t)
	platformID, channelID, attendant := setupSLAFixture(t, "Plataforma", "owner-threads@test.com")

	registerUser(t, "Cliente A", "threads-a@test.com", "pass1234")
	registerUser(t, "Cliente B", "threads-b@test.com", "pass1234")
	var custA, custB string
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT account_id FROM users WHERE email = $1", "threads-a@test.com").Scan(&custA))
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT account_id FROM users WHERE email = $1", "threads-b@test.com").Scan(&custB))

	createdAt := time.Date(2026, 6, 2, 14, 0, 0, 0, time.UTC)
	stamp := func(convID, customerAccountID, status string) {
		_, err := testPool.Exec(context.Background(),
			"UPDATE conversations SET customer_account_id = $1, status = $2 WHERE id = $3",
			customerAccountID, status, convID)
		require.NoError(t, err)
	}
	// Account A: two open threads + one closed.
	stamp(seedSLAConversation(t, platformID, channelID, attendant, createdAt, nil), custA, "open")
	stamp(seedSLAConversation(t, platformID, channelID, attendant, createdAt, nil), custA, "open")
	stamp(seedSLAConversation(t, platformID, channelID, attendant, createdAt, nil), custA, "closed")
	// Account B: one open thread — must not count for A.
	stamp(seedSLAConversation(t, platformID, channelID, attendant, createdAt, nil), custB, "open")
	// Unstamped platform conversation (regular inbox) — never counted.
	seedSLAConversation(t, platformID, channelID, attendant, createdAt, nil)

	repo := postgres.NewAccountRepository(testPool, nil)
	n, err := repo.CountActiveInternalThreads(context.Background(), custA)
	require.NoError(t, err)
	require.EqualValues(t, 2, n)

	n, err = repo.CountActiveInternalThreads(context.Background(), custB)
	require.NoError(t, err)
	require.EqualValues(t, 1, n)

	// An account with no threads reads 0, not an error.
	n, err = repo.CountActiveInternalThreads(context.Background(), ulid.Make().String())
	require.NoError(t, err)
	require.EqualValues(t, 0, n)
}
