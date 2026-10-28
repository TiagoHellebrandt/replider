-- Trial window enforcement. Without an expiry column, trial accounts stay
-- on the trial plan forever and keep consuming tokens we pay the provider
-- for. We set a 14-day window from creation; middleware blocks paid routes
-- once it elapses and subscription_status is still "trialing".
--
-- Backfill: existing trialing accounts get created_at + 14d (some will
-- already be expired, which is fine — they'll be forced to subscribe).
-- Non-trialing accounts (active/past_due/canceled) don't need a trial
-- deadline, so we leave the column NULL for them.
ALTER TABLE accounts
    ADD COLUMN trial_expires_at TIMESTAMPTZ;

UPDATE accounts
SET trial_expires_at = created_at + INTERVAL '14 days'
WHERE subscription_status = 'trialing';
