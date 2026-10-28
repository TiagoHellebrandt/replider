package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"

	"github.com/replider/api/internal/core/domain"
)

// ObjectionRepository is a raw-SQL repo (NOT sqlc) for the account's
// objection library (migration 000203). Small ordered list, whole-list
// replace — the same shape as the brand kit's claim lists.
type ObjectionRepository struct {
	pool *pgxpool.Pool
}

// NewObjectionRepository takes the POOL (not DBTX): Replace needs its own
// transaction.
func NewObjectionRepository(pool *pgxpool.Pool) *ObjectionRepository {
	return &ObjectionRepository{pool: pool}
}

func (r *ObjectionRepository) List(accountID string) ([]domain.Objection, error) {
	rows, err := r.pool.Query(context.Background(), `
		SELECT id, account_id, label, counter, position, created_at, updated_at
		FROM objections WHERE account_id = $1 ORDER BY position, id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Objection
	for rows.Next() {
		var o domain.Objection
		if err := rows.Scan(&o.ID, &o.AccountID, &o.Label, &o.Counter,
			&o.Position, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Replace swaps the account's whole library. Delete+insert in one
// transaction: order is positional and partial merges of a ten-row list
// buy nothing but bugs.
func (r *ObjectionRepository) Replace(accountID string, items []domain.Objection) error {
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	if _, err := tx.Exec(ctx, `DELETE FROM objections WHERE account_id = $1`, accountID); err != nil {
		return err
	}
	for i, o := range items {
		id := o.ID
		if id == "" {
			id = ulid.Make().String()
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO objections (id, account_id, label, counter, position)
			VALUES ($1, $2, $3, $4, $5)`,
			id, accountID, o.Label, o.Counter, i); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
