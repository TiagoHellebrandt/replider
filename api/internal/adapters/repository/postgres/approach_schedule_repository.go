package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/replider/api/internal/core/domain"
)

// ApproachScheduleRepository is raw SQL over the pool (the outbound_scheduled
// precedent): the claim query needs FOR UPDATE SKIP LOCKED inside an UPDATE,
// which sqlc renders no better than it reads here.
type ApproachScheduleRepository struct {
	db *pgxpool.Pool
}

func NewApproachScheduleRepository(db *pgxpool.Pool) *ApproachScheduleRepository {
	return &ApproachScheduleRepository{db: db}
}

func (r *ApproachScheduleRepository) Upsert(ctx context.Context, s *domain.ApproachSchedule) error {
	// The arbiter is the partial unique on live rows: a lead with a send
	// already waiting gets its subject and time refreshed, never a twin.
	_, err := r.db.Exec(ctx, `
		INSERT INTO approach_schedule (id, account_id, lead_id, subject, send_at, status)
		VALUES ($1, $2, $3, $4, $5, 'pending')
		ON CONFLICT (lead_id) WHERE status IN ('pending', 'processing')
		DO UPDATE SET subject = EXCLUDED.subject,
		              send_at = EXCLUDED.send_at,
		              status = 'pending',
		              updated_at = NOW()
	`, s.ID, s.AccountID, s.LeadID, s.Subject, s.SendAt)
	if err != nil {
		return fmt.Errorf("upsert approach schedule: %w", err)
	}
	return nil
}

func (r *ApproachScheduleRepository) ClaimDue(ctx context.Context, limit int32) ([]*domain.ApproachSchedule, error) {
	rows, err := r.db.Query(ctx, `
		WITH claimed AS (
			SELECT id FROM approach_schedule
			WHERE status = 'pending' AND send_at <= NOW()
			ORDER BY send_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE approach_schedule a
		SET status = 'processing', updated_at = NOW()
		FROM claimed
		WHERE a.id = claimed.id
		RETURNING a.id, a.account_id, a.lead_id, a.subject, a.send_at,
		          a.status, a.attempts, a.last_error
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("claim due approach schedules: %w", err)
	}
	defer rows.Close()
	var out []*domain.ApproachSchedule
	for rows.Next() {
		s := &domain.ApproachSchedule{}
		if err := rows.Scan(&s.ID, &s.AccountID, &s.LeadID, &s.Subject,
			&s.SendAt, &s.Status, &s.Attempts, &s.LastError); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *ApproachScheduleRepository) MarkSent(ctx context.Context, id string, at time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE approach_schedule
		SET status = 'sent', sent_at = $2, updated_at = NOW()
		WHERE id = $1
	`, id, at)
	return err
}

func (r *ApproachScheduleRepository) Reschedule(ctx context.Context, id string, at time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE approach_schedule
		SET status = 'pending', send_at = $2, updated_at = NOW()
		WHERE id = $1
	`, id, at)
	return err
}

func (r *ApproachScheduleRepository) MarkFailed(ctx context.Context, id, reason string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE approach_schedule
		SET status = 'failed', last_error = $2, updated_at = NOW()
		WHERE id = $1
	`, id, reason)
	return err
}

func (r *ApproachScheduleRepository) TouchAttempt(ctx context.Context, id, reason string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE approach_schedule
		SET status = 'pending', attempts = attempts + 1, last_error = $2,
		    updated_at = NOW()
		WHERE id = $1
	`, id, reason)
	return err
}

func (r *ApproachScheduleRepository) PendingByLeadIDs(ctx context.Context, accountID string, leadIDs []string) (map[string]time.Time, error) {
	if len(leadIDs) == 0 {
		return map[string]time.Time{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT lead_id, send_at FROM approach_schedule
		WHERE account_id = $1
		  AND lead_id = ANY($2)
		  AND status IN ('pending', 'processing')
	`, accountID, leadIDs)
	if err != nil {
		return nil, fmt.Errorf("pending approach schedules by lead: %w", err)
	}
	defer rows.Close()
	out := map[string]time.Time{}
	for rows.Next() {
		var leadID string
		var at time.Time
		if err := rows.Scan(&leadID, &at); err != nil {
			return nil, err
		}
		out[leadID] = at
	}
	return out, rows.Err()
}
