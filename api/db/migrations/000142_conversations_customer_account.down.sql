DROP INDEX IF EXISTS idx_conversations_customer_account;
ALTER TABLE conversations DROP COLUMN IF EXISTS customer_account_id;
