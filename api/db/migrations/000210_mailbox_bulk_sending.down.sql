ALTER TABLE email_mailboxes
    DROP COLUMN IF EXISTS bulk_domain_id,
    DROP COLUMN IF EXISTS bulk_verified_at;
