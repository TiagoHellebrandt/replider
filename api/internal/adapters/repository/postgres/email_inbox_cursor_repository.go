package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// EmailInboxCursorRepository persists where the inbox poller stopped reading.
//
// Raw SQL rather than sqlc for the same reason the other poll-state repos are:
// the table is mutable bookkeeping for one worker, nothing else reads it, and
// giving it a generated querier would put four methods nobody else calls into a
// shared interface.
type EmailInboxCursorRepository struct {
	db DBTX
}

func NewEmailInboxCursorRepository(db DBTX) *EmailInboxCursorRepository {
	return &EmailInboxCursorRepository{db: db}
}

var _ domain.EmailInboxCursorRepository = (*EmailInboxCursorRepository)(nil)

const cursorColumns = `channel_id, account_id, mailbox, uid_validity, last_uid,
	last_polled_at, COALESCE(last_error, ''), failure_count, created_at, updated_at`

func scanCursor(row pgx.Row) (*domain.EmailInboxCursor, error) {
	var c domain.EmailInboxCursor
	// uid_validity and last_uid are BIGINT so a uint32 near the top of its range
	// round-trips; read into int64 and narrow, because pgx will not scan a
	// BIGINT straight into a uint32.
	var validity, lastUID int64
	err := row.Scan(&c.ChannelID, &c.AccountID, &c.Mailbox, &validity, &lastUID,
		&c.LastPolledAt, &c.LastError, &c.FailureCount, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.UIDValidity = uint32(validity) //nolint:gosec // G115: written from a uint32; the column is BIGINT only so the full range fits
	c.LastUID = uint32(lastUID)      //nolint:gosec // G115: same
	return &c, nil
}

func normalizeMailbox(mailbox string) string {
	m := strings.TrimSpace(mailbox)
	if m == "" {
		return "INBOX"
	}
	return m
}

// EnsureForChannel creates the row unanchored on first use.
//
// The insert is a no-op on conflict rather than an update: an existing cursor is
// a position, and overwriting it here would silently re-read (or skip) mail
// every time the poller started.
func (r *EmailInboxCursorRepository) EnsureForChannel(ctx context.Context, accountID, channelID, mailbox string) (*domain.EmailInboxCursor, error) {
	mb := normalizeMailbox(mailbox)
	row := r.db.QueryRow(ctx,
		`INSERT INTO email_imap_cursors (channel_id, account_id, mailbox)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (channel_id) DO UPDATE SET channel_id = email_imap_cursors.channel_id
		 RETURNING `+cursorColumns,
		channelID, accountID, mb)
	return scanCursor(row)
}

// Advance moves the cursor forward and clears the failure state.
//
// `last_uid = GREATEST(...)` is the whole safety property: two overlapping polls
// (a slow one still finishing while the next tick starts) can otherwise write
// their results in either order, and the lower one would re-deliver every
// message between them. Guarded on uid_validity too, so a poll that read a
// mailbox generation the row has since moved past cannot rewind it.
func (r *EmailInboxCursorRepository) Advance(ctx context.Context, channelID, mailbox string, uidValidity, lastUID uint32, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE email_imap_cursors
		    SET last_uid = GREATEST(last_uid, $3::bigint),
		        uid_validity = $4::bigint,
		        last_polled_at = $5,
		        last_error = NULL,
		        failure_count = 0,
		        updated_at = NOW()
		  WHERE channel_id = $1 AND mailbox = $2
		    AND (uid_validity = 0 OR uid_validity = $4::bigint)`,
		channelID, normalizeMailbox(mailbox), int64(lastUID), int64(uidValidity), at)
	return err
}

// Reanchor replaces the position wholesale.
//
// The one operation allowed to move the cursor BACKWARDS, and only because the
// server told us the UID space was rebuilt: the old numbers address different
// messages now, so keeping the higher one would skip mail forever.
func (r *EmailInboxCursorRepository) Reanchor(ctx context.Context, channelID, mailbox string, uidValidity, lastUID uint32, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE email_imap_cursors
		    SET uid_validity = $3::bigint,
		        last_uid = $4::bigint,
		        last_polled_at = $5,
		        last_error = NULL,
		        failure_count = 0,
		        updated_at = NOW()
		  WHERE channel_id = $1 AND mailbox = $2`,
		channelID, normalizeMailbox(mailbox), int64(uidValidity), int64(lastUID), at)
	return err
}

// RecordFailure stamps the attempt so a failing inbox stops being polled every
// tick, and keeps the reason so "why did nothing arrive?" is answerable from the
// row instead of from logs.
func (r *EmailInboxCursorRepository) RecordFailure(ctx context.Context, channelID, mailbox string, at time.Time, reason string) error {
	// Truncated because the value is shown in the UI and a provider can answer
	// with a wall of text.
	if len(reason) > 500 {
		reason = reason[:500]
	}
	_, err := r.db.Exec(ctx,
		`UPDATE email_imap_cursors
		    SET last_polled_at = $3,
		        last_error = $4,
		        failure_count = failure_count + 1,
		        updated_at = NOW()
		  WHERE channel_id = $1 AND mailbox = $2`,
		channelID, normalizeMailbox(mailbox), at, reason)
	return err
}

// GetByChannel returns the channel's cursor, or nil when the inbox was never
// registered (a channel created before polling existed, or whose
// registration failed). Feeds the channel response's IMAP health fields.
func (r *EmailInboxCursorRepository) GetByChannel(ctx context.Context, channelID string) (*domain.EmailInboxCursor, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+cursorColumns+`
		   FROM email_imap_cursors
		  WHERE channel_id = $1
		  LIMIT 1`,
		channelID)
	c, err := scanCursor(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

// ListDue returns cursors not polled since `before`, never-polled first.
func (r *EmailInboxCursorRepository) ListDue(ctx context.Context, before time.Time, limit int32) ([]*domain.EmailInboxCursor, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(ctx,
		`SELECT `+cursorColumns+`
		   FROM email_imap_cursors
		  WHERE last_polled_at IS NULL OR last_polled_at < $1
		  ORDER BY last_polled_at ASC NULLS FIRST
		  LIMIT $2`,
		before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.EmailInboxCursor
	for rows.Next() {
		c, err := scanCursor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	return out, nil
}
