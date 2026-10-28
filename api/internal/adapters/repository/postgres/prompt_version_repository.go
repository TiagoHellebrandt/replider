package postgres

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

type PromptVersionRepository struct {
	db DBTX
}

func NewPromptVersionRepository(db DBTX) *PromptVersionRepository {
	return &PromptVersionRepository{db: db}
}

func (r *PromptVersionRepository) Create(ctx context.Context, v *domain.PromptVersion) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO prompt_versions (id, agent_id, prompt_id, body, applied_by, applied_at, note)
		 VALUES ($1, $2, $3, $4, $5, NOW(), $6)`,
		v.ID, v.AgentID, v.PromptID, v.Body, v.AppliedBy, v.Note,
	)
	return err
}

func (r *PromptVersionRepository) GetByID(ctx context.Context, id string) (*domain.PromptVersion, error) {
	v := &domain.PromptVersion{}
	err := r.db.QueryRow(ctx,
		`SELECT id, agent_id, prompt_id, body, applied_by, applied_at, note
		 FROM prompt_versions WHERE id = $1`, id).
		Scan(&v.ID, &v.AgentID, &v.PromptID, &v.Body, &v.AppliedBy, &v.AppliedAt, &v.Note)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (r *PromptVersionRepository) ListByAgentID(ctx context.Context, agentID string, limit, offset int) ([]*domain.PromptVersion, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, agent_id, prompt_id, body, applied_by, applied_at, note
		 FROM prompt_versions WHERE agent_id = $1 ORDER BY applied_at DESC LIMIT $2 OFFSET $3`,
		agentID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]*domain.PromptVersion, 0)
	for rows.Next() {
		v := &domain.PromptVersion{}
		if err := rows.Scan(&v.ID, &v.AgentID, &v.PromptID, &v.Body, &v.AppliedBy, &v.AppliedAt, &v.Note); err != nil {
			return nil, err
		}
		results = append(results, v)
	}
	return results, rows.Err()
}

var _ domain.PromptVersionRepository = (*PromptVersionRepository)(nil)
