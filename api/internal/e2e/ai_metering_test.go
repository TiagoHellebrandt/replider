package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-M015. The promise is "the provider's real price, no markup, on your
// invoice". That makes the arithmetic the feature: under-count and we
// silently eat cost, round wrong and the customer catches us.

func TestAIMetering_SumsEveryOperationAndRoundsOnce(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	// 550 = R$5.50 per USD, the platform default.
	repo := postgres.NewAICostRepository(testPool, 550)

	// Three entries whose USD sum has more precision than cents, so a
	// per-entry rounding would drift away from the correct total.
	for _, e := range []*domain.AICostEntry{
		{AccountID: accountID, Operation: "approach_draft", CostUSD: 0.001},
		{AccountID: accountID, Operation: "approach_draft", CostUSD: 0.001},
		{AccountID: accountID, Operation: "enrichment", CostUSD: 0.0004},
	} {
		require.NoError(t, repo.Record(ctx, e))
	}

	sum, err := repo.SumByAccount(ctx, accountID, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)

	assert.InDelta(t, 0.0024, sum.TotalUSD, 1e-9)
	// 0.0024 × 550 = 1.32 → 1 cent. Rounding each entry first would have
	// produced 3 × R$0.01 = 3 cents: a 3× overcharge on a "no markup" line.
	assert.Equal(t, int64(1), sum.TotalBRLCents)
	assert.Equal(t, 550, sum.RateCents)
	assert.InDelta(t, 0.002, sum.ByOperation["approach_draft"], 1e-9)
	assert.InDelta(t, 0.0004, sum.ByOperation["enrichment"], 1e-9)
}

func TestAIMetering_IsAccountAndPeriodScoped(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	mine := seedIdentityAccount(t)
	other := seedIdentityAccount(t)
	repo := postgres.NewAICostRepository(testPool, 550)

	now := time.Now()
	require.NoError(t, repo.Record(ctx, &domain.AICostEntry{
		AccountID: mine, Operation: "enrichment", CostUSD: 1, OccurredAt: now.Add(-2 * time.Hour)}))
	require.NoError(t, repo.Record(ctx, &domain.AICostEntry{
		AccountID: mine, Operation: "enrichment", CostUSD: 5, OccurredAt: now.Add(-72 * time.Hour)}))
	require.NoError(t, repo.Record(ctx, &domain.AICostEntry{
		AccountID: other, Operation: "enrichment", CostUSD: 99, OccurredAt: now.Add(-2 * time.Hour)}))

	sum, err := repo.SumByAccount(ctx, mine, now.Add(-24*time.Hour), now)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, sum.TotalUSD, 1e-9, "another account's spend and an older period must not leak in")
	assert.Equal(t, int64(550), sum.TotalBRLCents)
}

// A provider that reports no tokens is metered as unknown, never as free —
// the entry is skipped and logged rather than written as a zero that would
// read like a genuinely free call.
func TestAIMetering_NoUsageReportedIsNotChargedAsZero(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	repo := postgres.NewAICostRepository(testPool, 550)
	meter := usecases.NewAIMeter(repo)

	meter.Record(ctx, accountID, "approach_draft", "google", "gemini-3.1-flash-lite", domain.AIUsage{})

	sum, err := repo.SumByAccount(ctx, accountID, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Zero(t, sum.TotalUSD)
	assert.Empty(t, sum.ByOperation)
}

// Real tokens price through the shared rate table — the same function the
// per-message stamp uses, so a draft and a reply cost the same per token.
func TestAIMetering_PricesFromTheSharedRateTable(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	repo := postgres.NewAICostRepository(testPool, 550)

	usecases.NewAIMeter(repo).Record(ctx, accountID, "approach_draft",
		"google", "gemini-3.1-flash-lite", domain.AIUsage{PromptTokens: 1000, CompletionTokens: 500})

	sum, err := repo.SumByAccount(ctx, accountID, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Greater(t, sum.TotalUSD, 0.0, "a metered call with real tokens must cost something")
}

// A nil meter is a valid state: metering must not be able to break the
// operation it prices.
func TestAIMetering_NilMeterIsSafe(t *testing.T) {
	var meter *usecases.AIMeter
	assert.NotPanics(t, func() {
		meter.Record(context.Background(), "acc", "approach_draft", "google", "m", domain.AIUsage{PromptTokens: 10})
	})
}
