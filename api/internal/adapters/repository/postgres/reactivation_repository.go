package postgres

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// ReactivationRepository is a raw-SQL repo (NOT sqlc) over the same pool.
// It owns the accounts.reactivation_stage_sent column and the lapsed-trial
// due-list scan that feeds the reactivation email worker (US-181). Raw SQL
// keeps this off the expensive-to-extend sqlc AccountRepository, mirroring
// WeeklyDigestRepository.
type ReactivationRepository struct {
	db DBTX
}

func NewReactivationRepository(db DBTX) *ReactivationRepository {
	return &ReactivationRepository{db: db}
}

// ListLapsedTrials returns accounts whose trial has expired (before `now`),
// that never converted (still subscription_status = 'trialing'), are not
// internal bypass / suspended accounts, and still have a reactivation
// nudge pending (reactivation_stage_sent < the last stage). The use case
// decides per-row which stage, if any, is actually due. Ordered by id so
// the sweep is deterministic.
func (r *ReactivationRepository) ListLapsedTrials(ctx context.Context, now time.Time) ([]domain.ReactivationAccount, error) {
	lastStage := domain.ReactivationStages[len(domain.ReactivationStages)-1]
	rows, err := r.db.Query(ctx, `
		SELECT id, name, trial_expires_at, reactivation_stage_sent
		FROM accounts
		WHERE subscription_status = 'trialing'
		  AND trial_bypass = false
		  AND suspended_at IS NULL
		  AND trial_expires_at IS NOT NULL
		  AND trial_expires_at < $1
		  AND reactivation_stage_sent < $2
		ORDER BY id`, now, lastStage)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ReactivationAccount
	for rows.Next() {
		var a domain.ReactivationAccount
		if err := rows.Scan(&a.AccountID, &a.Name, &a.TrialExpiresAt, &a.StageSent); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// MarkReactivationSent records the milestone day just emailed so that
// stage isn't re-sent. Guards against going backwards (a concurrent tick)
// with a GREATEST so the column only ever advances.
func (r *ReactivationRepository) MarkReactivationSent(ctx context.Context, accountID string, stage int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE accounts
		    SET reactivation_stage_sent = GREATEST(reactivation_stage_sent, $2),
		        updated_at = now()
		  WHERE id = $1`,
		accountID, stage)
	return err
}
