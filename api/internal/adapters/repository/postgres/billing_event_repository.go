package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/core/domain"
)

type BillingEventRepository struct {
	queries *Queries
}

func NewBillingEventRepository(db DBTX) *BillingEventRepository {
	return &BillingEventRepository{queries: New(db)}
}

func (r *BillingEventRepository) Create(ctx context.Context, e *domain.BillingEvent) error {
	q := getQueries(ctx, r.queries)
	return q.CreateBillingEvent(ctx, CreateBillingEventParams{
		ID:          e.ID,
		AccountID:   e.AccountID,
		EventType:   e.EventType,
		Plan:        e.Plan,
		AmountCents: e.AmountCents,
		Currency:    e.Currency,
		OccurredAt:  pgtype.Timestamptz{Time: e.OccurredAt, Valid: true},
	})
}

func (r *BillingEventRepository) Aggregate(ctx context.Context, since time.Time) (*domain.BillingMetricsAggregate, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.AggregateBillingMetrics(ctx, pgtype.Timestamptz{Time: since, Valid: true})
	if err != nil {
		return nil, err
	}
	return &domain.BillingMetricsAggregate{
		MRRBRLCents:           row.MrrBrlCents,
		MRRUSDCents:           row.MrrUsdCents,
		ActiveAccounts:        int(row.ActiveAccounts),
		OtherCurrencyAccounts: int(row.OtherCurrencyAccounts),
		ChurnedInWindow:       int(row.ChurnedWindow),
	}, nil
}
