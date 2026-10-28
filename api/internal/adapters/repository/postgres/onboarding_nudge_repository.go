package postgres

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// OnboardingNudgeRepository is a raw-SQL repo (NOT sqlc) over the same pool.
// It owns the accounts.onboarding_nudge_stage_sent column and the due-list
// scan that feeds the onboarding-nudge email worker (US-182). Raw SQL keeps
// this off the expensive-to-extend sqlc AccountRepository, mirroring
// ReactivationRepository / WeeklyDigestRepository.
type OnboardingNudgeRepository struct {
	db DBTX
}

func NewOnboardingNudgeRepository(db DBTX) *OnboardingNudgeRepository {
	return &OnboardingNudgeRepository{db: db}
}

// ListDueForNudge returns accounts created at or after `createdAfter` (the
// rollout safety window — never nudge ancient accounts) that still have an
// onboarding nudge pending (onboarding_nudge_stage_sent < the last stage)
// and are not suspended. The channel/knowledge booleans are derived from
// the same EXISTS lookups as GetOnboardingProgress so "skip if already
// done" stays a single source of truth. The use case decides per-row which
// stage, if any, is actually due. Ordered by id for a deterministic sweep.
func (r *OnboardingNudgeRepository) ListDueForNudge(ctx context.Context, createdAfter time.Time) ([]domain.OnboardingNudgeAccount, error) {
	lastStage := domain.OnboardingNudgeStages[len(domain.OnboardingNudgeStages)-1]
	rows, err := r.db.Query(ctx, `
		SELECT a.id, a.name, a.created_at, a.onboarding_nudge_stage_sent,
		  EXISTS(SELECT 1 FROM channels ch WHERE ch.account_id = a.id) AS channel_connected,
		  EXISTS(
		    SELECT 1 FROM knowledge_sources ks
		    JOIN agents ag ON ag.id = ks.agent_id
		    WHERE ag.account_id = a.id
		  ) AS knowledge_added
		FROM accounts a
		WHERE a.suspended_at IS NULL
		  AND a.created_at >= $1
		  AND a.onboarding_nudge_stage_sent < $2
		ORDER BY a.id`, createdAfter, lastStage)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.OnboardingNudgeAccount
	for rows.Next() {
		var a domain.OnboardingNudgeAccount
		if err := rows.Scan(&a.AccountID, &a.Name, &a.CreatedAt, &a.StageSent,
			&a.ChannelConnected, &a.KnowledgeAdded); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// MarkOnboardingNudgeSent records the milestone day just emailed (or skipped
// because the step was already done) so that stage isn't re-processed.
// Guards against going backwards (a concurrent tick) with GREATEST so the
// column only ever advances.
func (r *OnboardingNudgeRepository) MarkOnboardingNudgeSent(ctx context.Context, accountID string, stage int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE accounts
		    SET onboarding_nudge_stage_sent = GREATEST(onboarding_nudge_stage_sent, $2),
		        updated_at = now()
		  WHERE id = $1`,
		accountID, stage)
	return err
}
