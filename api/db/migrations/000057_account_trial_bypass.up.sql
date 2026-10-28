-- Internal / reviewer accounts can bypass trial-expiration checks
-- without an active Stripe subscription. Used for:
--   * The owner's own account (so testing the product doesn't burn a
--     paid plan against ourselves).
--   * App Reviewer accounts handed to Meta during App Review (Meta
--     reviewers must be able to drive the full happy path without
--     payment).
--
-- The flag is admin-only (no self-serve UI) and the audit_log
-- captures every flip so it can't be abused silently. Stripe billing
-- still applies normally — bypass only short-circuits the
-- RequireActiveSubscription middleware, NOT the plan limit checks.
ALTER TABLE accounts
    ADD COLUMN trial_bypass BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN trial_bypass_reason TEXT;

CREATE INDEX idx_accounts_trial_bypass
    ON accounts(trial_bypass)
    WHERE trial_bypass = TRUE;
