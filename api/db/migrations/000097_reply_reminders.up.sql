-- Reply-reminder feature: opt-in per-account safety net so an attendant
-- doing manual takeover doesn't forget a pending customer message. The
-- worker scans conversations whose `waiting_since` has been NON-NULL
-- past `reply_reminder_threshold_minutes` and fires push/email/
-- dashboard alerts on a 30m / 1h / 2h escalation ladder.
--
-- Two layers:
--   accounts.* — global per-tenant toggle + threshold
--   conversations.* — per-conversation counters so a reminder cycle
--                     doesn't refire on every worker tick.

-- IF NOT EXISTS guards (migration-renumber divergence from the PR #33
-- merge): on production these columns/index already exist because main's
-- old 000070_reply_reminders ran at version 70 before this file was
-- renumbered to 97; the guards make this a no-op there while still creating
-- them on a fresh DB.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS reply_reminder_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS reply_reminder_threshold_minutes INT NOT NULL DEFAULT 30
        CHECK (reply_reminder_threshold_minutes BETWEEN 1 AND 1440);

ALTER TABLE conversations
    ADD COLUMN IF NOT EXISTS reply_reminder_sent_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS reply_reminder_last_at TIMESTAMPTZ;

-- Worker query filter: WHERE waiting_since IS NOT NULL AND
-- reply_reminder_sent_count < N. Hot path so we keep a partial index
-- aligned with the existing waiting_since one.
CREATE INDEX IF NOT EXISTS idx_conversations_reminder_pending
    ON conversations(waiting_since, reply_reminder_last_at)
    WHERE waiting_since IS NOT NULL AND reply_reminder_sent_count < 3;
