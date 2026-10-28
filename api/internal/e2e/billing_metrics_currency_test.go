package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
)

// US-018 (review fixes): the admin MRR aggregation must never add BRL
// and USD cents together. The old AggregateBillingMetrics SQL summed
// amount_cents across every currency, so a $199 USD account inflated
// "BRL" MRR by 19900 raw cents. The query now segregates per currency
// (empty legacy currency = BRL) and excludes unknown currencies.
func TestAggregateBillingMetrics_SegregatesCurrencies(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	repo := postgres.NewBillingEventRepository(testPool)
	occurred := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	seedEvent := func(amountCents int64, currency, eventType string) {
		accID := ulid.Make().String()
		_, err := testPool.Exec(ctx,
			`INSERT INTO accounts (id, name) VALUES ($1, $2)`, accID, "MRR Co "+currency)
		require.NoError(t, err)
		require.NoError(t, repo.Create(ctx, &domain.BillingEvent{
			ID:          ulid.Make().String(),
			AccountID:   accID,
			EventType:   eventType,
			Plan:        "Pro",
			AmountCents: amountCents,
			Currency:    currency,
			OccurredAt:  occurred,
		}))
	}

	seedEvent(19900, "brl", "subscription_created") // R$199
	seedEvent(10000, "", "subscription_created")    // legacy row, pre-currency = BRL
	seedEvent(19900, "usd", "subscription_created") // $199 — NOT 19900 BRL cents
	seedEvent(5000, "eur", "subscription_created")  // unsupported — excluded from both

	agg, err := repo.Aggregate(ctx, occurred.AddDate(0, 0, -30))
	require.NoError(t, err)

	require.Equal(t, int64(29900), agg.MRRBRLCents, "BRL slice = brl + legacy empty currency")
	require.Equal(t, int64(19900), agg.MRRUSDCents, "USD slice stays in USD cents")
	require.Equal(t, 4, agg.ActiveAccounts)
	require.Equal(t, 1, agg.OtherCurrencyAccounts, "unknown currency flagged, not mislabeled")
}
