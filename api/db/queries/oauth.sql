-- name: CreateOAuthClient :exec
INSERT INTO oauth_clients (
    id, client_secret_hash, token_endpoint_auth_method, client_name, redirect_uris, grant_types, scope, logo_uri
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
);

-- name: GetOAuthClient :one
SELECT * FROM oauth_clients WHERE id = $1;

-- name: TouchOAuthClientLastUsed :exec
UPDATE oauth_clients SET last_used_at = NOW() WHERE id = $1;

-- name: CreateAuthorizationCode :exec
INSERT INTO oauth_authorization_codes (
    code_hash, client_id, user_id, account_id, redirect_uri, scope, code_challenge, expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
);

-- name: ConsumeAuthorizationCode :one
-- Atomic single-use consume: the WHERE clause guarantees a code can only
-- ever be exchanged once, even under concurrent token requests.
UPDATE oauth_authorization_codes
SET used_at = NOW()
WHERE code_hash = $1 AND used_at IS NULL AND expires_at > NOW()
RETURNING client_id, user_id, account_id, redirect_uri, scope, code_challenge;

-- name: GetAuthorizationCodeByHash :one
-- Used to detect replay of an already-consumed code (OAuth 2.1 requires
-- revoking every token issued from that code on reuse).
SELECT * FROM oauth_authorization_codes WHERE code_hash = $1;

-- name: DeleteExpiredAuthorizationCodes :exec
DELETE FROM oauth_authorization_codes WHERE expires_at < NOW() - INTERVAL '1 hour';
