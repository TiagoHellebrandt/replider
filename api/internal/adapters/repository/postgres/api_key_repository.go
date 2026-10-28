package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/replider/api/internal/core/domain"
)

// Machine API keys (§6.19, J1).
//
// A raw-SQL sibling of the sqlc-generated token repository, deliberately
// separate: the key reads are ACCOUNT-scoped and the generated ones are all
// user-scoped, so adding them there would mean regenerating for four statements
// that share nothing with the PAT queries. Same table, same hash column — this
// only reads and revokes.
type APIKeyRepository struct {
	db *pgxpool.Pool
}

func NewAPIKeyRepository(db *pgxpool.Pool) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

var _ domain.APIKeyRepository = (*APIKeyRepository)(nil)

// ListByAccount returns the account's live keys, never the hashes.
func (r *APIKeyRepository) ListByAccount(ctx context.Context, accountID string) ([]*domain.APIKeyInfo, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, token_prefix, scope, created_at, last_used_at, expires_at
		 FROM api_tokens
		 WHERE account_id = $1 AND token_type = $2 AND revoked_at IS NULL
		 ORDER BY created_at DESC`,
		accountID, domain.APITokenTypeAPIKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*domain.APIKeyInfo{}
	for rows.Next() {
		var key domain.APIKeyInfo
		var scope string
		if err := rows.Scan(&key.ID, &key.Name, &key.TokenPrefix, &scope,
			&key.CreatedAt, &key.LastUsedAt, &key.ExpiresAt); err != nil {
			return nil, err
		}
		key.Scopes = domain.DecodeAPIScopes(scope)
		out = append(out, &key)
	}
	return out, rows.Err()
}

// CountActive backs the per-account ceiling.
func (r *APIKeyRepository) CountActive(ctx context.Context, accountID string) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM api_tokens
		 WHERE account_id = $1 AND token_type = $2 AND revoked_at IS NULL
		   AND (expires_at IS NULL OR expires_at > NOW())`,
		accountID, domain.APITokenTypeAPIKey).Scan(&count)
	return count, err
}

// Revoke kills one key.
//
// Account-scoped in the statement, not checked beforehand: this is reachable
// from an HTTP route, and another tenant's key id must match zero rows rather
// than depend on a caller remembering. Returns whether anything was revoked, so
// the handler can answer 404 instead of a success that did nothing.
func (r *APIKeyRepository) Revoke(ctx context.Context, accountID, id string) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE api_tokens SET revoked_at = NOW()
		 WHERE id = $1 AND account_id = $2 AND token_type = $3 AND revoked_at IS NULL`,
		id, accountID, domain.APITokenTypeAPIKey)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ResolveKey authenticates a presented key.
//
// Returns (nil, nil) for anything that is not a live machine key: revoked,
// expired, the wrong type, or simply not there. One answer for all of them, so a
// caller cannot distinguish "revoked" from "never existed" by timing or message —
// which is the difference between a 401 and an oracle.
func (r *APIKeyRepository) ResolveKey(ctx context.Context, tokenHash string) (*domain.ResolvedAPIKey, error) {
	var resolved domain.ResolvedAPIKey
	var scope string
	err := r.db.QueryRow(ctx,
		`SELECT t.id, t.account_id, t.name, t.scope
		 FROM api_tokens t
		 JOIN accounts a ON a.id = t.account_id
		 WHERE t.token_hash = $1
		   AND t.token_type = $2
		   AND t.revoked_at IS NULL
		   AND (t.expires_at IS NULL OR t.expires_at > NOW())
		   -- A suspended account's key stops working with the account, not a
		   -- billing cycle later.
		   AND a.suspended_at IS NULL`,
		tokenHash, domain.APITokenTypeAPIKey,
	).Scan(&resolved.KeyID, &resolved.AccountID, &resolved.Name, &scope)
	if err != nil {
		// pgx.ErrNoRows and any other read failure both mean "not
		// authenticated" to the caller; the error is returned so the middleware
		// can tell a database problem from a bad key in the logs.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	resolved.Scopes = domain.DecodeAPIScopes(scope)
	return &resolved, nil
}

// TouchLastUsed records that the key was used. Best-effort: an integration must
// not fail because the bookkeeping write did.
func (r *APIKeyRepository) TouchLastUsed(ctx context.Context, keyID string) {
	_, _ = r.db.Exec(ctx,
		`UPDATE api_tokens SET last_used_at = NOW()
		 WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < NOW() - interval '1 minute')`,
		keyID)
}
