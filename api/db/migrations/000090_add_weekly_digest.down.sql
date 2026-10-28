DROP INDEX IF EXISTS idx_accounts_weekly_digest_due;

ALTER TABLE accounts
    DROP COLUMN IF EXISTS weekly_digest_last_sent_at,
    DROP COLUMN IF EXISTS weekly_digest_opt_out;
