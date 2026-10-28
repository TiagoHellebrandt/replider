package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// Raw SQL, like the other tables sqlc does not own.
type AutopilotRepository struct{ db DBTX }

func NewAutopilotRepository(db DBTX) *AutopilotRepository {
	return &AutopilotRepository{db: db}
}

// Get returns the account's setting, or a zero-value one.
//
// Never nil for an account that simply never touched the feature: a nil would
// make every caller nil-check before asking "is this on?", and the honest
// answer for an untouched account is "off, streak zero".
func (r *AutopilotRepository) Get(ctx context.Context, accountID string) (*domain.Autopilot, error) {
	a := &domain.Autopilot{AccountID: accountID}
	var enabledBy *string
	err := r.db.QueryRow(ctx,
		`SELECT enabled, daily_cap, min_score, unedited_streak,
		        offered_at, enabled_at, enabled_by, updated_at
		   FROM autopilot_settings WHERE account_id = $1`, accountID).
		Scan(&a.Enabled, &a.DailyCap, &a.MinScore, &a.UneditedStreak,
			&a.OfferedAt, &a.EnabledAt, &enabledBy, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return a, nil
		}
		return nil, err
	}
	if enabledBy != nil {
		a.EnabledBy = *enabledBy
	}
	return a, nil
}

// Save writes the operator's choices.
//
// Deliberately does NOT touch unedited_streak: that number is earned by
// behaviour, and a settings save must not be able to grant it.
func (r *AutopilotRepository) Save(ctx context.Context, a *domain.Autopilot) error {
	var enabledBy *string
	if a.EnabledBy != "" {
		enabledBy = &a.EnabledBy
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO autopilot_settings
		   (account_id, enabled, daily_cap, min_score, enabled_at, enabled_by)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (account_id) DO UPDATE
		    SET enabled    = EXCLUDED.enabled,
		        daily_cap  = EXCLUDED.daily_cap,
		        min_score  = EXCLUDED.min_score,
		        enabled_at = EXCLUDED.enabled_at,
		        enabled_by = EXCLUDED.enabled_by,
		        updated_at = NOW()`,
		a.AccountID, a.Enabled, a.EffectiveDailyCap(), a.EffectiveMinScore(),
		a.EnabledAt, enabledBy)
	return err
}

// RecordApproval moves the streak in one statement.
//
// One method rather than an increment and a reset, so the reset can never be
// forgotten at a call site — which is the failure that would quietly offer
// autopilot to somebody who edits every draft.
func (r *AutopilotRepository) RecordApproval(ctx context.Context, accountID string, unedited bool) error {
	next := 0
	if unedited {
		next = 1
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO autopilot_settings (account_id, unedited_streak)
		 VALUES ($1, $2)
		 ON CONFLICT (account_id) DO UPDATE
		    SET unedited_streak = CASE
		          WHEN $3 THEN autopilot_settings.unedited_streak + 1
		          ELSE 0
		        END,
		        updated_at = NOW()`,
		accountID, next, unedited)
	return err
}

func (r *AutopilotRepository) MarkOffered(ctx context.Context, accountID string, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO autopilot_settings (account_id, offered_at)
		 VALUES ($1, $2)
		 ON CONFLICT (account_id) DO UPDATE
		    SET offered_at = COALESCE(autopilot_settings.offered_at, EXCLUDED.offered_at),
		        updated_at = NOW()`,
		accountID, at)
	return err
}

// CountSentToday backs the daily cap.
//
// Counted from the audit rows rather than a counter column: a counter that
// drifts silently uncaps the feature, and the rows have to exist anyway.
func (r *AutopilotRepository) CountSentToday(ctx context.Context, accountID string, since time.Time) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM autopilot_sends
		  WHERE account_id = $1 AND sent_at >= $2`, accountID, since).Scan(&n)
	return n, err
}

func (r *AutopilotRepository) RecordSend(ctx context.Context, e *domain.AutopilotSend) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO autopilot_sends
		   (id, account_id, lead_id, recipient, text, lead_score)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		e.ID, e.AccountID, e.LeadID, e.Recipient, e.Text, e.LeadScore)
	return err
}

func (r *AutopilotRepository) ListSends(ctx context.Context, accountID string, limit, offset int) ([]*domain.AutopilotSend, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, account_id, lead_id, recipient, text, lead_score, sent_at
		   FROM autopilot_sends
		  WHERE account_id = $1
		  ORDER BY sent_at DESC
		  LIMIT $2 OFFSET $3`, accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.AutopilotSend
	for rows.Next() {
		e := &domain.AutopilotSend{}
		if err := rows.Scan(&e.ID, &e.AccountID, &e.LeadID, &e.Recipient,
			&e.Text, &e.LeadScore, &e.SentAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

var _ domain.AutopilotRepository = (*AutopilotRepository)(nil)
