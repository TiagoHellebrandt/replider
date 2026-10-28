package postgres

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// SupportNotifyRepository is a raw-SQL repo (NOT sqlc) over the same pool.
// It owns the internal-CS-thread read/notify columns on conversations
// (support_last_read_at / support_fallback_email_at, migration 000143) and
// the due-query that feeds the unread-email fallback worker (US-004).
// Raw SQL keeps these off the sqlc ConversationRepository and its many
// test mocks — the same pattern as WeeklyDigestRepository.
type SupportNotifyRepository struct {
	db DBTX
}

func NewSupportNotifyRepository(db DBTX) *SupportNotifyRepository {
	return &SupportNotifyRepository{db: db}
}

// MarkThreadRead stamps "the thread owner saw the thread now". Called on
// every GET/POST of /me/support/messages — reading (or writing to) the
// widget IS the read signal. Idempotent; a missing row is a no-op.
func (r *SupportNotifyRepository) MarkThreadRead(ctx context.Context, conversationID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE conversations SET support_last_read_at = NOW()
		WHERE id = $1`, conversationID)
	return err
}

// MarkFallbackEmailSent stamps the unread-fallback email so the thread
// stops being due until the owner reads it again (see ListDueEmailFallbacks).
func (r *SupportNotifyRepository) MarkFallbackEmailSent(ctx context.Context, conversationID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE conversations SET support_fallback_email_at = NOW()
		WHERE id = $1`, conversationID)
	return err
}

// ListDueEmailFallbacks returns internal threads whose newest PLATFORM-side
// (outbound) message is older than `cutoff` and still unread by the thread
// owner, and for which no fallback email was sent inside the current unread
// window. The window logic is the idempotency guard (restart-safe, one
// email per unread session):
//
//   - due while last_outbound > support_last_read_at (unread)
//   - AND support_fallback_email_at <= support_last_read_at (not yet
//     emailed since the owner last read — a second unread message inside
//     the same window is NOT due again)
//   - reading the thread moves support_last_read_at forward, re-arming
//     the fallback for the next unread message.
//
// NULL timestamps coalesce to -infinity so never-read/never-emailed
// threads compare correctly.
func (r *SupportNotifyRepository) ListDueEmailFallbacks(ctx context.Context, cutoff time.Time, limit int) ([]*domain.SupportFallbackCandidate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id,
		       c.contact_id,
		       COALESCE(c.customer_account_id, ''),
		       u.email,
		       u.name,
		       COALESCE(a.currency, ''),
		       lo.last_outbound_at
		FROM conversations c
		JOIN channels ch ON ch.id = c.channel_id AND ch.provider = 'internal'
		JOIN users u ON u.id = c.contact_id
		LEFT JOIN accounts a ON a.id = c.customer_account_id
		JOIN LATERAL (
			SELECT MAX(m.created_at) AS last_outbound_at
			FROM messages m
			WHERE m.conversation_id = c.id AND m.direction = 'outbound'
		) lo ON lo.last_outbound_at IS NOT NULL
		WHERE lo.last_outbound_at <= $1
		  AND lo.last_outbound_at > COALESCE(c.support_last_read_at, '-infinity'::timestamptz)
		  AND COALESCE(c.support_fallback_email_at, '-infinity'::timestamptz)
		      <= COALESCE(c.support_last_read_at, '-infinity'::timestamptz)
		ORDER BY lo.last_outbound_at
		LIMIT $2`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.SupportFallbackCandidate
	for rows.Next() {
		var c domain.SupportFallbackCandidate
		if err := rows.Scan(&c.ConversationID, &c.UserID, &c.CustomerAccountID,
			&c.UserEmail, &c.UserName, &c.Currency, &c.LastOutboundAt); err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}
