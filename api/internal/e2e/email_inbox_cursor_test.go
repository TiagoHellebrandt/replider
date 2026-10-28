package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The cursor is the only thing standing between "every reply arrives once" and
// either losing mail or re-delivering it. These run against real Postgres
// because the guarantees live in the SQL — GREATEST, the validity guard, and the
// ordering of the due query.

func seedIMAPCursorChannel(t *testing.T, accountID string) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO channels (id, account_id, provider, name, webhook_token, credentials, is_active, created_at, updated_at)
		 VALUES ($1, $2, 'email', 'E-mail', $3, '{}'::jsonb, true, NOW(), NOW())`,
		id, accountID, ulid.Make().String())
	require.NoError(t, err)
	return id
}

func TestEmailInboxCursor_EnsureIsIdempotentAndNeverResetsAPosition(t *testing.T) {
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	channelID := seedIMAPCursorChannel(t, accountID)
	repo := postgres.NewEmailInboxCursorRepository(testPool)

	first, err := repo.EnsureForChannel(ctx, accountID, channelID, "")
	require.NoError(t, err)
	assert.Equal(t, "INBOX", first.Mailbox)
	assert.False(t, first.Anchored())

	now := time.Now().UTC()
	require.NoError(t, repo.Advance(ctx, channelID, "INBOX", 42, 100, now))

	// Re-registering must not rewind: it happens on every channel edit, and a
	// reset would re-deliver everything the customer already read in the app.
	again, err := repo.EnsureForChannel(ctx, accountID, channelID, "INBOX")
	require.NoError(t, err)
	assert.Equal(t, uint32(42), again.UIDValidity)
	assert.Equal(t, uint32(100), again.LastUID)
}

// Two overlapping polls (a slow one finishing while the next tick starts) can
// write in either order. Without GREATEST the lower one wins and every message
// between them is delivered again.
func TestEmailInboxCursor_AdvanceNeverMovesBackwards(t *testing.T) {
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	channelID := seedIMAPCursorChannel(t, accountID)
	repo := postgres.NewEmailInboxCursorRepository(testPool)
	_, err := repo.EnsureForChannel(ctx, accountID, channelID, "INBOX")
	require.NoError(t, err)

	now := time.Now().UTC()
	require.NoError(t, repo.Advance(ctx, channelID, "INBOX", 42, 200, now))
	// The straggler reports an older position.
	require.NoError(t, repo.Advance(ctx, channelID, "INBOX", 42, 150, now))

	got, err := repo.EnsureForChannel(ctx, accountID, channelID, "INBOX")
	require.NoError(t, err)
	assert.Equal(t, uint32(200), got.LastUID)
}

// A poll that read a generation the row has since moved past cannot rewind it.
func TestEmailInboxCursor_AdvanceIgnoresAStaleGeneration(t *testing.T) {
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	channelID := seedIMAPCursorChannel(t, accountID)
	repo := postgres.NewEmailInboxCursorRepository(testPool)
	_, err := repo.EnsureForChannel(ctx, accountID, channelID, "INBOX")
	require.NoError(t, err)

	now := time.Now().UTC()
	require.NoError(t, repo.Reanchor(ctx, channelID, "INBOX", 77, 5, now))
	// An in-flight poll from before the rebuild reports the old generation.
	require.NoError(t, repo.Advance(ctx, channelID, "INBOX", 42, 9000, now))

	got, err := repo.EnsureForChannel(ctx, accountID, channelID, "INBOX")
	require.NoError(t, err)
	assert.Equal(t, uint32(77), got.UIDValidity)
	assert.Equal(t, uint32(5), got.LastUID, "a stale poll cannot move the cursor")
}

// Reanchor is the ONE operation allowed to move backwards, and only because the
// server said the UID space was rebuilt.
func TestEmailInboxCursor_ReanchorMayMoveBackwards(t *testing.T) {
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	channelID := seedIMAPCursorChannel(t, accountID)
	repo := postgres.NewEmailInboxCursorRepository(testPool)
	_, err := repo.EnsureForChannel(ctx, accountID, channelID, "INBOX")
	require.NoError(t, err)

	now := time.Now().UTC()
	require.NoError(t, repo.Advance(ctx, channelID, "INBOX", 42, 9000, now))
	require.NoError(t, repo.Reanchor(ctx, channelID, "INBOX", 43, 3, now))

	got, err := repo.EnsureForChannel(ctx, accountID, channelID, "INBOX")
	require.NoError(t, err)
	assert.Equal(t, uint32(43), got.UIDValidity)
	assert.Equal(t, uint32(3), got.LastUID)
}

// The reason is kept on the row so "why did nothing arrive?" is answerable
// without reading logs, and a success clears it — a stale error next to working
// polls would send somebody debugging a problem that is gone.
func TestEmailInboxCursor_RecordsAndThenClearsTheFailure(t *testing.T) {
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	channelID := seedIMAPCursorChannel(t, accountID)
	repo := postgres.NewEmailInboxCursorRepository(testPool)
	_, err := repo.EnsureForChannel(ctx, accountID, channelID, "INBOX")
	require.NoError(t, err)

	now := time.Now().UTC()
	require.NoError(t, repo.RecordFailure(ctx, channelID, "INBOX", now, "535 Authentication failed"))
	require.NoError(t, repo.RecordFailure(ctx, channelID, "INBOX", now, "535 Authentication failed"))

	got, err := repo.EnsureForChannel(ctx, accountID, channelID, "INBOX")
	require.NoError(t, err)
	assert.Equal(t, 2, got.FailureCount)
	assert.Contains(t, got.LastError, "535")
	require.NotNil(t, got.LastPolledAt)

	require.NoError(t, repo.Advance(ctx, channelID, "INBOX", 42, 1, now))
	healed, err := repo.EnsureForChannel(ctx, accountID, channelID, "INBOX")
	require.NoError(t, err)
	assert.Zero(t, healed.FailureCount)
	assert.Empty(t, healed.LastError)
}

// Never-polled inboxes come FIRST: they are the ones a customer just connected
// and is watching, and a freshly added inbox waiting behind an hour of others is
// the product looking broken on the first try.
func TestEmailInboxCursor_ListDuePutsNeverPolledFirst(t *testing.T) {
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	repo := postgres.NewEmailInboxCursorRepository(testPool)

	polled := seedIMAPCursorChannel(t, accountID)
	fresh := seedIMAPCursorChannel(t, accountID)
	_, err := repo.EnsureForChannel(ctx, accountID, polled, "INBOX")
	require.NoError(t, err)
	_, err = repo.EnsureForChannel(ctx, accountID, fresh, "INBOX")
	require.NoError(t, err)

	long := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, repo.Advance(ctx, polled, "INBOX", 42, 1, long))

	due, err := repo.ListDue(ctx, time.Now().UTC(), 50)
	require.NoError(t, err)

	var order []string
	for _, c := range due {
		if c.ChannelID == polled || c.ChannelID == fresh {
			order = append(order, c.ChannelID)
		}
	}
	require.Len(t, order, 2)
	assert.Equal(t, fresh, order[0], "a just-connected inbox is polled first")
}

// An inbox polled a moment ago is not due: the interval exists so a provider is
// not dialed every tick.
func TestEmailInboxCursor_ListDueSkipsFreshlyPolled(t *testing.T) {
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	channelID := seedIMAPCursorChannel(t, accountID)
	repo := postgres.NewEmailInboxCursorRepository(testPool)
	_, err := repo.EnsureForChannel(ctx, accountID, channelID, "INBOX")
	require.NoError(t, err)

	now := time.Now().UTC()
	require.NoError(t, repo.Advance(ctx, channelID, "INBOX", 42, 1, now))

	due, err := repo.ListDue(ctx, now.Add(-2*time.Minute), 50)
	require.NoError(t, err)
	for _, c := range due {
		assert.NotEqual(t, channelID, c.ChannelID)
	}
}
