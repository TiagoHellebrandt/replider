package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/core/domain"
)

type OutboundScheduledRepository struct {
	queries *Queries
}

func NewOutboundScheduledRepository(db DBTX) *OutboundScheduledRepository {
	return &OutboundScheduledRepository{queries: New(db)}
}

func (r *OutboundScheduledRepository) Create(ctx context.Context, row *domain.OutboundScheduled) error {
	q := getQueries(ctx, r.queries)
	var convID *string
	if row.ConversationID != nil {
		convID = row.ConversationID
	}
	return q.CreateOutboundScheduled(ctx, CreateOutboundScheduledParams{
		ID:             row.ID,
		AccountID:      row.AccountID,
		ChannelID:      row.ChannelID,
		ConversationID: convID,
		Payload:        row.Payload,
		Recipient:      row.Recipient,
		SendAt:         pgtype.Timestamptz{Time: row.SendAt, Valid: true},
	})
}

func (r *OutboundScheduledRepository) ListDue(ctx context.Context, batchSize int) ([]*domain.OutboundScheduled, error) {
	q := getQueries(ctx, r.queries)
	// ClaimDueOutboundScheduled flips status pending → processing
	// inside a FOR UPDATE SKIP LOCKED transaction so multi-replica
	// dispatchers split work cleanly. The dispatcher then writes the
	// terminal status (sent / failed) on each row.
	rows, err := q.ClaimDueOutboundScheduled(ctx, int32(batchSize))
	if err != nil {
		return nil, err
	}
	out := make([]*domain.OutboundScheduled, 0, len(rows))
	for _, raw := range rows {
		out = append(out, mapOutboundScheduled(raw))
	}
	return out, nil
}

// RecoverStale flips processing rows older than staleMinutes back
// to pending so a crashed replica's claims aren't lost forever. A
// separate ticker calls this; the dispatcher itself stays simple.
func (r *OutboundScheduledRepository) RecoverStale(ctx context.Context, staleMinutes int) (int64, error) {
	q := getQueries(ctx, r.queries)
	return q.RecoverStaleProcessingScheduled(ctx, int32(staleMinutes))
}

func (r *OutboundScheduledRepository) MarkSent(ctx context.Context, id, messageID string) error {
	q := getQueries(ctx, r.queries)
	return q.MarkOutboundScheduledSent(ctx, MarkOutboundScheduledSentParams{
		ID:        id,
		MessageID: messageID,
	})
}

func (r *OutboundScheduledRepository) MarkFailed(ctx context.Context, id, reason string) error {
	q := getQueries(ctx, r.queries)
	return q.MarkOutboundScheduledFailed(ctx, MarkOutboundScheduledFailedParams{
		ID:     id,
		Reason: reason,
	})
}

func (r *OutboundScheduledRepository) TouchAttempt(ctx context.Context, id, reason string) error {
	q := getQueries(ctx, r.queries)
	return q.TouchOutboundScheduledAttempt(ctx, TouchOutboundScheduledAttemptParams{
		ID:     id,
		Reason: reason,
	})
}

func (r *OutboundScheduledRepository) Cancel(ctx context.Context, id, accountID string) error {
	q := getQueries(ctx, r.queries)
	return q.CancelOutboundScheduled(ctx, CancelOutboundScheduledParams{
		ID:        id,
		AccountID: accountID,
	})
}

func (r *OutboundScheduledRepository) ListByAccount(ctx context.Context, accountID string, limit, offset int) ([]*domain.OutboundScheduled, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListAccountScheduledOutbound(ctx, ListAccountScheduledOutboundParams{
		AccountID: accountID,
		LimitN:    int32(limit),
		OffsetN:   int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.OutboundScheduled, 0, len(rows))
	for _, raw := range rows {
		out = append(out, &domain.OutboundScheduled{
			ID:             raw.ID,
			AccountID:      raw.AccountID,
			ChannelID:      raw.ChannelID,
			ConversationID: raw.ConversationID,
			Recipient:      raw.Recipient,
			SendAt:         raw.SendAt.Time,
			Status:         raw.Status,
			Attempts:       int(raw.Attempts),
			LastError:      raw.LastError,
			CreatedAt:      raw.CreatedAt.Time,
		})
	}
	return out, nil
}

func mapOutboundScheduled(raw OutboundScheduled) *domain.OutboundScheduled {
	out := &domain.OutboundScheduled{
		ID:             raw.ID,
		AccountID:      raw.AccountID,
		ChannelID:      raw.ChannelID,
		ConversationID: raw.ConversationID,
		Recipient:      raw.Recipient,
		Payload:        raw.Payload,
		SendAt:         raw.SendAt.Time,
		Status:         raw.Status,
		Attempts:       int(raw.Attempts),
		LastError:      raw.LastError,
		SentMessageID:  raw.SentMessageID,
		CreatedAt:      raw.CreatedAt.Time,
		UpdatedAt:      raw.UpdatedAt.Time,
	}
	return out
}
