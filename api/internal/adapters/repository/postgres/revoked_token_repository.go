package postgres

import (
	"context"
	"time"
)

// RevokedTokenRepository is the durable, Postgres-backed implementation
// of domain.RevokedTokenStore (US-082). It persists revoked token
// identifiers so logout survives a Redis flush/restart — the cache is a
// fast path, this table is the source of truth.
type RevokedTokenRepository struct {
	db DBTX
}

func NewRevokedTokenRepository(db DBTX) *RevokedTokenRepository {
	return &RevokedTokenRepository{db: db}
}

// Add records jti as revoked until expiresAt. ON CONFLICT DO NOTHING
// makes re-revoking the same token a harmless no-op (the first revoke's
// expiry stands).
func (r *RevokedTokenRepository) Add(ctx context.Context, jti string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO revoked_refresh_tokens (jti, expires_at)
		 VALUES ($1, $2)
		 ON CONFLICT (jti) DO NOTHING`,
		jti, expiresAt,
	)
	return err
}

// Exists reports whether jti is revoked and has not yet aged out. The
// expires_at guard means a stale row that hasn't been pruned yet never
// causes a false rejection — it behaves exactly like an expired cache
// entry.
func (r *RevokedTokenRepository) Exists(ctx context.Context, jti string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM revoked_refresh_tokens
			WHERE jti = $1 AND expires_at > now()
		)`,
		jti,
	).Scan(&exists)
	return exists, err
}

// PruneExpired deletes entries whose expiry has passed and returns the
// number removed.
func (r *RevokedTokenRepository) PruneExpired(ctx context.Context, now time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM revoked_refresh_tokens WHERE expires_at <= $1`,
		now,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
