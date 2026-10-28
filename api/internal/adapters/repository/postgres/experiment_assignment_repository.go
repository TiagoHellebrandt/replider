package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

type ExperimentAssignmentRepository struct {
	db DBTX
}

func NewExperimentAssignmentRepository(db DBTX) *ExperimentAssignmentRepository {
	return &ExperimentAssignmentRepository{db: db}
}

func (r *ExperimentAssignmentRepository) Assign(ctx context.Context, a *domain.ExperimentAssignment) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO experiment_assignments (experiment_id, conversation_id, variant, assigned_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (experiment_id, conversation_id) DO NOTHING`,
		a.ExperimentID, a.ConversationID, a.Variant,
	)
	return err
}

// AssignReturning upserts and reads the committed variant atomically.
// ON CONFLICT DO UPDATE (a no-op SET to the row's own variant) is what
// makes RETURNING fire even when the row already existed: a plain DO
// NOTHING returns no rows on conflict, forcing a second SELECT and
// reopening the race. The no-op update keeps the stored variant intact
// while guaranteeing exactly one row comes back — the winner's variant.
func (r *ExperimentAssignmentRepository) AssignReturning(ctx context.Context, a *domain.ExperimentAssignment) (string, error) {
	var variant string
	err := r.db.QueryRow(ctx,
		`INSERT INTO experiment_assignments (experiment_id, conversation_id, variant, assigned_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (experiment_id, conversation_id)
		 DO UPDATE SET variant = experiment_assignments.variant
		 RETURNING variant`,
		a.ExperimentID, a.ConversationID, a.Variant,
	).Scan(&variant)
	if err != nil {
		return "", err
	}
	return variant, nil
}

func (r *ExperimentAssignmentRepository) Get(ctx context.Context, experimentID, conversationID string) (*domain.ExperimentAssignment, error) {
	a := &domain.ExperimentAssignment{}
	err := r.db.QueryRow(ctx,
		`SELECT experiment_id, conversation_id, variant, assigned_at
		 FROM experiment_assignments WHERE experiment_id = $1 AND conversation_id = $2`,
		experimentID, conversationID,
	).Scan(&a.ExperimentID, &a.ConversationID, &a.Variant, &a.AssignedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return a, nil
}

func (r *ExperimentAssignmentRepository) GetByConversation(ctx context.Context, conversationID string) (*domain.ExperimentAssignment, error) {
	a := &domain.ExperimentAssignment{}
	err := r.db.QueryRow(ctx,
		`SELECT experiment_id, conversation_id, variant, assigned_at
		 FROM experiment_assignments WHERE conversation_id = $1
		 ORDER BY assigned_at DESC LIMIT 1`,
		conversationID,
	).Scan(&a.ExperimentID, &a.ConversationID, &a.Variant, &a.AssignedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return a, nil
}

var _ domain.ExperimentAssignmentRepository = (*ExperimentAssignmentRepository)(nil)
