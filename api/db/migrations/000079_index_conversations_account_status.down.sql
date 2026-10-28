-- Restore the original single-purpose (account_id, status) index, then drop the
-- composite that superseded it.
CREATE INDEX IF NOT EXISTS idx_conversations_account_status ON conversations (account_id, status);
DROP INDEX IF EXISTS idx_conversations_account_status_recent;
