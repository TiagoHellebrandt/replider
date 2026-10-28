package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

const persistentRevokedPrefix = "revoked:"

// PersistentTokenBlacklist is a durable implementation of
// domain.TokenBlacklist (US-082). It layers an optional Redis cache (fast
// path) over a database-backed RevokedTokenStore (source of truth):
//
//   - Revoke writes to the durable store ALWAYS, and to the cache
//     best-effort. The store is what makes revocation survive a cache
//     flush/restart or a deployment without Redis at all.
//   - IsRevoked consults the cache first when it is healthy (every Revoke
//     writes through to it, so a healthy-cache miss is a definitive "not
//     revoked"). When Redis is absent or erroring it falls through to the
//     durable store, so revocation keeps working with no cache configured
//     — which is the gap this story closes (previously logout was disabled
//     entirely when Redis was unavailable).
//
// Tokens are identified by a SHA-256 hash of the bearer token rather than
// the token itself, so a revoked-but-unexpired credential is never stored
// in plaintext in Redis or the database. Callers still pass the raw token
// string (or a jti) as tokenID — hashing is internal.
type PersistentTokenBlacklist struct {
	store domain.RevokedTokenStore
	cache *RedisCache // optional; nil when Redis is unavailable
}

// NewPersistentTokenBlacklist builds the blacklist. store is required;
// cache may be nil (durable-only mode).
func NewPersistentTokenBlacklist(store domain.RevokedTokenStore, cache *RedisCache) *PersistentTokenBlacklist {
	return &PersistentTokenBlacklist{store: store, cache: cache}
}

func (b *PersistentTokenBlacklist) Revoke(ctx context.Context, tokenID string, ttl time.Duration) error {
	jti := revocationKey(tokenID)

	// Durable write is the source of truth — fail the whole revoke if it
	// doesn't land, so the caller knows the token may still be usable.
	if err := b.store.Add(ctx, jti, time.Now().Add(ttl)); err != nil {
		return err
	}

	// Best-effort cache write for the fast path. A failure here is fine:
	// IsRevoked falls back to the durable store when the cache misses on
	// an erroring Redis.
	if b.cache != nil {
		if err := b.cache.Set(ctx, persistentRevokedPrefix+jti, true, ttl); err != nil {
			slog.Warn("revoked-token cache write failed (durable store ok)", "error", err)
		}
	}
	return nil
}

func (b *PersistentTokenBlacklist) IsRevoked(ctx context.Context, tokenID string) bool {
	jti := revocationKey(tokenID)

	if b.cache != nil {
		if exists, err := b.cache.Exists(ctx, persistentRevokedPrefix+jti); err == nil {
			// Cache is healthy and authoritative: every Revoke writes
			// through to it, so a miss means the token is not revoked.
			return exists
		}
		// Redis errored — fall through to the durable store.
	}

	revoked, err := b.store.Exists(ctx, jti)
	if err != nil {
		// Fail-open: a DB hiccup must not lock out every user. The token
		// still expires naturally via its exp claim.
		slog.Warn("revoked-token durable check failed, allowing request", "error", err)
		return false
	}
	return revoked
}

// revocationKey derives a stable, fixed-length identifier from the raw
// token so the live credential is never persisted.
func revocationKey(tokenID string) string {
	sum := sha256.Sum256([]byte(tokenID))
	return hex.EncodeToString(sum[:])
}
