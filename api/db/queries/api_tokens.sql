-- name: CreateAPIToken :exec
INSERT INTO api_tokens (
    id, token_hash, token_type, user_id, account_id, client_id, family_id, name, token_prefix, scope, expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
);

-- name: ResolveAPIToken :one
-- The MCP auth middleware's single lookup: bearer hash -> identity.
-- Only access tokens and PATs authenticate requests; refresh tokens
-- are exchange-only and deliberately excluded.
SELECT t.id, t.token_type, t.user_id, t.account_id, t.scope, t.family_id,
       u.name AS user_name, u.role, u.is_active
FROM api_tokens t
JOIN users u ON u.id = t.user_id
WHERE t.token_hash = $1
  AND t.token_type IN ('oauth_access', 'pat')
  AND t.revoked_at IS NULL
  AND (t.expires_at IS NULL OR t.expires_at > NOW());

-- name: GetRefreshTokenByHash :one
SELECT * FROM api_tokens
WHERE token_hash = $1 AND token_type = 'oauth_refresh';

-- name: RevokeToken :exec
UPDATE api_tokens SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeTokenByHash :exec
UPDATE api_tokens SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL;

-- name: RevokeTokenFamily :exec
UPDATE api_tokens SET revoked_at = NOW() WHERE family_id = $1 AND revoked_at IS NULL;

-- name: ListPATsByUser :many
SELECT id, name, token_prefix, expires_at, last_used_at, created_at
FROM api_tokens
WHERE user_id = $1 AND token_type = 'pat' AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: RevokePATByIDAndUser :execrows
UPDATE api_tokens SET revoked_at = NOW()
WHERE id = $1 AND user_id = $2 AND token_type = 'pat' AND revoked_at IS NULL;

-- name: CountActivePATsByUser :one
SELECT COUNT(*) FROM api_tokens
WHERE user_id = $1 AND token_type = 'pat' AND revoked_at IS NULL;

-- name: ListOAuthConnectionsByUser :many
-- Dashboard "conexões ativas": one row per client the user granted,
-- summarized across active token families.
SELECT t.client_id, c.client_name, c.logo_uri,
       MIN(t.created_at)::timestamptz AS first_granted_at,
       MAX(t.last_used_at)::timestamptz AS last_used_at
FROM api_tokens t
JOIN oauth_clients c ON c.id = t.client_id
WHERE t.user_id = $1
  AND t.token_type IN ('oauth_access', 'oauth_refresh')
  AND t.revoked_at IS NULL
  AND (t.expires_at IS NULL OR t.expires_at > NOW())
GROUP BY t.client_id, c.client_name, c.logo_uri
ORDER BY MAX(t.created_at) DESC;

-- name: RevokeOAuthConnection :execrows
UPDATE api_tokens SET revoked_at = NOW()
WHERE user_id = $1 AND client_id = $2
  AND token_type IN ('oauth_access', 'oauth_refresh')
  AND revoked_at IS NULL;

-- name: TouchAPITokenLastUsed :exec
-- Throttled like TrackActivity: avoids one write per MCP request.
UPDATE api_tokens SET last_used_at = NOW()
WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < NOW() - INTERVAL '5 minutes');

-- name: DeleteExpiredAPITokens :exec
-- PATs (expires_at NULL unless user-set) are kept; expired/revoked OAuth
-- tokens are purged after a grace window for auditability.
DELETE FROM api_tokens
WHERE token_type IN ('oauth_access', 'oauth_refresh')
  AND (
    (expires_at IS NOT NULL AND expires_at < NOW() - INTERVAL '30 days')
    OR (revoked_at IS NOT NULL AND revoked_at < NOW() - INTERVAL '30 days')
  );
