package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/replider/api/internal/core/domain"
)

// WAWebhookOutboxRepository persists raw WhatsApp webhook payloads
// before any side effects run. The async worker drains pending rows;
// the handler writes them inline so a crash mid-processing leaves a
// replayable trail in `wa_webhook_events`.
type WAWebhookOutboxRepository struct {
	queries *Queries
}

func NewWAWebhookOutboxRepository(db DBTX) *WAWebhookOutboxRepository {
	return &WAWebhookOutboxRepository{queries: New(db)}
}

func (r *WAWebhookOutboxRepository) Insert(ctx context.Context, id string, payload []byte, signature string) error {
	q := getQueries(ctx, r.queries)
	return q.InsertWAWebhookEvent(ctx, InsertWAWebhookEventParams{
		ID:        id,
		Payload:   payload,
		Signature: signature,
	})
}

func (r *WAWebhookOutboxRepository) MarkProcessed(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.MarkWAWebhookEventProcessed(ctx, id)
}

func (r *WAWebhookOutboxRepository) MarkSkipped(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.MarkWAWebhookEventSkipped(ctx, id)
}

func (r *WAWebhookOutboxRepository) MarkFailed(ctx context.Context, id string, errMsg string) error {
	q := getQueries(ctx, r.queries)
	return q.MarkWAWebhookEventFailed(ctx, MarkWAWebhookEventFailedParams{
		ID:    id,
		Error: &errMsg,
	})
}

func (r *WAWebhookOutboxRepository) MarkDead(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.MarkWAWebhookEventDead(ctx, id)
}

func (r *WAWebhookOutboxRepository) ListPending(ctx context.Context, limit int) ([]*domain.WAWebhookEvent, error) {
	q := getQueries(ctx, r.queries)
	// Multi-replica safe: the SQL flips claimed rows to
	// status='processing' inside a FOR UPDATE SKIP LOCKED block so
	// two worker replicas split work cleanly. The worker writes the
	// terminal status (Processed/Skipped/Failed) on each row.
	rows, err := q.ListPendingWAWebhookEvents(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]*domain.WAWebhookEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, &domain.WAWebhookEvent{
			ID:        row.ID,
			Payload:   row.Payload,
			Signature: row.Signature,
			Attempts:  int(row.Attempts),
		})
	}
	return out, nil
}

// RecoverStale flips rows stuck in 'processing' for >staleMinutes
// back to 'pending' so a crashed replica's claims aren't lost.
func (r *WAWebhookOutboxRepository) RecoverStale(ctx context.Context, staleMinutes int) (int64, error) {
	q := getQueries(ctx, r.queries)
	return q.RecoverStaleProcessingWAWebhookEvents(ctx, int32(staleMinutes))
}

func (r *WAWebhookOutboxRepository) PruneProcessed(ctx context.Context, olderThan time.Time) (int64, error) {
	q := getQueries(ctx, r.queries)
	return q.PruneProcessedWAWebhookEvents(ctx, pgtype.Timestamptz{Time: olderThan, Valid: true})
}

func (r *WAWebhookOutboxRepository) ListByStatus(ctx context.Context, status string, limit int) ([]*domain.WAWebhookEventDigest, error) {
	q := getQueries(ctx, r.queries)
	limit32 := int32(limit)
	if status == "" || status == "all" {
		rows, err := q.ListAllWAWebhookEvents(ctx, limit32)
		if err != nil {
			return nil, err
		}
		out := make([]*domain.WAWebhookEventDigest, 0, len(rows))
		for _, row := range rows {
			out = append(out, mapWebhookEventDigest(row.ID, row.Status, row.Attempts, row.ReceivedAt, row.ProcessedAt, row.LastAttemptAt, row.Error))
		}
		return out, nil
	}
	rows, err := q.ListWAWebhookEventsByStatus(ctx, ListWAWebhookEventsByStatusParams{
		Status: status,
		Limit:  limit32,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.WAWebhookEventDigest, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapWebhookEventDigest(row.ID, row.Status, row.Attempts, row.ReceivedAt, row.ProcessedAt, row.LastAttemptAt, row.Error))
	}
	return out, nil
}

func (r *WAWebhookOutboxRepository) ResetForRetry(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.ResetWAWebhookEventForRetry(ctx, id)
}

// mapWebhookEventDigest centralises the pgtype → domain conversion so
// the two list paths above stay in sync (a single status filter
// difference is the only thing that varies between them).
func mapWebhookEventDigest(
	id, status string, attempts int32,
	receivedAt pgtype.Timestamptz,
	processedAt pgtype.Timestamptz,
	lastAttemptAt pgtype.Timestamptz,
	errMsg *string,
) *domain.WAWebhookEventDigest {
	d := &domain.WAWebhookEventDigest{
		ID:       id,
		Status:   status,
		Attempts: int(attempts),
	}
	if receivedAt.Valid {
		d.ReceivedAt = receivedAt.Time
	}
	if processedAt.Valid {
		t := processedAt.Time
		d.ProcessedAt = &t
	}
	if lastAttemptAt.Valid {
		t := lastAttemptAt.Time
		d.LastAttemptAt = &t
	}
	if errMsg != nil {
		d.Error = *errMsg
	}
	return d
}
