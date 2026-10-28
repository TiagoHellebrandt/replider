DROP INDEX IF EXISTS idx_messages_wa_conversation;

ALTER TABLE messages
    DROP COLUMN IF EXISTS pricing_billable,
    DROP COLUMN IF EXISTS pricing_category,
    DROP COLUMN IF EXISTS conversation_origin,
    DROP COLUMN IF EXISTS wa_conversation_expires_at,
    DROP COLUMN IF EXISTS wa_conversation_id,
    DROP COLUMN IF EXISTS failed_reason,
    DROP COLUMN IF EXISTS delivered_at;
