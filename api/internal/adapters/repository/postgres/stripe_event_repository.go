package postgres

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

// --- StripeEventRepository ---

type StripeEventRepository struct {
	db DBTX
}

func NewStripeEventRepository(db DBTX) *StripeEventRepository {
	return &StripeEventRepository{db: db}
}

func (r *StripeEventRepository) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	var processed bool
	err := r.db.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM processed_stripe_events WHERE event_id = $1 AND processed_at IS NOT NULL)",
		eventID,
	).Scan(&processed)
	return processed, err
}

// MarkProcessed stamps processed_at, recording that the event's
// side-effects have committed. After this the reconciler will never
// re-run the event (its claim is no longer "stuck"). (US-011)
func (r *StripeEventRepository) MarkProcessed(ctx context.Context, eventID string, eventType string) error {
	_, err := r.db.Exec(ctx,
		"UPDATE processed_stripe_events SET processed_at = NOW() WHERE event_id = $1",
		eventID,
	)
	return err
}

// ClaimEvent atomically claims an event for processing (US-011). It
// returns true when THIS caller owns the claim and must run the
// side-effects; false when the event is already processed or freshly
// claimed by a concurrent worker.
//
// Background: Stripe delivers at-least-once and retries non-2xx, so two
// deliveries of the same event can race. The claim must be atomic to
// avoid double side-effects (e.g. AddAddonTokens twice). The earlier
// design INSERT-ed a single row that meant BOTH "claimed" and
// "processed" — correct for the race, but if the handler crashed after
// claiming and before finishing, the event was marked done forever and
// silently dropped.
//
// The fix splits claimed_at from processed_at. A fresh INSERT claims an
// unseen event. The ON CONFLICT DO UPDATE re-claims a STUCK row — one
// claimed >5m ago that never reached processed_at — so a crashed event
// is recoverable. A row already processed, or claimed within the last
// 5m, matches the WHERE on neither branch and yields rowsAffected=0, so
// the caller bails. Postgres serializes concurrent upserts on the row
// lock, guaranteeing exactly one winner.
func (r *StripeEventRepository) ClaimEvent(ctx context.Context, eventID string, eventType string, payload []byte) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`INSERT INTO processed_stripe_events (event_id, event_type, claimed_at, payload)
		 VALUES ($1, $2, NOW(), $3)
		 ON CONFLICT (event_id) DO UPDATE
		   SET claimed_at = NOW(), payload = EXCLUDED.payload
		   WHERE processed_stripe_events.processed_at IS NULL
		     AND processed_stripe_events.claimed_at < NOW() - INTERVAL '5 minutes'`,
		eventID, eventType, payload,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// ListStuckEvents returns events claimed but never finished
// (processed_at IS NULL) whose claim is older than 5 minutes — the
// crash-recovery work-list for the reconciler. The stored payload lets
// the reconciler replay the event without re-fetching from Stripe.
// Capped so a backlog can't blow up memory; the reconciler drains it
// across ticks. (US-011)
func (r *StripeEventRepository) ListStuckEvents(ctx context.Context) ([]domain.StripeEventRecord, error) {
	rows, err := r.db.Query(ctx,
		`SELECT event_id, event_type, payload
		 FROM processed_stripe_events
		 WHERE processed_at IS NULL
		   AND claimed_at < NOW() - INTERVAL '5 minutes'
		 ORDER BY claimed_at
		 LIMIT 100`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.StripeEventRecord
	for rows.Next() {
		var rec domain.StripeEventRecord
		if err := rows.Scan(&rec.EventID, &rec.EventType, &rec.Payload); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// CleanupOldEvents prunes processed-event rows past their useful life so
// the idempotency table doesn't grow unbounded (US-020). The rows only
// exist to dedup Stripe's at-least-once retries; Stripe stops retrying an
// event after ~72h, so a 7-day cutoff is a generous safety margin — once
// past it the row can never match a live retry. Backed by the
// created_at index. Returns the number of rows deleted.
func (r *StripeEventRepository) CleanupOldEvents(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx,
		"DELETE FROM processed_stripe_events WHERE created_at < NOW() - INTERVAL '7 days'",
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
