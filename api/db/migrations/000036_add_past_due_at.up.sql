-- Tracks when an account entered past_due status. Used to enforce
-- a 7-day grace period before suspending AI access.
ALTER TABLE accounts ADD COLUMN past_due_at TIMESTAMPTZ;
