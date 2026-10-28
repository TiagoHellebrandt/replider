-- Overnight drafting (§6.17, K2).
--
-- `draft_approaches_batch` is not real time. Nobody is watching the queue fill;
-- what a salesperson wants is to open the app at eight and find the approaches
-- already written. Which means the work can run when nothing else is competing
-- for the provider — and, where the provider offers a batch endpoint, at roughly
-- half the price.
CREATE TABLE IF NOT EXISTS overnight_draft_settings (
    account_id CHAR(26) PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,

    -- Opt-in, not opt-out. This spends the account's AI allowance without anyone
    -- asking for it that night, so it has to be a decision somebody made.
    enabled    BOOLEAN NOT NULL DEFAULT FALSE,

    -- The funnel to draft for. NULL means every active funnel — the common case
    -- for an account with one.
    funnel_id  CHAR(26),

    -- How many to draft per night. Bounded by the plan's monthly AI-draft
    -- allowance (K4) on top of this, so a generous nightly number cannot quietly
    -- spend the month in a week.
    nightly_cap INTEGER NOT NULL DEFAULT 40 CHECK (nightly_cap > 0),

    -- The hour the run starts, in the ACCOUNT's timezone. 3am by default: after
    -- the working day everywhere in Brazil, well before anyone opens the app.
    run_hour   SMALLINT NOT NULL DEFAULT 3
               CHECK (run_hour >= 0 AND run_hour <= 23),

    -- Guards against a worker tick running the same night twice. Compared
    -- against the account's local date, never against UTC's.
    last_run_on DATE,

    enabled_by CHAR(26),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The worker's read: who is due tonight.
CREATE INDEX IF NOT EXISTS idx_overnight_draft_enabled
    ON overnight_draft_settings (enabled, last_run_on)
    WHERE enabled;

-- What each night produced.
--
-- A row per RUN, not per draft: the morning briefing is one sentence ("seu SDR
-- trabalhou de madrugada: 40 abordagens prontas"), and the drafts themselves are
-- already in the approval queue where they belong.
CREATE TABLE IF NOT EXISTS overnight_draft_runs (
    id         CHAR(26)    PRIMARY KEY,
    account_id CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    -- The account's local date the run belongs to, so "this morning's briefing"
    -- is a lookup and not a timezone calculation at read time.
    ran_on     DATE        NOT NULL,

    drafted        INTEGER NOT NULL DEFAULT 0,
    skipped_opted_out INTEGER NOT NULL DEFAULT 0,
    skipped_budget INTEGER NOT NULL DEFAULT 0,
    skipped_error  INTEGER NOT NULL DEFAULT 0,

    -- Whether the provider's batch endpoint was used. Recorded because the whole
    -- cost argument for running overnight rests on it, and an account whose
    -- provider has no batch endpoint gets the convenience without the discount —
    -- which the briefing must not imply otherwise.
    used_batch BOOLEAN NOT NULL DEFAULT FALSE,

    -- Cleared when somebody sees it, so the briefing appears once rather than
    -- every time the app is opened that day.
    seen_at    TIMESTAMPTZ,

    started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);

-- One run per account per local night.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_overnight_run_day
    ON overnight_draft_runs (account_id, ran_on);

-- The briefing read: this account's most recent unseen run.
CREATE INDEX IF NOT EXISTS idx_overnight_runs_unseen
    ON overnight_draft_runs (account_id, ran_on DESC)
    WHERE seen_at IS NULL;
