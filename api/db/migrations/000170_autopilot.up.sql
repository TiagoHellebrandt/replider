-- Progressive autopilot (§3.4).
--
-- Human-in-the-loop stays the default. This is the exception for somebody who
-- has already proved, twenty times over, that they approve the drafts
-- unchanged — at which point the queue is not a review, it is a formality.
CREATE TABLE IF NOT EXISTS autopilot_settings (
    account_id      CHAR(26)    PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    enabled         BOOLEAN     NOT NULL DEFAULT FALSE,
    -- Bounds a bad day. Even a perfect model with a wrong prompt only gets to
    -- be wrong this many times before a human has to look.
    daily_cap       INTEGER     NOT NULL DEFAULT 20,
    -- Autopilot is for the easy half of the queue; the rest deserves a person.
    min_score       INTEGER     NOT NULL DEFAULT 80,
    -- Consecutive approvals sent EXACTLY as drafted. Any edit or discard
    -- resets it: the streak is evidence the model writes what this person
    -- would have written, and an edit is that evidence expiring.
    unedited_streak INTEGER     NOT NULL DEFAULT 0,
    -- The offer is made once and not nagged.
    offered_at      TIMESTAMPTZ,
    enabled_at      TIMESTAMPTZ,
    enabled_by      CHAR(26),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Everything autopilot sends, kept for audit. It is the only way an operator
-- can answer "what did it do while I was away?".
CREATE TABLE IF NOT EXISTS autopilot_sends (
    id         CHAR(26)    PRIMARY KEY,
    account_id CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    lead_id    CHAR(26)    NOT NULL,
    recipient  TEXT        NOT NULL,
    text       TEXT        NOT NULL,
    lead_score INTEGER     NOT NULL DEFAULT 0,
    sent_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Backs the daily cap, which is counted from these rows rather than from a
-- counter column: a counter that drifts silently uncaps the feature.
CREATE INDEX IF NOT EXISTS idx_autopilot_sends_account_day
    ON autopilot_sends (account_id, sent_at DESC);
