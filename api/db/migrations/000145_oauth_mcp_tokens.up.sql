-- Remote MCP auth: OAuth 2.1 authorization server (DCR clients, auth codes)
-- plus unified bearer tokens (OAuth access/refresh + personal access tokens).

CREATE TABLE IF NOT EXISTS oauth_clients (
    id CHAR(26) PRIMARY KEY,
    client_secret_hash CHAR(64),
    token_endpoint_auth_method VARCHAR(32) NOT NULL DEFAULT 'none',
    client_name TEXT NOT NULL DEFAULT '',
    redirect_uris JSONB NOT NULL,
    grant_types JSONB NOT NULL DEFAULT '["authorization_code","refresh_token"]',
    scope TEXT NOT NULL DEFAULT 'mcp',
    logo_uri TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS oauth_authorization_codes (
    code_hash CHAR(64) PRIMARY KEY,
    client_id CHAR(26) NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id CHAR(26) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    redirect_uri TEXT NOT NULL,
    scope TEXT NOT NULL DEFAULT 'mcp',
    code_challenge VARCHAR(128) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS api_tokens (
    id CHAR(26) PRIMARY KEY,
    token_hash CHAR(64) NOT NULL UNIQUE,
    token_type VARCHAR(16) NOT NULL CHECK (token_type IN ('oauth_access', 'oauth_refresh', 'pat')),
    user_id CHAR(26) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    client_id CHAR(26) REFERENCES oauth_clients(id) ON DELETE CASCADE,
    family_id CHAR(26),
    name TEXT NOT NULL DEFAULT '',
    token_prefix VARCHAR(16) NOT NULL DEFAULT '',
    scope TEXT NOT NULL DEFAULT 'mcp',
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_tokens_user ON api_tokens (user_id) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_api_tokens_family ON api_tokens (family_id) WHERE family_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_oauth_codes_expires ON oauth_authorization_codes (expires_at);
