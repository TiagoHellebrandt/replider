package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The candidates learned from a Sent folder. Against real Postgres because the
// accumulation is the SQL: the count, the date window, and the guarantee that a
// rescan never undoes a human's decision.

func TestDiscoveredContacts_AccumulatesSightingsInsteadOfResetting(t *testing.T) {
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	channelID := seedIMAPCursorChannel(t, accountID)
	repo := postgres.NewDiscoveredContactRepository(testPool)

	older := time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)

	// Pages arrive NEWEST first, which is exactly why the date window uses
	// LEAST/GREATEST — a plain assignment would leave first_sent_at holding the
	// most recent date and every candidate would look brand new.
	require.NoError(t, repo.Upsert(ctx, accountID, channelID, domain.SentRecipient{
		Address: "ana@clinicaaurora.com.br", SentAt: newer,
	}))
	require.NoError(t, repo.Upsert(ctx, accountID, channelID, domain.SentRecipient{
		Address: "ana@clinicaaurora.com.br", Name: "Ana Souza", SentAt: older,
	}))
	require.NoError(t, repo.Upsert(ctx, accountID, channelID, domain.SentRecipient{
		Address: "ana@clinicaaurora.com.br", SentAt: newer,
	}))

	pending, err := repo.ListPending(ctx, accountID, 50, 0)
	require.NoError(t, err)
	require.Len(t, pending, 1, "one person, three sightings")

	got := pending[0]
	assert.Equal(t, 3, got.SentCount)
	require.NotNil(t, got.FirstSentAt)
	require.NotNil(t, got.LastSentAt)
	assert.Equal(t, older.Unix(), got.FirstSentAt.Unix())
	assert.Equal(t, newer.Unix(), got.LastSentAt.Unix())
	// A name that arrived on a later sighting fills the blank one.
	assert.Equal(t, "Ana Souza", got.Name)
}

// The same person appears named in some messages and bare in others. A good name
// must not be overwritten by a blank one — the name is what a human recognises
// in the review list.
func TestDiscoveredContacts_ABlankNameNeverOverwritesAGoodOne(t *testing.T) {
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	channelID := seedIMAPCursorChannel(t, accountID)
	repo := postgres.NewDiscoveredContactRepository(testPool)

	require.NoError(t, repo.Upsert(ctx, accountID, channelID, domain.SentRecipient{
		Address: "ana@clinicaaurora.com.br", Name: "Ana Souza",
	}))
	require.NoError(t, repo.Upsert(ctx, accountID, channelID, domain.SentRecipient{
		Address: "ana@clinicaaurora.com.br",
	}))

	pending, err := repo.ListPending(ctx, accountID, 50, 0)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, "Ana Souza", pending[0].Name)
}

// Most-emailed first: 40 messages over two years is a customer, one is a
// stranger who asked a question, and the review list has to put them in that
// order or the human reads noise before signal.
func TestDiscoveredContacts_RanksByHowMuchSheWritesToThem(t *testing.T) {
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	channelID := seedIMAPCursorChannel(t, accountID)
	repo := postgres.NewDiscoveredContactRepository(testPool)

	for i := 0; i < 5; i++ {
		require.NoError(t, repo.Upsert(ctx, accountID, channelID, domain.SentRecipient{
			Address: "cliente@frequente.com.br",
		}))
	}
	require.NoError(t, repo.Upsert(ctx, accountID, channelID, domain.SentRecipient{
		Address: "alguem@umavez.com.br",
	}))

	pending, err := repo.ListPending(ctx, accountID, 50, 0)
	require.NoError(t, err)
	require.Len(t, pending, 2)
	assert.Equal(t, "cliente@frequente.com.br", pending[0].Address)
	assert.Equal(t, 5, pending[0].SentCount)
}

// The decision survives a rescan. Re-offering a name she already refused, or
// re-offering somebody already in the CRM, is how a review list becomes noise
// she stops reading.
func TestDiscoveredContacts_ARescanNeverUndoesADecision(t *testing.T) {
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	channelID := seedIMAPCursorChannel(t, accountID)
	repo := postgres.NewDiscoveredContactRepository(testPool)

	for _, addr := range []string{"importado@cliente.com.br", "recusado@ninguem.com.br"} {
		require.NoError(t, repo.Upsert(ctx, accountID, channelID, domain.SentRecipient{Address: addr}))
	}
	now := time.Now().UTC()
	require.NoError(t, repo.MarkImported(ctx, accountID, []string{"importado@cliente.com.br"}, now))
	require.NoError(t, repo.MarkDismissed(ctx, accountID, []string{"recusado@ninguem.com.br"}, now))

	// The rescan sees both again.
	for _, addr := range []string{"importado@cliente.com.br", "recusado@ninguem.com.br"} {
		require.NoError(t, repo.Upsert(ctx, accountID, channelID, domain.SentRecipient{Address: addr}))
	}

	pending, err := repo.ListPending(ctx, accountID, 50, 0)
	require.NoError(t, err)
	assert.Empty(t, pending, "a decided candidate stays decided")

	total, err := repo.CountPending(ctx, accountID)
	require.NoError(t, err)
	assert.Zero(t, total)
}

// The stamp is the answer to "when did this come in?", so a second click must
// not rewrite it.
func TestDiscoveredContacts_ADecisionStampIsWrittenOnce(t *testing.T) {
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	channelID := seedIMAPCursorChannel(t, accountID)
	repo := postgres.NewDiscoveredContactRepository(testPool)
	require.NoError(t, repo.Upsert(ctx, accountID, channelID,
		domain.SentRecipient{Address: "ana@clinicaaurora.com.br"}))

	first := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	later := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	require.NoError(t, repo.MarkImported(ctx, accountID, []string{"ana@clinicaaurora.com.br"}, first))
	require.NoError(t, repo.MarkImported(ctx, accountID, []string{"ana@clinicaaurora.com.br"}, later))

	got, err := repo.GetByAddresses(ctx, accountID, []string{"ana@clinicaaurora.com.br"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.NotNil(t, got[0].ImportedAt)
	assert.Equal(t, first.Unix(), got[0].ImportedAt.Unix())
}

// Naming another tenant's address must not reach their candidate: the whole
// import flow takes addresses from the client, and an unscoped lookup would let
// anyone import a stranger's customer list by guessing an email.
func TestDiscoveredContacts_AreScopedToTheirAccount(t *testing.T) {
	ctx := context.Background()
	mine := seedIdentityAccount(t)
	theirs := seedIdentityAccount(t)
	repo := postgres.NewDiscoveredContactRepository(testPool)

	require.NoError(t, repo.Upsert(ctx, theirs, seedIMAPCursorChannel(t, theirs),
		domain.SentRecipient{Address: "cliente@deoutro.com.br"}))

	got, err := repo.GetByAddresses(ctx, mine, []string{"cliente@deoutro.com.br"})
	require.NoError(t, err)
	assert.Empty(t, got)

	// And a decision written under the wrong account changes nothing.
	require.NoError(t, repo.MarkDismissed(ctx, mine, []string{"cliente@deoutro.com.br"}, time.Now().UTC()))
	pending, err := repo.ListPending(ctx, theirs, 50, 0)
	require.NoError(t, err)
	require.Len(t, pending, 1, "the other account's candidate is untouched")
}

// Uppercase in a header is the same person as lowercase. Two rows would split
// one customer's history and offer them twice.
func TestDiscoveredContacts_AddressesAreCaseInsensitive(t *testing.T) {
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	channelID := seedIMAPCursorChannel(t, accountID)
	repo := postgres.NewDiscoveredContactRepository(testPool)

	require.NoError(t, repo.Upsert(ctx, accountID, channelID,
		domain.SentRecipient{Address: "Ana@ClinicaAurora.com.br"}))
	require.NoError(t, repo.Upsert(ctx, accountID, channelID,
		domain.SentRecipient{Address: "ana@clinicaaurora.com.br"}))

	pending, err := repo.ListPending(ctx, accountID, 50, 0)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, 2, pending[0].SentCount)

	// And a selection typed with different case still resolves.
	got, err := repo.GetByAddresses(ctx, accountID, []string{"ANA@clinicaaurora.COM.BR"})
	require.NoError(t, err)
	assert.Len(t, got, 1)
}
