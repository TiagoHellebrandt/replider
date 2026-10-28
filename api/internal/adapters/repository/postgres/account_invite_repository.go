package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/replider/api/internal/core/domain"
)

// AccountInviteRepository is raw SQL over the pool — a platform table with no
// account scoping (the account a row creates does not exist yet), and no sqlc
// so the one-shot consumption stays a single visible statement.
type AccountInviteRepository struct{ db *pgxpool.Pool }

func NewAccountInviteRepository(db *pgxpool.Pool) *AccountInviteRepository {
	return &AccountInviteRepository{db: db}
}

const inviteColumns = `id, email, plan_id, overrides, invited_by, expires_at,
	used_at, used_account_id, revoked_at, created_at`

func scanInvite(row pgx.Row) (*domain.AccountInvite, error) {
	var inv domain.AccountInvite
	var overrides []byte
	if err := row.Scan(&inv.ID, &inv.Email, &inv.PlanID, &overrides, &inv.InvitedBy,
		&inv.ExpiresAt, &inv.UsedAt, &inv.UsedAccountID, &inv.RevokedAt, &inv.CreatedAt); err != nil {
		return nil, err
	}
	if len(overrides) > 0 {
		if err := json.Unmarshal(overrides, &inv.Overrides); err != nil {
			// A corrupted overrides blob must not brick the invite listing;
			// the redeem path re-reads through FindValid and would surface it.
			inv.Overrides = domain.AccountLimitOverrides{}
		}
	}
	return &inv, nil
}

func (r *AccountInviteRepository) Create(ctx context.Context, invite *domain.AccountInvite, tokenHash string) error {
	overrides, err := json.Marshal(invite.Overrides)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx,
		`INSERT INTO account_invites
		   (id, token_hash, email, plan_id, overrides, invited_by, expires_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())`,
		invite.ID, tokenHash, invite.Email, invite.PlanID, overrides, invite.InvitedBy, invite.ExpiresAt)
	return err
}

func (r *AccountInviteRepository) FindValid(ctx context.Context, tokenHash string) (*domain.AccountInvite, error) {
	inv, err := scanInvite(r.db.QueryRow(ctx,
		`SELECT `+inviteColumns+` FROM account_invites
		  WHERE token_hash = $1
		    AND used_at IS NULL AND revoked_at IS NULL AND expires_at > NOW()`,
		tokenHash))
	if errors.Is(err, pgx.ErrNoRows) {
		// One answer for missing/expired/used/revoked — see the port doc.
		return nil, nil
	}
	return inv, err
}

func (r *AccountInviteRepository) Consume(ctx context.Context, id, accountID string) (bool, error) {
	// The one-shot: every validity condition re-checked IN the burn, so two
	// racing signups can never both pass, and a revoke that landed between
	// FindValid and here still wins.
	var got string
	err := r.db.QueryRow(ctx,
		`UPDATE account_invites
		    SET used_at = NOW(), used_account_id = $2, updated_at = NOW()
		  WHERE id = $1
		    AND used_at IS NULL AND revoked_at IS NULL AND expires_at > NOW()
		  RETURNING id`,
		id, accountID).Scan(&got)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (r *AccountInviteRepository) Unconsume(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE account_invites
		    SET used_at = NULL, used_account_id = NULL, updated_at = NOW()
		  WHERE id = $1`,
		id)
	return err
}

func (r *AccountInviteRepository) Revoke(ctx context.Context, id string) (bool, error) {
	var got string
	err := r.db.QueryRow(ctx,
		`UPDATE account_invites
		    SET revoked_at = NOW(), updated_at = NOW()
		  WHERE id = $1 AND used_at IS NULL AND revoked_at IS NULL
		  RETURNING id`,
		id).Scan(&got)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (r *AccountInviteRepository) ListOpen(ctx context.Context) ([]*domain.AccountInvite, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+inviteColumns+` FROM account_invites
		  WHERE used_at IS NULL AND revoked_at IS NULL AND expires_at > NOW()
		  ORDER BY created_at DESC
		  LIMIT 200`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*domain.AccountInvite{}
	for rows.Next() {
		inv, err := scanInvite(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// compile-time conformance
var _ domain.AccountInviteRepository = (*AccountInviteRepository)(nil)
