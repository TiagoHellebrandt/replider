DROP INDEX IF EXISTS idx_api_tokens_account_keys;

ALTER TABLE api_tokens DROP CONSTRAINT IF EXISTS api_tokens_token_type_check;
ALTER TABLE api_tokens ADD CONSTRAINT api_tokens_token_type_check
    CHECK (token_type IN ('oauth_access', 'oauth_refresh', 'pat'));
