package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/usecases"
)

// US-007 (review fixes): the profit computation's token cost must come
// from the per-message stamp (messages.provider_cost_usd) — immutable
// attribution. The old aggregation JOINed the mutable agents row
// (conversations.agent_id → agents.provider/model) and priced via
// provider_costs: deleting or repointing an agent zeroed a past
// cycle's cost retroactively, inflating net profit and commission.

// seedCostedConversation inserts a conversation bound to agentID plus
// AI messages carrying the per-message token/cost stamp inside the
// 2026-06 cycle. One extra legacy row has tokens but no stamped cost.
func seedCostedConversation(t *testing.T, accountID, channelID string, agentID *string) string {
	t.Helper()
	ctx := context.Background()
	convID := ulid.Make().String()
	contactPhone := "+55659" + convID[18:]
	createdAt := time.Date(2026, 6, 10, 14, 0, 0, 0, time.UTC)

	_, err := testPool.Exec(ctx, `
		INSERT INTO conversations (id, account_id, channel_id, agent_id, contact_id, contact_name, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'Cliente', 'open', $6)
	`, convID, accountID, channelID, agentID, contactPhone, createdAt)
	require.NoError(t, err)

	// Two stamped AI turns: $0.75 + $0.50 = $1.25.
	for i, cost := range []float64{0.75, 0.50} {
		_, err = testPool.Exec(ctx, `
			INSERT INTO messages (id, channel_id, conversation_id, sender, type, content, direction, created_at,
			                      input_tokens, output_tokens, provider_cost_usd)
			VALUES ($1, $2, $3, 'bot', 'text', 'resposta', 'outbound', $4, 10000, 2000, $5)
		`, ulid.Make().String(), channelID, convID, createdAt.Add(time.Duration(i+1)*time.Minute), cost)
		require.NoError(t, err)
	}

	// Legacy row: tokens but no stamped cost — priced at zero, surfaced
	// as unpriced.
	_, err = testPool.Exec(ctx, `
		INSERT INTO messages (id, channel_id, conversation_id, sender, type, content, direction, created_at,
		                      input_tokens, output_tokens)
		VALUES ($1, $2, $3, 'bot', 'text', 'antiga', 'outbound', $4, 500, 100)
	`, ulid.Make().String(), channelID, convID, createdAt.Add(10*time.Minute))
	require.NoError(t, err)

	return convID
}

func TestE2E_ProfitTokenCost_SurvivesAgentDeletion(t *testing.T) {
	cleanDB(t)
	accountID, channelID, _ := setupSLAFixture(t, "Plataforma", "profit-cost@test.com")
	ctx := context.Background()

	agentID := ulid.Make().String()
	_, err := testPool.Exec(ctx, `
		INSERT INTO agents (id, account_id, name, provider, model)
		VALUES ($1, $2, 'Bot', 'google', 'gemini-2.5-flash')
	`, agentID, accountID)
	require.NoError(t, err)
	seedCostedConversation(t, accountID, channelID, &agentID)

	repo := postgres.NewMonthlyProfitRepository(testPool)
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)

	before, err := repo.SumTokenCost(ctx, accountID, from, to)
	require.NoError(t, err)
	require.InDelta(t, 1.25, before.CostUSD, 1e-9)
	require.Equal(t, int64(500), before.UnpricedInputTokens)
	require.Equal(t, int64(100), before.UnpricedOutputTokens)

	// Delete the agent — conversations.agent_id goes NULL (ON DELETE
	// SET NULL). The old agents-JOIN aggregation now grouped these
	// tokens under ('', '') and priced them at zero; the stamped cost
	// must not move.
	_, err = testPool.Exec(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
	require.NoError(t, err)

	after, err := repo.SumTokenCost(ctx, accountID, from, to)
	require.NoError(t, err)
	require.InDelta(t, before.CostUSD, after.CostUSD, 1e-9,
		"deleting the agent must not rewrite the cycle's token cost")

	// End-to-end through the usecase: the recomputed row prices the
	// cycle at the stamped $1.25 → 125 USD cents × default rate 550 →
	// 687.5 → 688 BRL cents, agent long gone.
	settings := postgres.NewProviderCostRepository(testPool)
	uc := usecases.NewComputeMonthlyProfitUseCase(repo, settings, postgres.NewAccountRepository(testPool, nil), nil, nil)
	rows, err := uc.RecomputeCycle(ctx, "owner@test.com", "2026-06", accountID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(688), rows[0].TokenCostCents)
}
