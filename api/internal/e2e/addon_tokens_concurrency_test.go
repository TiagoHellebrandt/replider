package e2e

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/stretchr/testify/require"
)

// TestE2E_AddAddonTokens_ConcurrentSumIsCorrect verifies US-012 against a
// real Postgres: N goroutines each crediting addon tokens to the same
// account must sum exactly, with no lost updates. The atomic
// `SET addon_tokens = addon_tokens + $2` UPDATE serializes concurrent
// writers at the row-lock level — a naive read-modify-write in app code
// would drop increments under this load.
func TestE2E_AddAddonTokens_ConcurrentSumIsCorrect(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	accountID := ulid.Make().String()
	_, err := testPool.Exec(ctx,
		`INSERT INTO accounts (id, name) VALUES ($1, $2)`, accountID, "Concurrency Test Co")
	require.NoError(t, err)

	usageID := ulid.Make().String()
	_, err = testPool.Exec(ctx,
		`INSERT INTO account_usages (id, account_id, billing_cycle_start, addon_tokens)
		 VALUES ($1, $2, $3, 0)`, usageID, accountID, time.Now())
	require.NoError(t, err)

	repo := postgres.NewUsageRepository(testPool)

	const goroutines = 50
	const perCall = int64(1000)
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := repo.AddAddonTokens(ctx, accountID, perCall); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}

	var total int64
	err = testPool.QueryRow(ctx,
		`SELECT addon_tokens FROM account_usages WHERE account_id = $1`, accountID).Scan(&total)
	require.NoError(t, err)

	require.Equal(t, int64(goroutines)*perCall, total,
		"concurrent addon credits lost an increment: got %d, want %d", total, int64(goroutines)*perCall)
}
