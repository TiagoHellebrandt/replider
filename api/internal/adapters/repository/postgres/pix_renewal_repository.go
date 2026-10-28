package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/replider/api/internal/core/domain"
)

// PIX renewal state (US-M014). Raw SQL: it reads columns no sqlc query
// touches, and the whole surface is one list plus two stamps.
type PixRenewalRepository struct {
	db *pgxpool.Pool
}

func NewPixRenewalRepository(db *pgxpool.Pool) *PixRenewalRepository {
	return &PixRenewalRepository{db: db}
}

var _ domain.PixRenewalRepository = (*PixRenewalRepository)(nil)

// ListDue returns BRL accounts whose renewal is inside the ladder's reach
// (from 3 days before the period ends to 30 days after).
//
// The upper bound matters: without it, every account ever suspended stays
// in the work-list forever, and the job's cost grows with churn instead of
// with active customers.
func (r *PixRenewalRepository) ListDue(ctx context.Context, now time.Time, limit int) ([]domain.PixRenewalAccount, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, COALESCE(plan_id, ''), current_period_end,
		        pix_cycle_stage_sent, pix_cycle_period_end,
		        COALESCE(pix_cycle_charge_id, '')
		 FROM accounts
		 WHERE current_period_end IS NOT NULL
		   AND currency = 'brl'
		   AND stripe_subscription_id IS NULL
		   AND current_period_end BETWEEN $1::timestamptz - interval '30 days'
		                             AND $1::timestamptz + interval '3 days'
		 ORDER BY current_period_end
		 LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.PixRenewalAccount
	for rows.Next() {
		var (
			a       domain.PixRenewalAccount
			stage   int16
			stampAt *time.Time
		)
		if err := rows.Scan(&a.AccountID, &a.PlanID, &a.PeriodEnd, &stage, &stampAt, &a.ChargeID); err != nil {
			return nil, err
		}
		a.StageSent = domain.PixDunningStage(stage)
		a.StagePeriodEnd = stampAt
		out = append(out, a)
	}
	return out, rows.Err()
}

// MarkStage advances the ladder with GREATEST, so a late worker cannot
// walk an account backwards and re-send a rung it already delivered.
func (r *PixRenewalRepository) MarkStage(ctx context.Context, accountID string, stage domain.PixDunningStage, periodEnd time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE accounts
		    SET pix_cycle_stage_sent = CASE
		          WHEN pix_cycle_period_end IS DISTINCT FROM $3 THEN $2
		          ELSE GREATEST(pix_cycle_stage_sent, $2)
		        END,
		        pix_cycle_period_end = $3
		  WHERE id = $1`, accountID, int16(stage), periodEnd) //nolint:gosec // G115: PixDunningStage is a closed 0..4 enum (PixStageNone..PixStageSuspended), so it can never exceed int16
	return err
}

// SetChargeID remembers the outstanding charge so reminders point at the
// same PIX code.
func (r *PixRenewalRepository) SetChargeID(ctx context.Context, accountID, chargeID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE accounts SET pix_cycle_charge_id = $2 WHERE id = $1`, accountID, chargeID)
	return err
}
