package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/core/domain"
)

type WABillingMeterRepository struct {
	queries *Queries
}

func NewWABillingMeterRepository(db DBTX) *WABillingMeterRepository {
	return &WABillingMeterRepository{queries: New(db)}
}

func (r *WABillingMeterRepository) Aggregate(ctx context.Context, periodStart, periodEnd time.Time) error {
	q := getQueries(ctx, r.queries)
	return q.AggregateWAConversationsForMetering(ctx, AggregateWAConversationsForMeteringParams{
		PeriodStart: pgtype.Timestamptz{Time: periodStart, Valid: true},
		PeriodEnd:   pgtype.Timestamptz{Time: periodEnd, Valid: true},
	})
}

func (r *WABillingMeterRepository) ListPending(ctx context.Context, batchSize int) ([]*domain.WABillingMeterRollup, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListPendingMeterRollups(ctx, int32(batchSize))
	if err != nil {
		return nil, err
	}
	out := make([]*domain.WABillingMeterRollup, 0, len(rows))
	for _, raw := range rows {
		out = append(out, &domain.WABillingMeterRollup{
			AccountID:                  raw.AccountID,
			PricingCategory:            raw.PricingCategory,
			BillingPeriod:              raw.BillingPeriod.Time,
			BillableCount:              int(raw.BillableCount),
			TotalCount:                 int(raw.TotalCount),
			StripeMarketingItemID:      raw.StripeMarketingItemID,
			StripeUtilityItemID:        raw.StripeUtilityItemID,
			StripeAuthenticationItemID: raw.StripeAuthenticationItemID,
			StripeServiceItemID:        raw.StripeServiceItemID,
		})
	}
	return out, nil
}

func (r *WABillingMeterRepository) MarkPushed(ctx context.Context, accountID, pricingCategory string, period time.Time) (int64, error) {
	q := getQueries(ctx, r.queries)
	return q.MarkMeterRollupPushed(ctx, MarkMeterRollupPushedParams{
		AccountID:       accountID,
		PricingCategory: pricingCategory,
		BillingPeriod:   pgtype.Date{Time: period, Valid: true},
	})
}
