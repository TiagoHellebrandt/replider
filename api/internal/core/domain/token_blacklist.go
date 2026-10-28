package domain

import (
	"context"
	"time"
)

// TokenBlacklist provides a mechanism to revoke JWT tokens before their
// natural expiration. When a user logs out, their token is added to the
// blacklist with a TTL equal to the remaining token lifetime. The auth
// middleware checks this blacklist on every request.
type TokenBlacklist interface {
	// Revoke adds a token to the blacklist. The token should be
	// identified by a unique claim (e.g. jti or the token string itself).
	// The TTL should be the remaining time until the token expires.
	Revoke(ctx context.Context, tokenID string, ttl time.Duration) error

	// IsRevoked returns true if the token has been revoked (is in the
	// blacklist). Returns false if the token is valid or if the blacklist
	// is unavailable (fail-open to avoid locking out all users on Redis
	// downtime).
	IsRevoked(ctx context.Context, tokenID string) bool
}

// RevokedTokenStore is the DURABLE (database-backed) record of token
// identifiers (jti) that were revoked before their natural expiry. It
// backs the in-memory/Redis TokenBlacklist cache: unlike the cache it
// survives a flush or restart, so a logged-out token stays rejected for
// its full remaining lifetime (US-082). Entries carry an expiry so reads
// can ignore aged-out rows and a janitor can prune them.
type RevokedTokenStore interface {
	// Add records jti as revoked until expiresAt. Idempotent — adding the
	// same jti again is a no-op.
	Add(ctx context.Context, jti string, expiresAt time.Time) error

	// Exists returns true if jti is in the list and has not yet aged out.
	Exists(ctx context.Context, jti string) (bool, error)

	// PruneExpired deletes entries whose expiry is at or before now and
	// returns the number removed.
	PruneExpired(ctx context.Context, now time.Time) (int64, error)
}
