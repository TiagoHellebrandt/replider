ALTER TABLE messages
    DROP COLUMN IF EXISTS sent_at,
    DROP COLUMN IF EXISTS failed_at;
