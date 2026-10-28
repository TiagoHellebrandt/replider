DROP INDEX IF EXISTS idx_conversations_first_response_by;
ALTER TABLE conversations
    DROP COLUMN IF EXISTS first_response_by,
    DROP COLUMN IF EXISTS first_inbound_at;
