package e2e

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-M007 against a real Postgres. The domain tests pin the policy; this
// pins the one thing only the database can guarantee — that the daily
// counter survives concurrency.

func seedMailboxChannel(t *testing.T) (accountID, channelID string) {
	t.Helper()
	accountID = seedIdentityAccount(t)
	channelID = ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO channels (id, account_id, name, provider, webhook_token, is_active, credentials, created_at, updated_at)
		 VALUES ($1, $2, 'Comercial', 'email', $3, true, '{}', NOW(), NOW())`,
		channelID, accountID, ulid.Make().String())
	require.NoError(t, err)
	return accountID, channelID
}

func TestEmailMailbox_EnsureIsIdempotentAndKeepsGuardState(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewEmailMailboxRepository(testPool)
	accountID, channelID := seedMailboxChannel(t)

	first, err := repo.EnsureForChannel(ctx, accountID, channelID, "Comercial@Clinica.com.BR")
	require.NoError(t, err)
	assert.Equal(t, "comercial@clinica.com.br", first.Address, "address is normalized for the unique key")
	assert.Equal(t, "clinica.com.br", first.Domain)
	assert.Equal(t, domain.DefaultDailyCap, first.DailyCap)

	// Guard state accumulates...
	now := time.Now()
	require.NoError(t, repo.StartWarmup(ctx, accountID, first.Address, now.Add(-3*24*time.Hour)))
	require.NoError(t, repo.MarkDNSVerified(ctx, accountID, first.Address, now))
	_, err = repo.RecordSend(ctx, first.ID, now)
	require.NoError(t, err)

	// ...and re-creating the channel must NOT reset it: that would hand back
	// a fresh daily budget and restart a warmup, which is exactly how a
	// domain gets burned by someone clicking "reconnect".
	second, err := repo.EnsureForChannel(ctx, accountID, channelID, "comercial@clinica.com.br")
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID)
	assert.NotNil(t, second.WarmupStartedAt)
	assert.NotNil(t, second.DNSVerifiedAt)
	assert.Equal(t, 1, second.SentToday)
}

// The bug this prevents is real, not theoretical: two approvals a second
// apart both read `sent_today = 24`, both decide they are under the cap of
// 25, and both send.
func TestEmailMailbox_ConcurrentSendsCannotExceedTheCounter(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewEmailMailboxRepository(testPool)
	accountID, channelID := seedMailboxChannel(t)

	mb, err := repo.EnsureForChannel(ctx, accountID, channelID, "vendas@clinica.com.br")
	require.NoError(t, err)

	const concurrent = 20
	now := time.Now()
	var wg sync.WaitGroup
	counts := make([]int, concurrent)
	errs := make([]error, concurrent)
	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			counts[i], errs[i] = repo.RecordSend(ctx, mb.ID, now)
		}(i)
	}
	wg.Wait()

	seen := map[int]bool{}
	for i := 0; i < concurrent; i++ {
		require.NoError(t, errs[i])
		assert.False(t, seen[counts[i]], "counter value %d handed out twice", counts[i])
		seen[counts[i]] = true
	}

	reloaded, err := repo.GetByAddress(ctx, accountID, "vendas@clinica.com.br")
	require.NoError(t, err)
	assert.Equal(t, concurrent, reloaded.SentToday, "every send is counted exactly once")
}

// The counter is keyed by UTC date: a send recorded yesterday must not eat
// today's budget.
func TestEmailMailbox_CounterRollsOverAtUTCMidnight(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewEmailMailboxRepository(testPool)
	accountID, channelID := seedMailboxChannel(t)

	mb, err := repo.EnsureForChannel(ctx, accountID, channelID, "sdr@clinica.com.br")
	require.NoError(t, err)

	yesterday := time.Now().UTC().Add(-24 * time.Hour)
	for i := 0; i < 5; i++ {
		_, err := repo.RecordSend(ctx, mb.ID, yesterday)
		require.NoError(t, err)
	}

	count, err := repo.RecordSend(ctx, mb.ID, time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, 1, count, "a new UTC day starts from zero")
}

// StartWarmup must be idempotent: re-running it on a half-warmed mailbox
// would silently re-throttle the customer for another three weeks.
func TestEmailMailbox_StartWarmupDoesNotRestartAnOngoingRamp(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewEmailMailboxRepository(testPool)
	accountID, channelID := seedMailboxChannel(t)

	mb, err := repo.EnsureForChannel(ctx, accountID, channelID, "novo@clinica.com.br")
	require.NoError(t, err)

	original := time.Now().Add(-10 * 24 * time.Hour).Truncate(time.Second)
	require.NoError(t, repo.StartWarmup(ctx, accountID, mb.Address, original))
	require.NoError(t, repo.StartWarmup(ctx, accountID, mb.Address, time.Now()))

	reloaded, err := repo.GetByAddress(ctx, accountID, mb.Address)
	require.NoError(t, err)
	require.NotNil(t, reloaded.WarmupStartedAt)
	assert.WithinDuration(t, original, *reloaded.WarmupStartedAt, time.Second)
}

// A cap of 0 would stop all sending with no error anywhere; a cap of 5000
// would not raise throughput, it would burn the domain.
func TestEmailMailbox_DailyCapIsBounded(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewEmailMailboxRepository(testPool)
	accountID, channelID := seedMailboxChannel(t)

	mb, err := repo.EnsureForChannel(ctx, accountID, channelID, "cap@clinica.com.br")
	require.NoError(t, err)

	require.NoError(t, repo.SetDailyCap(ctx, accountID, mb.Address, 0))
	got, err := repo.GetByAddress(ctx, accountID, mb.Address)
	require.NoError(t, err)
	assert.Equal(t, 1, got.DailyCap)

	require.NoError(t, repo.SetDailyCap(ctx, accountID, mb.Address, 100000))
	got, err = repo.GetByAddress(ctx, accountID, mb.Address)
	require.NoError(t, err)
	assert.Equal(t, 200, got.DailyCap)
}

// Mailboxes are account-scoped: two customers can legitimately send from
// the same address (an agency and its client), and neither may see or
// consume the other's budget.
func TestEmailMailbox_IsAccountScoped(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewEmailMailboxRepository(testPool)

	accountA, channelA := seedMailboxChannel(t)
	accountB, channelB := seedMailboxChannel(t)

	a, err := repo.EnsureForChannel(ctx, accountA, channelA, "mesmo@dominio.com.br")
	require.NoError(t, err)
	b, err := repo.EnsureForChannel(ctx, accountB, channelB, "mesmo@dominio.com.br")
	require.NoError(t, err)
	assert.NotEqual(t, a.ID, b.ID)

	_, err = repo.RecordSend(ctx, a.ID, time.Now())
	require.NoError(t, err)

	otherAccount, err := repo.GetByAddress(ctx, accountB, "mesmo@dominio.com.br")
	require.NoError(t, err)
	assert.Zero(t, otherAccount.SentToday, "one account's sends must not consume another's budget")

	missing, err := repo.GetByAddress(ctx, accountA, "naoexiste@dominio.com.br")
	require.NoError(t, err)
	assert.Nil(t, missing)
}
