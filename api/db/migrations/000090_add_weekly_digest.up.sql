-- US-176: Weekly digest email (retention).
-- Per-account opt-out + a "last sent" stamp so the cron is idempotent
-- and only fires once per week per account.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS weekly_digest_opt_out      BOOLEAN     NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS weekly_digest_last_sent_at TIMESTAMPTZ;

-- Partial index over the small set of accounts that still receive the
-- digest — keeps the worker's due-list scan cheap as the table grows.
CREATE INDEX IF NOT EXISTS idx_accounts_weekly_digest_due
    ON accounts (weekly_digest_last_sent_at)
    WHERE weekly_digest_opt_out = false AND suspended_at IS NULL;
