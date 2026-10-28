package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"github.com/replider/api/internal/core/domain"
)

// AIBatchJobRepository is a raw-SQL repo (NOT sqlc) for provider batch jobs
// (migration 000205).
type AIBatchJobRepository struct {
	db DBTX
}

func NewAIBatchJobRepository(db DBTX) *AIBatchJobRepository {
	return &AIBatchJobRepository{db: db}
}

func (r *AIBatchJobRepository) Create(ctx context.Context, job *domain.AIBatchJob) error {
	items, err := json.Marshal(job.Items)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO ai_batch_jobs (id, account_id, kind, provider, model, job_ref, status, items)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		job.ID, job.AccountID, job.Kind, job.Provider, job.Model, job.JobRef,
		domain.AIBatchSubmitted, items)
	return err
}

func (r *AIBatchJobRepository) ListSubmitted(ctx context.Context) ([]*domain.AIBatchJob, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, account_id, kind, provider, model, job_ref, status, items, error, created_at, updated_at
		FROM ai_batch_jobs WHERE status = 'submitted' ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.AIBatchJob
	for rows.Next() {
		var j domain.AIBatchJob
		var items []byte
		if err := rows.Scan(&j.ID, &j.AccountID, &j.Kind, &j.Provider, &j.Model,
			&j.JobRef, &j.Status, &items, &j.Error, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(items, &j.Items); err != nil {
			j.Items = nil
		}
		out = append(out, &j)
	}
	return out, rows.Err()
}

func (r *AIBatchJobRepository) HasOpenJob(ctx context.Context, accountID, kind string) (bool, error) {
	var one int
	err := r.db.QueryRow(ctx, `
		SELECT 1 FROM ai_batch_jobs
		WHERE account_id = $1 AND kind = $2 AND status = 'submitted' LIMIT 1`,
		accountID, kind).Scan(&one)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *AIBatchJobRepository) SetStatus(ctx context.Context, id, status, errMsg string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ai_batch_jobs SET status = $2, error = $3, updated_at = NOW() WHERE id = $1`,
		id, status, errMsg)
	return err
}
