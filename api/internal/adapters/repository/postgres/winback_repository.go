package postgres

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// WinBackRepository is a raw-SQL repo (NOT sqlc) over the same pool. It
// owns the accounts.winback_* columns and the churned-account due-list
// scan that feeds the win-back follow-up worker (US-179). Raw SQL keeps
// this off the expensive-to-extend sqlc AccountRepository, mirroring
// ReactivationRepository.
type WinBackRepository struct {
	db DBTX
}

func NewWinBackRepository(db DBTX) *WinBackRepository {
	return &WinBackRepository{db: db}
}

// StartWinBack records that the win-back flow has begun for an account:
// stamps the recovery-window anchor and the personal coupon code, and
// resets the follow-up stage to 0. Idempotent on replay — a second
// cancellation webhook just overwrites with a fresh window + code.
func (r *WinBackRepository) StartWinBack(ctx context.Context, accountID, couponCode string, now time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE accounts
		    SET winback_started_at = $2,
		        winback_coupon_code = $3,
		        winback_stage_sent = 0,
		        updated_at = now()
		  WHERE id = $1`,
		accountID, now, couponCode)
	return err
}

// ListWinBackDue returns accounts inside the win-back window (started, not
// yet through the last follow-up stage). The use case decides per row
// which milestone, if any, is actually due from StartedAt. Ordered by id
// so the sweep is deterministic.
func (r *WinBackRepository) ListWinBackDue(ctx context.Context) ([]domain.WinBackAccount, error) {
	lastStage := domain.WinBackStages[len(domain.WinBackStages)-1]
	rows, err := r.db.Query(ctx, `
		SELECT id, name, winback_started_at, COALESCE(winback_coupon_code, ''), winback_stage_sent
		FROM accounts
		WHERE winback_started_at IS NOT NULL
		  AND winback_stage_sent < $1
		ORDER BY id`, lastStage)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.WinBackAccount
	for rows.Next() {
		var a domain.WinBackAccount
		if err := rows.Scan(&a.AccountID, &a.Name, &a.StartedAt, &a.CouponCode, &a.StageSent); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// MarkWinBackSent advances the last follow-up milestone emailed so it
// isn't re-sent. GREATEST guards against a concurrent tick moving it
// backwards (the column only ever advances).
func (r *WinBackRepository) MarkWinBackSent(ctx context.Context, accountID string, stage int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE accounts
		    SET winback_stage_sent = GREATEST(winback_stage_sent, $2),
		        updated_at = now()
		  WHERE id = $1`,
		accountID, stage)
	return err
}
