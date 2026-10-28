package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
)

// US-022 (review fixes): channel-scoped co-pilot dismissals + the
// carteira query carrying the engine columns (N+1 fix). DB-real: pins
// migration 000141 (channel_id in the unique key) and the extended
// ListCarteiraAccounts SQL.

func TestE2E_SuggestionStateUpsertIsChannelScoped(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Plataforma", "sugg-seller@test.com", "pass1234")
	registerUser(t, "Cliente", "sugg-client@test.com", "pass1234")
	ctx := context.Background()

	var sellerID, clientAccountID string
	require.NoError(t, testPool.QueryRow(ctx,
		"SELECT id FROM users WHERE email = $1", "sugg-seller@test.com").Scan(&sellerID))
	require.NoError(t, testPool.QueryRow(ctx,
		"SELECT account_id FROM users WHERE email = $1", "sugg-client@test.com").Scan(&clientAccountID))

	repo := postgres.NewSellerSuggestionRepository(testPool)

	// Same (seller, account, type, cycle) under two DIFFERENT channels →
	// two independent rows. The pre-000141 unique key would collapse the
	// second upsert into an overwrite of the first.
	require.NoError(t, repo.UpsertSuggestionState(ctx, sellerID, clientAccountID, "whatsapp_ban_risk", "ch-1", "2026-07", "dismissed"))
	require.NoError(t, repo.UpsertSuggestionState(ctx, sellerID, clientAccountID, "whatsapp_ban_risk", "ch-2", "2026-07", "dismissed"))
	// Same channel again → overwrite, not a third row.
	require.NoError(t, repo.UpsertSuggestionState(ctx, sellerID, clientAccountID, "whatsapp_ban_risk", "ch-1", "2026-07", "done"))
	// Account-level type keeps the "" channel slot.
	require.NoError(t, repo.UpsertSuggestionState(ctx, sellerID, clientAccountID, "add_knowledge", "", "2026-07", "dismissed"))

	states, err := repo.ListSuggestionStates(ctx, sellerID, "2026-07")
	require.NoError(t, err)
	require.Len(t, states, 3)
	byChannel := map[string]string{}
	for _, s := range states {
		if s.SuggestionType == "whatsapp_ban_risk" {
			byChannel[s.ChannelID] = s.Status
		}
	}
	assert.Equal(t, map[string]string{"ch-1": "done", "ch-2": "dismissed"}, byChannel,
		"one dismissal row per channel; re-upsert overwrites in place")
}

func TestE2E_CarteiraCarriesEngineColumns(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Plataforma", "cart-seller@test.com", "pass1234")
	registerUser(t, "Cliente", "cart-client@test.com", "pass1234")
	ctx := context.Background()

	var sellerID, clientAccountID string
	require.NoError(t, testPool.QueryRow(ctx,
		"SELECT id FROM users WHERE email = $1", "cart-seller@test.com").Scan(&sellerID))
	require.NoError(t, testPool.QueryRow(ctx,
		"SELECT account_id FROM users WHERE email = $1", "cart-client@test.com").Scan(&clientAccountID))

	var planID string
	require.NoError(t, testPool.QueryRow(ctx,
		"SELECT id FROM plans WHERE stripe_price_id IS NOT NULL LIMIT 1").Scan(&planID))

	override := 500
	_, err := testPool.Exec(ctx, `
		UPDATE accounts SET seller_user_id = $1, plan_id = $2,
		       max_conversations_per_month_override = $3
		 WHERE id = $4`, sellerID, planID, override, clientAccountID)
	require.NoError(t, err)

	// Registration already seeded the trial usage row — bump its
	// counters, then refresh the account_usage_summary materialized view
	// the carteira query reads.
	_, err = testPool.Exec(ctx, `
		UPDATE account_usages SET conversations_count = 42, ai_tokens_used = 12345, addon_tokens = 100
		 WHERE account_id = $1`, clientAccountID)
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, "REFRESH MATERIALIZED VIEW account_usage_summary")
	require.NoError(t, err)

	repo := postgres.NewCommissionRepository(testPool)
	accounts, err := repo.ListCarteiraAccounts(ctx, sellerID, "2026-06")
	require.NoError(t, err)
	require.Len(t, accounts, 1)

	acc := accounts[0]
	assert.Equal(t, clientAccountID, acc.AccountID)
	assert.False(t, acc.CreatedAt.IsZero(), "created_at must ride on the carteira row")
	require.NotNil(t, acc.PlanID)
	assert.Equal(t, planID, *acc.PlanID)
	require.NotNil(t, acc.MaxConversationsPerMonthOverride)
	assert.Equal(t, override, *acc.MaxConversationsPerMonthOverride)
	require.NotNil(t, acc.UsageConversations)
	assert.EqualValues(t, 42, *acc.UsageConversations)
	require.NotNil(t, acc.UsageAITokens)
	assert.EqualValues(t, 12345, *acc.UsageAITokens)
	require.NotNil(t, acc.UsageAddonTokens)
	assert.EqualValues(t, 100, *acc.UsageAddonTokens)

	// No usage row (fresh view) → nil pointers, engine treats as zero.
	_, err = testPool.Exec(ctx, "DELETE FROM account_usages WHERE account_id = $1", clientAccountID)
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, "REFRESH MATERIALIZED VIEW account_usage_summary")
	require.NoError(t, err)
	accounts, err = repo.ListCarteiraAccounts(ctx, sellerID, "2026-06")
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	assert.Nil(t, accounts[0].UsageConversations)
}
