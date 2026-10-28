-- Actionable daily digest (US-M010).
--
-- One stamp per account, in the same shape as the weekly retention digest
-- (000090): the due-query excludes anything already sent today, so a
-- process restart re-running the tick cannot double-send. Idempotency
-- lives in the DB, not in worker memory, because worker memory is exactly
-- what a restart loses.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS sdr_digest_last_sent_at TIMESTAMPTZ,
    -- Per-account opt-out. A digest nobody wants trains people to ignore
    -- notifications from us in general, which costs more than the digest
    -- is worth.
    ADD COLUMN IF NOT EXISTS sdr_digest_opt_out BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_accounts_sdr_digest_due
    ON accounts (sdr_digest_last_sent_at)
    WHERE sdr_digest_opt_out = FALSE;
