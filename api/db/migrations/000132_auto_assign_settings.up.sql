-- US-016: per-account round-robin auto-assignment config.
-- No row = feature enabled with every active attendant participating,
-- which preserves the pre-existing always-on behavior for all accounts.
-- participant_user_ids = '{}' also means "all active attendants"; a
-- non-empty array restricts the rotation to those users (stale ids are
-- filtered against active attendants at assign time, so user removal
-- needs no cleanup here).
CREATE TABLE IF NOT EXISTS auto_assign_settings (
    account_id CHAR(26) PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    participant_user_ids TEXT[] NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
