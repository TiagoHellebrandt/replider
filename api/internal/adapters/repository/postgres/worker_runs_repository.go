package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/replider/api/internal/core/domain"
)

// WorkerRunsRepository backs domain.WorkerRunRepository (migration 000223).
// Raw SQL: one upsert and one lookup on a two-column table.
type WorkerRunsRepository struct {
	db *pgxpool.Pool
}

func NewWorkerRunsRepository(db *pgxpool.Pool) *WorkerRunsRepository {
	return &WorkerRunsRepository{db: db}
}

var _ domain.WorkerRunRepository = (*WorkerRunsRepository)(nil)

func (r *WorkerRunsRepository) LastRun(ctx context.Context, name string) (*time.Time, error) {
	var at time.Time
	err := r.db.QueryRow(ctx,
		`SELECT last_run_at FROM worker_runs WHERE name = $1`, name).Scan(&at)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &at, nil
}

func (r *WorkerRunsRepository) Stamp(ctx context.Context, name string, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO worker_runs (name, last_run_at) VALUES ($1, $2)
		 ON CONFLICT (name) DO UPDATE SET last_run_at = EXCLUDED.last_run_at`,
		name, at)
	return err
}
