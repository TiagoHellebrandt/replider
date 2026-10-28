package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// Raw SQL, like the other tables sqlc does not own.
type EmailRiskRepository struct{ db DBTX }

func NewEmailRiskRepository(db DBTX) *EmailRiskRepository {
	return &EmailRiskRepository{db: db}
}

// Save records an assessment. Idempotent by message: a reprocessed webhook
// must overwrite the verdict, not accumulate a second one.
func (r *EmailRiskRepository) Save(ctx context.Context, rec *domain.EmailRiskRecord) error {
	signals, err := json.Marshal(rec.Assessment.Signals)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx,
		`INSERT INTO email_risk_assessments
		   (message_id, account_id, score, quarantined, signals, sender_domain)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (message_id) DO UPDATE
		    SET score = EXCLUDED.score,
		        quarantined = EXCLUDED.quarantined,
		        signals = EXCLUDED.signals,
		        sender_domain = EXCLUDED.sender_domain`,
		rec.MessageID, rec.AccountID, rec.Assessment.Score,
		rec.Assessment.Quarantined(), signals, rec.SenderDomain)
	return err
}

// GetByMessage returns the assessment, or nil when nothing was found for
// that message — which is the normal case and not an error.
func (r *EmailRiskRepository) GetByMessage(ctx context.Context, messageID string) (*domain.EmailRiskRecord, error) {
	var (
		rec     domain.EmailRiskRecord
		signals []byte
	)
	err := r.db.QueryRow(ctx,
		`SELECT message_id, account_id, score, signals, sender_domain, created_at
		   FROM email_risk_assessments WHERE message_id = $1`, messageID).
		Scan(&rec.MessageID, &rec.AccountID, &rec.Assessment.Score,
			&signals, &rec.SenderDomain, &rec.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(signals, &rec.Assessment.Signals); err != nil {
		return nil, err
	}
	return &rec, nil
}

// ListByMessages is the batched form for a transcript page.
func (r *EmailRiskRepository) ListByMessages(ctx context.Context, messageIDs []string) (map[string]*domain.EmailRiskRecord, error) {
	out := map[string]*domain.EmailRiskRecord{}
	if len(messageIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx,
		`SELECT message_id, account_id, score, signals, sender_domain, created_at
		   FROM email_risk_assessments WHERE message_id = ANY($1)`, messageIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			rec     domain.EmailRiskRecord
			signals []byte
		)
		if err := rows.Scan(&rec.MessageID, &rec.AccountID, &rec.Assessment.Score,
			&signals, &rec.SenderDomain, &rec.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(signals, &rec.Assessment.Signals); err != nil {
			return nil, err
		}
		copied := rec
		out[rec.MessageID] = &copied
	}
	return out, rows.Err()
}

// KnownDomains lists the domains this account has corresponded with, which is
// what makes a lookalike detectable at all: a domain resembling nothing you
// know is just a domain.
//
// Reads from contacts rather than from messages so it covers people the
// account prospected as well as people who wrote in.
func (r *EmailRiskRepository) KnownDomains(ctx context.Context, accountID string, limit int32) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT DISTINCT lower(split_part(external_id, '@', 2)) AS domain
		   FROM contacts
		  WHERE account_id = $1
		    AND external_id LIKE '%@%'
		  LIMIT $2`, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		if d != "" {
			out = append(out, d)
		}
	}
	return out, rows.Err()
}

// SeenBefore reports whether this exact address has written to the account
// before. Excludes the message being assessed, which is already persisted by
// the time the check runs.
func (r *EmailRiskRepository) SeenBefore(ctx context.Context, accountID, address, exceptMessageID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM messages m
		     JOIN conversations c ON c.id = m.conversation_id
		    WHERE c.account_id = $1
		      AND m.sender = $2
		      AND m.direction = 'inbound'
		      AND m.id <> $3)`, accountID, address, exceptMessageID).Scan(&exists)
	return exists, err
}

// Clear removes an assessment, used when a human vouches for the sender. The
// warning disappears because it was answered, not because it was hidden.
func (r *EmailRiskRepository) Clear(ctx context.Context, messageID, accountID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM email_risk_assessments WHERE message_id = $1 AND account_id = $2`,
		messageID, accountID)
	return err
}

// SenderAllowlistRepository remembers senders a human has vouched for.
type SenderAllowlistRepository struct{ db DBTX }

func NewSenderAllowlistRepository(db DBTX) *SenderAllowlistRepository {
	return &SenderAllowlistRepository{db: db}
}

// Allow is idempotent: clicking twice is not an error, and the original
// author is kept rather than overwritten by whoever clicked last.
func (r *SenderAllowlistRepository) Allow(ctx context.Context, accountID, address, byUserID string) error {
	var by any
	if byUserID != "" {
		by = byUserID
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO sender_allowlist (account_id, address, allowed_by)
		 VALUES ($1,$2,$3)
		 ON CONFLICT (account_id, address) DO NOTHING`,
		accountID, address, by)
	return err
}

func (r *SenderAllowlistRepository) IsAllowed(ctx context.Context, accountID, address string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM sender_allowlist WHERE account_id = $1 AND address = $2)`,
		accountID, address).Scan(&exists)
	return exists, err
}
