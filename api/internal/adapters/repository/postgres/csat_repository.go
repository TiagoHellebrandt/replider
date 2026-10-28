package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

type CSATRepository struct {
	queries *Queries
}

func NewCSATRepository(db DBTX) *CSATRepository {
	return &CSATRepository{queries: New(db)}
}

func (r *CSATRepository) CreatePending(ctx context.Context, rating *domain.CSATRating) error {
	q := getQueries(ctx, r.queries)
	return q.CreatePendingCSAT(ctx, CreatePendingCSATParams{
		ID:             rating.ID,
		AccountID:      rating.AccountID,
		ConversationID: rating.ConversationID,
		ContactID:      rating.ContactID,
	})
}

func (r *CSATRepository) RecordAnswer(ctx context.Context, conversationID string, rating int, feedback *string) error {
	q := getQueries(ctx, r.queries)
	return q.RecordCSATAnswer(ctx, RecordCSATAnswerParams{
		ConversationID: conversationID,
		Rating:         int16(rating),
		Feedback:       feedback,
	})
}

func (r *CSATRepository) GetPending(ctx context.Context, conversationID string) (*domain.CSATRating, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetPendingCSATForConversation(ctx, conversationID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	rating := &domain.CSATRating{
		ID:             row.ID,
		AccountID:      row.AccountID,
		ConversationID: row.ConversationID,
		ContactID:      row.ContactID,
		Rating:         int(row.Rating),
	}
	if row.SentAt.Valid {
		t := row.SentAt.Time
		rating.SentAt = &t
	}
	return rating, nil
}

func (r *CSATRepository) Stats(ctx context.Context, accountID string) (*domain.CSATStats, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetCSATStats(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return &domain.CSATStats{
		AvgRating: row.AvgRating,
		Sent:      row.Sent,
		Answered:  row.Answered,
	}, nil
}
