-- US-182: Onboarding nudge emails (D1/D3/D7).
-- After signup we drip activation nudges to accounts that haven't finished
-- onboarding: D1 "connect a channel", D3 "add knowledge", D7 "share your
-- link". onboarding_nudge_stage_sent is the last day-milestone emailed (or
-- skipped because the step was already done) — 0 = none — so the daily
-- worker is idempotent and never re-sends a stage.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS onboarding_nudge_stage_sent SMALLINT NOT NULL DEFAULT 0;

-- Partial index over the small set of recently-created, not-yet-fully-nudged
-- accounts the worker scans. Matches the WHERE in ListDueForNudge.
CREATE INDEX IF NOT EXISTS idx_accounts_onboarding_nudge_due
    ON accounts (created_at)
    WHERE onboarding_nudge_stage_sent < 7;
