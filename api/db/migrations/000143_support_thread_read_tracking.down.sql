ALTER TABLE conversations
    DROP COLUMN IF EXISTS support_last_read_at,
    DROP COLUMN IF EXISTS support_fallback_email_at;
