-- Machine API keys (§6.19, J1).
--
-- Everything the product authenticates today belongs to a PERSON: a 24h user JWT,
-- or a personal access token for one person's MCP client. Neither is a credential
-- a customer's ERP can hold — a JWT expires overnight and a PAT dies with the
-- employee who created it.
--
-- The key rides the SAME table as the PATs: same hash column, same prefix, same
-- revocation, same expiry sweep. Only the discriminator is new, so nothing about
-- authentication or cleanup had to be written twice.
ALTER TABLE api_tokens DROP CONSTRAINT IF EXISTS api_tokens_token_type_check;
ALTER TABLE api_tokens ADD CONSTRAINT api_tokens_token_type_check
    CHECK (token_type IN ('oauth_access', 'oauth_refresh', 'pat', 'api_key'));

-- A machine key belongs to the ACCOUNT, not to the person who happened to press
-- the button. user_id still records WHO created it (that is an audit fact worth
-- keeping), but the key survives them leaving — which is the entire reason it
-- exists.
COMMENT ON COLUMN api_tokens.user_id IS
    'Owner for a PAT; for an api_key, who created it — the key is the account''s and outlives them.';

-- The account-scoped listing, which the PAT indexes (keyed on user) do not serve.
CREATE INDEX IF NOT EXISTS idx_api_tokens_account_keys
    ON api_tokens (account_id)
    WHERE token_type = 'api_key' AND revoked_at IS NULL;
