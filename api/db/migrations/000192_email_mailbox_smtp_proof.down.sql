-- Dropping this re-blocks every mailbox that was proven by a test send
-- rather than by DNS: cold sending returns to requiring our own records.
ALTER TABLE email_mailboxes
    DROP COLUMN IF EXISTS smtp_verified_at;
