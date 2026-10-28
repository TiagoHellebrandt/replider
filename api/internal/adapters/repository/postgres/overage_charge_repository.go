package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// OverageChargeRepository is the durable journal backing the C2 fix.

type OverageChargeRepository struct {
	queries *Queries
}

func NewOverageChargeRepository(db DBTX) *OverageChargeRepository {
	return &OverageChargeRepository{queries: New(db)}
}

func (r *OverageChargeRepository) Create(ctx context.Context, c *domain.OverageCharge) error {
	q := getQueries(ctx, r.queries)
	return q.CreateOverageCharge(ctx, CreateOverageChargeParams{
		ID:                        c.ID,
		AccountID:                 c.AccountID,
		AmountCents:               c.AmountCents,
		RunningTotalCents:         c.RunningTotalCents,
		OverageConversationsCount: c.OverageConversationsCount,
		Capped:                    c.Capped,
	})
}

func (r *OverageChargeRepository) GetPending(ctx context.Context, accountID string) (*domain.OverageCharge, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetPendingOverageCharge(ctx, accountID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	charge := &domain.OverageCharge{
		ID:                        row.ID,
		AccountID:                 row.AccountID,
		AmountCents:               row.AmountCents,
		RunningTotalCents:         row.RunningTotalCents,
		OverageConversationsCount: row.OverageConversationsCount,
		Capped:                    row.Capped,
		Status:                    row.Status,
		CreatedAt:                 row.CreatedAt.Time,
	}
	if row.ChargedAt.Valid {
		t := row.ChargedAt.Time
		charge.ChargedAt = &t
	}
	return charge, nil
}

func (r *OverageChargeRepository) MarkCharged(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.MarkOverageChargeCharged(ctx, id)
}

func (r *OverageChargeRepository) MarkNeedsReview(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.MarkOverageChargeNeedsReview(ctx, id)
}

func (r *OverageChargeRepository) SumCharged(ctx context.Context, accountID string) (int32, error) {
	q := getQueries(ctx, r.queries)
	return q.SumChargedOverageCents(ctx, accountID)
}

func (r *OverageChargeRepository) ArchiveCharged(ctx context.Context, accountID string) error {
	q := getQueries(ctx, r.queries)
	return q.ArchiveChargedOverage(ctx, accountID)
}
