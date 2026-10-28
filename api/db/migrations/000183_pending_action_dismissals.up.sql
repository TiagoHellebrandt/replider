-- Snoozing an item on "Precisa de você" (§6.13, I2).
--
-- Every action on that list is DERIVED from live state — a draft waiting for
-- approval, a signature viewed and unsigned, a contract about to renew itself.
-- Doing the work makes the row disappear on its own, which is the whole reason
-- the list can be trusted.
--
-- This table is the escape hatch for the other case: an action a human has
-- judged, correctly, as not worth doing. It records a SNOOZE, not a completion —
-- marking work done without doing it would put a lie on the one screen someone
-- opens to decide what to do next.
CREATE TABLE IF NOT EXISTS pending_action_dismissals (
    id         CHAR(26)    PRIMARY KEY,
    account_id CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    -- Per USER, not per account: "eu já vi isso" is one person's judgement, and
    -- hiding a R$40k renewal from a whole company because one attendant swiped
    -- it away is how a deadline gets missed. No FK to users — a dismissal
    -- outliving the person who made it is harmless, and a GDPR deletion must not
    -- take the rest of the table with it.
    user_id    CHAR(26)    NOT NULL,

    -- What was dismissed. Free-form kind + target id rather than a FK per
    -- source table: the action list grows by adding a query, and a new source
    -- must not need a migration to become snoozeable.
    kind       TEXT        NOT NULL,
    target_id  TEXT        NOT NULL,

    -- When it comes back. NOT NULL on purpose: there is no "forever" here. A
    -- contract that renews itself in March is still going to renew itself in
    -- March, and a permanent dismissal would let the product agree to forget
    -- something expensive.
    snoozed_until TIMESTAMPTZ NOT NULL,
    note       TEXT        NOT NULL DEFAULT '',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One live dismissal per (person, thing). Re-snoozing moves the deadline
-- instead of stacking rows.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_pending_action_dismissal
    ON pending_action_dismissals (account_id, user_id, kind, target_id);

-- The filter read: one person's live snoozes.
CREATE INDEX IF NOT EXISTS idx_pending_action_dismissals_live
    ON pending_action_dismissals (account_id, user_id, snoozed_until);
