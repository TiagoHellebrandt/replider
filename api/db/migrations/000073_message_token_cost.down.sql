DROP INDEX IF EXISTS idx_conversations_account_cost;
ALTER TABLE conversations DROP COLUMN IF EXISTS total_cost_usd;
ALTER TABLE messages DROP COLUMN IF EXISTS provider_cost_usd;
ALTER TABLE messages DROP COLUMN IF EXISTS output_tokens;
ALTER TABLE messages DROP COLUMN IF EXISTS input_tokens;
