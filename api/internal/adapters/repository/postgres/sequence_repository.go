package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Raw SQL, like the other tables sqlc does not own.
type SequenceRepository struct{ db DBTX }

func NewSequenceRepository(db DBTX) *SequenceRepository {
	return &SequenceRepository{db: db}
}

const sequenceCols = `id, account_id, funnel_id, name, steps, stop_on_reply,
	business_hours_only, is_active, created_at, updated_at`

func scanSequence(row pgx.Row) (*domain.Sequence, error) {
	s := &domain.Sequence{}
	var funnelID *string
	var steps []byte
	err := row.Scan(&s.ID, &s.AccountID, &funnelID, &s.Name, &steps,
		&s.StopOnReply, &s.BusinessHoursOnly, &s.IsActive, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if funnelID != nil {
		s.FunnelID = *funnelID
	}
	if len(steps) > 0 {
		// A cadence whose steps will not parse is a cadence that would run
		// nothing. Reported as empty so Validate refuses it loudly rather
		// than the worker silently skipping it forever.
		_ = json.Unmarshal(steps, &s.Steps)
	}
	return s, nil
}

func (r *SequenceRepository) List(ctx context.Context, accountID string) ([]*domain.Sequence, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+sequenceCols+` FROM sequences
		  WHERE account_id = $1 ORDER BY created_at DESC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Sequence
	for rows.Next() {
		s, err := scanSequence(rows)
		if err != nil {
			return nil, err
		}
		if s != nil {
			out = append(out, s)
		}
	}
	return out, rows.Err()
}

func (r *SequenceRepository) Get(ctx context.Context, id, accountID string) (*domain.Sequence, error) {
	return scanSequence(r.db.QueryRow(ctx,
		`SELECT `+sequenceCols+` FROM sequences WHERE id = $1 AND account_id = $2`,
		id, accountID))
}

func (r *SequenceRepository) Upsert(ctx context.Context, s *domain.Sequence) error {
	steps, err := json.Marshal(s.Steps)
	if err != nil {
		return err
	}
	var funnelID *string
	if s.FunnelID != "" {
		funnelID = &s.FunnelID
	}
	_, err = r.db.Exec(ctx,
		`INSERT INTO sequences (id, account_id, funnel_id, name, steps,
		    stop_on_reply, business_hours_only, is_active)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT (id) DO UPDATE
		    SET funnel_id           = EXCLUDED.funnel_id,
		        name                = EXCLUDED.name,
		        steps               = EXCLUDED.steps,
		        business_hours_only = EXCLUDED.business_hours_only,
		        is_active           = EXCLUDED.is_active,
		        -- stop_on_reply is deliberately NOT updatable. It is the rule
		        -- that makes a cadence a follow-up rather than a machine
		        -- talking over somebody, and an API that can switch it off is
		        -- an API where somebody eventually does.
		        updated_at          = NOW()
		  WHERE sequences.account_id = EXCLUDED.account_id`,
		s.ID, s.AccountID, funnelID, s.Name, steps,
		true, s.BusinessHoursOnly, s.IsActive)
	return err
}

func (r *SequenceRepository) Delete(ctx context.Context, id, accountID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM sequences WHERE id = $1 AND account_id = $2`, id, accountID)
	return err
}

// ── Enrollment ────────────────────────────────────────────────────────────

const enrollmentCols = `id, account_id, sequence_id, lead_id, current_step,
	status, next_run_at, enrolled_at, ended_at`

func scanEnrollment(row pgx.Row) (*domain.SequenceEnrollment, error) {
	e := &domain.SequenceEnrollment{}
	var status string
	err := row.Scan(&e.ID, &e.AccountID, &e.SequenceID, &e.LeadID,
		&e.CurrentStep, &status, &e.NextRunAt, &e.EnrolledAt, &e.EndedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	e.Status = domain.EnrollmentStatus(status)
	return e, nil
}

// Enroll starts a lead on a cadence.
//
// DO NOTHING on the partial unique index rather than an error: two people
// enrolling the same lead a second apart is an ordinary race, and the honest
// outcome is that the lead is enrolled once.
func (r *SequenceRepository) Enroll(ctx context.Context, e *domain.SequenceEnrollment) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO sequence_enrollments
		   (id, account_id, sequence_id, lead_id, current_step, status, next_run_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 ON CONFLICT (account_id, lead_id) WHERE status = 'active' DO NOTHING`,
		e.ID, e.AccountID, e.SequenceID, e.LeadID, e.CurrentStep,
		string(e.Status), e.NextRunAt)
	return err
}

func (r *SequenceRepository) GetEnrollment(ctx context.Context, accountID, leadID string) (*domain.SequenceEnrollment, error) {
	return scanEnrollment(r.db.QueryRow(ctx,
		`SELECT `+enrollmentCols+` FROM sequence_enrollments
		  WHERE account_id = $1 AND lead_id = $2 AND status = 'active'`,
		accountID, leadID))
}

// DueEnrollments is what the worker acts on.
//
// Ordered by next_run_at so the most overdue goes first: after an outage the
// backlog should drain in the order it was owed, not in id order.
func (r *SequenceRepository) DueEnrollments(ctx context.Context, now time.Time, limit int) ([]*domain.SequenceEnrollment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+enrollmentCols+` FROM sequence_enrollments
		  WHERE status = 'active' AND next_run_at IS NOT NULL AND next_run_at <= $1
		  ORDER BY next_run_at
		  LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.SequenceEnrollment
	for rows.Next() {
		e, err := scanEnrollment(rows)
		if err != nil {
			return nil, err
		}
		if e != nil {
			out = append(out, e)
		}
	}
	return out, rows.Err()
}

func (r *SequenceRepository) UpdateEnrollment(ctx context.Context, e *domain.SequenceEnrollment) error {
	_, err := r.db.Exec(ctx,
		`UPDATE sequence_enrollments
		    SET current_step = $2, status = $3, next_run_at = $4, ended_at = $5
		  WHERE id = $1`,
		e.ID, e.CurrentStep, string(e.Status), e.NextRunAt, e.EndedAt)
	return err
}

// StopForLead ends every active enrollment for a lead.
//
// This is the rule that makes a cadence a follow-up rather than a machine
// talking over somebody, so it is one statement: a read-then-write could lose
// the race against the worker firing the next step.
func (r *SequenceRepository) StopForLead(ctx context.Context, accountID, leadID string, status domain.EnrollmentStatus, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE sequence_enrollments
		    SET status = $3, ended_at = $4, next_run_at = NULL
		  WHERE account_id = $1 AND lead_id = $2 AND status = 'active'`,
		accountID, leadID, string(status), at)
	return err
}

// RecordStepEvent writes one funnel fact.
//
// Idempotent by (enrollment, step, kind): a retried worker pass must not
// inflate the funnel it is measuring.
func (r *SequenceRepository) RecordStepEvent(ctx context.Context, sequenceID, enrollmentID string, stepOrder int, kind string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO sequence_step_events
		   (id, sequence_id, enrollment_id, step_order, kind)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (enrollment_id, step_order, kind) DO NOTHING`,
		ulid.Make().String(), sequenceID, enrollmentID, stepOrder, kind)
	return err
}

// StepStats is the per-step funnel.
func (r *SequenceRepository) StepStats(ctx context.Context, sequenceID string) ([]domain.SequenceStepStat, error) {
	rows, err := r.db.Query(ctx,
		`SELECT step_order,
		        COUNT(*) FILTER (WHERE kind = 'reached') AS reached,
		        COUNT(*) FILTER (WHERE kind = 'replied') AS replied
		   FROM sequence_step_events
		  WHERE sequence_id = $1
		  GROUP BY step_order
		  ORDER BY step_order`, sequenceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.SequenceStepStat
	for rows.Next() {
		var s domain.SequenceStepStat
		if err := rows.Scan(&s.StepOrder, &s.Reached, &s.Replied); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

var _ domain.SequenceRepository = (*SequenceRepository)(nil)
