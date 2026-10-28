package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/replider/api/internal/core/domain"
)

// AutoHuntRepository names the accounts the nightly hunt should work for.
//
// Raw SQL and its own type, like the other cross-table listers (referral,
// win-back, reactivation): the question spans accounts and products, and
// neither sqlc repository owns it.
type AutoHuntRepository struct {
	db *pgxpool.Pool
}

func NewAutoHuntRepository(db *pgxpool.Pool) *AutoHuntRepository {
	return &AutoHuntRepository{db: db}
}

// AccountsForAutoHunt returns accounts with a territory UF and at least one
// product.
//
// Both conditions are the hunt itself, not a preference: without a territory
// there is nowhere to look, and without a catalogue there is nothing to look
// FOR — the words come from the product names. An account missing either gets
// no leads and no error, which is the honest answer.
func (r *AutoHuntRepository) AccountsForAutoHunt(ctx context.Context) ([]domain.SignalWatchCandidate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT a.id, UPPER(a.territory->>'uf')
		FROM accounts a
		WHERE a.territory->>'uf' IS NOT NULL AND a.territory->>'uf' <> ''
		  AND a.suspended_at IS NULL
		  AND EXISTS (SELECT 1 FROM products p WHERE p.account_id = a.id AND p.active)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.SignalWatchCandidate
	for rows.Next() {
		var c domain.SignalWatchCandidate
		if err := rows.Scan(&c.AccountID, &c.UF); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// HuntCursor is how far into its territory this account's sweep has read.
// A missing row is 0 — a first night starts at the top, which is where the
// biggest parks are.
func (r *AutoHuntRepository) HuntCursor(ctx context.Context, accountID string) (int, error) {
	var offset int
	err := r.db.QueryRow(ctx,
		`SELECT next_offset FROM auto_hunt_cursors WHERE account_id = $1`, accountID).Scan(&offset)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	if offset < 0 {
		return 0, nil
	}
	return offset, nil
}

// SetHuntCursor records where the next night should start.
func (r *AutoHuntRepository) SetHuntCursor(ctx context.Context, accountID string, offset int) error {
	if offset < 0 {
		offset = 0
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO auto_hunt_cursors (account_id, next_offset, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (account_id) DO UPDATE
		SET next_offset = EXCLUDED.next_offset, updated_at = NOW()`, accountID, offset)
	return err
}
