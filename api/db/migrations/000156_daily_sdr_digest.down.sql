-- ALLOW-DESTRUCTIVE: drops the columns this migration added.
DROP INDEX IF EXISTS idx_accounts_sdr_digest_due;
ALTER TABLE accounts
    DROP COLUMN IF EXISTS sdr_digest_last_sent_at,
    DROP COLUMN IF EXISTS sdr_digest_opt_out;
