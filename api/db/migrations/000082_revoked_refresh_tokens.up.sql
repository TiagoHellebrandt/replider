-- US-082: durable token revocation list.
--
-- The Redis-backed token blacklist (cache.TokenBlacklist) is the fast
-- path for logout, but it is OPTIONAL — when Redis is unavailable the
-- API disables revocation entirely, so a "logged out" token keeps
-- working until it expires (up to 24h). This table is the durable
-- source of truth that survives a cache flush/restart: a revoked token
-- identifier (jti) stays rejected for its full remaining lifetime.
--
-- `jti` holds a stable per-token identifier (a SHA-256 hash of the
-- bearer token) so we never persist the live credential itself.
-- `expires_at` mirrors the token's own exp claim so reads can ignore
-- entries that have aged out and a janitor can prune them.
CREATE TABLE IF NOT EXISTS revoked_refresh_tokens (
    jti        TEXT PRIMARY KEY,
    revoked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

-- Supports the periodic prune of aged-out entries.
CREATE INDEX IF NOT EXISTS idx_revoked_refresh_tokens_expires_at
    ON revoked_refresh_tokens (expires_at);
