-- Cadences (§1.5, E1).
--
-- The single biggest reason a B2B deal dies is that nobody followed up a
-- third time. The second biggest is that somebody followed up eleven times.
CREATE TABLE IF NOT EXISTS sequences (
    id                  CHAR(26)    PRIMARY KEY,
    account_id          CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    funnel_id           CHAR(26),
    name                TEXT        NOT NULL,
    -- The steps ride as JSONB rather than a child table: they are always
    -- read and written as one plan, never queried across cadences, and a
    -- join per step would buy nothing.
    steps               JSONB       NOT NULL DEFAULT '[]',
    -- Defaults TRUE and there is no UI to turn it off. A cadence that keeps
    -- firing after a human answered is a machine talking over somebody.
    stop_on_reply       BOOLEAN     NOT NULL DEFAULT TRUE,
    -- A cold email at 03:14 reads as a machine, which is what it must not.
    business_hours_only BOOLEAN     NOT NULL DEFAULT TRUE,
    is_active           BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sequences_account ON sequences (account_id);

-- One lead walking through one cadence.
CREATE TABLE IF NOT EXISTS sequence_enrollments (
    id           CHAR(26)    PRIMARY KEY,
    account_id   CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    sequence_id  CHAR(26)    NOT NULL REFERENCES sequences(id) ON DELETE CASCADE,
    lead_id      CHAR(26)    NOT NULL,
    -- The NEXT step to run, 1-based. Zero means nothing has run yet.
    current_step INTEGER     NOT NULL DEFAULT 0,
    status       TEXT        NOT NULL DEFAULT 'active',
    -- NULL for a finished or paused enrollment: the worker keys on this, so
    -- a null is what takes a row out of its reach.
    next_run_at  TIMESTAMPTZ,
    enrolled_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at     TIMESTAMPTZ
);

-- A lead may be in at most ONE cadence at a time. Two means two follow-ups a
-- week from the same company, which reads as disorganised at best.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_sequence_enrollment_active_lead
    ON sequence_enrollments (account_id, lead_id)
    WHERE status = 'active';

-- What the worker scans.
CREATE INDEX IF NOT EXISTS idx_sequence_enrollments_due
    ON sequence_enrollments (next_run_at)
    WHERE status = 'active' AND next_run_at IS NOT NULL;

-- Per-step funnel: how many reached each step, and how many replied after it.
CREATE TABLE IF NOT EXISTS sequence_step_events (
    id            CHAR(26)    PRIMARY KEY,
    sequence_id   CHAR(26)    NOT NULL REFERENCES sequences(id) ON DELETE CASCADE,
    enrollment_id CHAR(26)    NOT NULL REFERENCES sequence_enrollments(id) ON DELETE CASCADE,
    step_order    INTEGER     NOT NULL,
    -- 'reached' when the step ran; 'replied' when the lead answered before
    -- the next step. Separate rows so "este passo funciona" is a ratio of
    -- two counted facts rather than an inferred one.
    kind          TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sequence_step_events_seq
    ON sequence_step_events (sequence_id, step_order, kind);

-- One 'reached' and one 'replied' per (enrollment, step). A retried worker
-- pass must not inflate the funnel it is measuring.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_sequence_step_event
    ON sequence_step_events (enrollment_id, step_order, kind);
