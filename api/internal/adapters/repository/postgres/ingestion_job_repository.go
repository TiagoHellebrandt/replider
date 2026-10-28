package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// IngestionJobRepository is the Postgres adapter behind the domain
// IngestionJobRepository port. Uses sqlc-generated Queries under the
// hood and translates nullable pgtype values back to the domain's
// optional pointers.
type IngestionJobRepository struct {
	queries *Queries
}

func NewIngestionJobRepository(db DBTX) *IngestionJobRepository {
	return &IngestionJobRepository{queries: New(db)}
}

func (r *IngestionJobRepository) Create(ctx context.Context, job *domain.IngestionJob) error {
	q := getQueries(ctx, r.queries)
	return q.CreateIngestionJob(ctx, CreateIngestionJobParams{
		ID:                job.ID,
		AccountID:         job.AccountID,
		KnowledgeSourceID: job.KnowledgeSourceID,
		Type:              string(job.Type),
		Input:             job.Input,
	})
}

func (r *IngestionJobRepository) GetByID(ctx context.Context, id, accountID string) (*domain.IngestionJob, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetIngestionJob(ctx, GetIngestionJobParams{ID: id, AccountID: accountID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return mapIngestionJob(row), nil
}

func (r *IngestionJobRepository) ListBySource(ctx context.Context, sourceID, accountID string, limit int32) ([]*domain.IngestionJob, error) {
	q := getQueries(ctx, r.queries)
	rows, err := q.ListIngestionJobsBySource(ctx, ListIngestionJobsBySourceParams{
		KnowledgeSourceID: sourceID,
		AccountID:         accountID,
		Limit:             limit,
	})
	if err != nil {
		return nil, err
	}
	jobs := make([]*domain.IngestionJob, len(rows))
	for i, row := range rows {
		jobs[i] = mapIngestionJob(row)
	}
	return jobs, nil
}

func (r *IngestionJobRepository) ClaimNext(ctx context.Context) (*domain.IngestionJob, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.ClaimNextPendingIngestionJob(ctx)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return mapIngestionJob(row), nil
}

func (r *IngestionJobRepository) Complete(ctx context.Context, id string, chunksCreated int32) error {
	q := getQueries(ctx, r.queries)
	return q.CompleteIngestionJob(ctx, CompleteIngestionJobParams{
		ID:            id,
		ChunksCreated: chunksCreated,
	})
}

func (r *IngestionJobRepository) Fail(ctx context.Context, id string, errorMessage string) error {
	q := getQueries(ctx, r.queries)
	msg := errorMessage
	return q.FailIngestionJob(ctx, FailIngestionJobParams{
		ID:           id,
		ErrorMessage: &msg,
	})
}

func mapIngestionJob(row IngestionJob) *domain.IngestionJob {
	job := &domain.IngestionJob{
		ID:                row.ID,
		AccountID:         row.AccountID,
		KnowledgeSourceID: row.KnowledgeSourceID,
		Type:              domain.IngestionJobType(row.Type),
		Input:             row.Input,
		Status:            domain.IngestionJobStatus(row.Status),
		ErrorMessage:      row.ErrorMessage,
		ChunksCreated:     row.ChunksCreated,
		CreatedAt:         row.CreatedAt.Time,
	}
	if row.StartedAt.Valid {
		t := row.StartedAt.Time
		job.StartedAt = &t
	}
	if row.FinishedAt.Valid {
		t := row.FinishedAt.Time
		job.FinishedAt = &t
	}
	return job
}
