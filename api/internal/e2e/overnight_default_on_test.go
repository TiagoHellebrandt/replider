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

// Overnight drafting is ON by default since 2026-08-19: an account that never
// opened the setting gets its queue filled at night, because the promise is
// "open the app in the morning to work already done". These pin the three
// halves of that flip at the repository level: the rowless account is listed
// with the defaults, the first claim materializes its row, and an explicit
// enabled=false is respected forever.

func seedOvernightLead(t *testing.T, accountID string) {
	t.Helper()
	funnelID := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO sales_funnels (id, account_id, name, active, created_at, updated_at)
		 VALUES ($1, $2, 'Clientes', TRUE, NOW(), NOW())`, funnelID, accountID)
	require.NoError(t, err)
	contactID := seedSaleContact(t, accountID)
	_, err = testPool.Exec(context.Background(),
		`INSERT INTO leads (id, account_id, funnel_id, contact_id, status, source,
		                    collected_metadata, enrichment, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'new', 'manual', '{}', '{}', NOW(), NOW())`,
		ulid.Make().String(), accountID, funnelID, contactID)
	require.NoError(t, err)
}

func TestOvernightDefaultOn_ARowlessAccountWithLeadsIsDue(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewOvernightDraftRepository(testPool)

	withLeads := seedIdentityAccount(t)
	seedOvernightLead(t, withLeads)
	empty := seedIdentityAccount(t) // no leads: nothing to draft, not listed

	targets, err := repo.ListEnabled(ctx)
	require.NoError(t, err)

	ids := map[string]bool{}
	for _, tg := range targets {
		ids[tg.Settings.AccountID] = true
		if tg.Settings.AccountID == withLeads {
			assert.True(t, tg.Settings.Enabled)
			assert.Equal(t, 40, tg.Settings.NightlyCap)
			assert.Equal(t, 3, tg.Settings.RunHour)
			assert.Nil(t, tg.Settings.LastRunOn, "never ran")
		}
	}
	assert.True(t, ids[withLeads], "an account with leads and no row is due by default")
	assert.False(t, ids[empty], "an empty account is not woken for nothing")
}

func TestOvernightDefaultOn_TheFirstClaimMaterializesTheRow(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewOvernightDraftRepository(testPool)
	accountID := seedIdentityAccount(t)
	seedOvernightLead(t, accountID)
	night := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)

	claimed, err := repo.ClaimNight(ctx, accountID, night)
	require.NoError(t, err)
	assert.True(t, claimed, "the rowless account's first claim wins and inserts the row")

	// Same night again (a second pod): already claimed.
	claimed, err = repo.ClaimNight(ctx, accountID, night)
	require.NoError(t, err)
	assert.False(t, claimed)

	// The row now exists with the defaults and the stamp.
	s, err := repo.Settings(ctx, accountID)
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.True(t, s.Enabled)
	require.NotNil(t, s.LastRunOn)
}

func TestOvernightDefaultOn_AnExplicitOffStaysOff(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	repo := postgres.NewOvernightDraftRepository(testPool)
	accountID := seedIdentityAccount(t)
	seedOvernightLead(t, accountID)
	_, err := testPool.Exec(ctx,
		`INSERT INTO overnight_draft_settings (account_id, enabled) VALUES ($1, FALSE)`, accountID)
	require.NoError(t, err)

	targets, err := repo.ListEnabled(ctx)
	require.NoError(t, err)
	for _, tg := range targets {
		assert.NotEqual(t, accountID, tg.Settings.AccountID, "enabled=false is a decision")
	}

	claimed, err := repo.ClaimNight(ctx, accountID, time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.False(t, claimed, "a disabled row must never be claimed")
}
