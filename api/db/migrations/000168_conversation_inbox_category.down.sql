DROP INDEX IF EXISTS idx_conversations_inbox;
ALTER TABLE conversations
    DROP COLUMN IF EXISTS inbox_category,
    DROP COLUMN IF EXISTS inbox_importance;
