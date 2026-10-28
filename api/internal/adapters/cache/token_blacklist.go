package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const tokenBlacklistPrefix = "revoked:"

// TokenBlacklist implements domain.TokenBlacklist using Redis.
//
// Revoked tokens are stored with a key of "revoked:<tokenHash>" and a
// TTL equal to the remaining token lifetime. This ensures automatic
// cleanup — once the token would have expired naturally, the blacklist
// entry disappears too.
type TokenBlacklist struct {
	cache *RedisCache
}

// NewTokenBlacklist creates a new Redis-backed token blacklist.
func NewTokenBlacklist(cache *RedisCache) *TokenBlacklist {
	return &TokenBlacklist{cache: cache}
}

func (b *TokenBlacklist) Revoke(ctx context.Context, tokenID string, ttl time.Duration) error {
	key := fmt.Sprintf("%s%s", tokenBlacklistPrefix, tokenID)
	return b.cache.Set(ctx, key, true, ttl)
}

func (b *TokenBlacklist) IsRevoked(ctx context.Context, tokenID string) bool {
	key := fmt.Sprintf("%s%s", tokenBlacklistPrefix, tokenID)
	exists, err := b.cache.Exists(ctx, key)
	if err != nil {
		// Fail-open: if Redis is down, don't lock out all users.
		// The token will still expire naturally via JWT exp claim.
		slog.Warn("token blacklist check failed, allowing request", "error", err)
		return false
	}
	return exists
}
