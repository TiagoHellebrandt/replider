DROP INDEX IF EXISTS idx_conversations_reminder_pending;

ALTER TABLE conversations
    DROP COLUMN IF EXISTS reply_reminder_last_at,
    DROP COLUMN IF EXISTS reply_reminder_sent_count;

ALTER TABLE accounts
    DROP COLUMN IF EXISTS reply_reminder_threshold_minutes,
    DROP COLUMN IF EXISTS reply_reminder_enabled;
