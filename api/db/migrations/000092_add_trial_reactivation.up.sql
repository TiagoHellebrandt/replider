-- US-181: Reactivation emails for lapsed trials.
-- Tracks the last reactivation milestone (days after trial expiry: 7/14/30)
-- emailed to an account so the daily cron is idempotent and never re-sends
-- the same stage. 0 = none sent yet.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS reactivation_stage_sent SMALLINT NOT NULL DEFAULT 0;

-- Partial index over the small set of still-trialing, not-yet-fully-nudged
-- accounts the worker scans — keeps the due-list query cheap as the table
-- grows. Matches the WHERE in ListLapsedTrials.
CREATE INDEX IF NOT EXISTS idx_accounts_reactivation_due
    ON accounts (trial_expires_at)
    WHERE subscription_status = 'trialing'
      AND trial_bypass = false
      AND suspended_at IS NULL
      AND reactivation_stage_sent < 30;
