-- A/B testing for system prompts.
--
-- An experiment binds two prompt bodies (variant A + variant B) to a
-- single agent and splits new conversations between them according to
-- `split_pct_b` (1..99 — never 0 or 100 because that would just be the
-- normal agent.system_prompt path).
--
-- `experiment_assignments` is sticky: a conversation is bucketed once
-- (PK ensures uniqueness on conflict) and stays on that variant for
-- its entire lifetime, even if the experiment is later stopped — so
-- the metrics tab can attribute closed conversations correctly.
--
-- A partial unique index enforces "at most one running experiment per
-- agent" at the DB layer so two start_experiment calls can't race past
-- the usecase's check.
CREATE TABLE prompt_experiments (
    id              CHAR(26) PRIMARY KEY,                                        -- ULID
    account_id      CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    agent_id        CHAR(26) NOT NULL REFERENCES agents(id)   ON DELETE CASCADE,
    name            VARCHAR(120) NOT NULL,
    status          VARCHAR(20)  NOT NULL DEFAULT 'draft',                       -- draft | running | stopped | completed
    variant_a_body  TEXT NOT NULL,
    variant_b_body  TEXT NOT NULL,
    variant_a_label VARCHAR(40) NOT NULL DEFAULT 'A',
    variant_b_label VARCHAR(40) NOT NULL DEFAULT 'B',
    split_pct_b     SMALLINT NOT NULL DEFAULT 50 CHECK (split_pct_b BETWEEN 1 AND 99),
    winner          CHAR(1),                                                     -- 'A' | 'B' | NULL
    started_at      TIMESTAMPTZ,
    stopped_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_prompt_experiments_agent ON prompt_experiments(agent_id, created_at DESC);
CREATE INDEX idx_prompt_experiments_account ON prompt_experiments(account_id, created_at DESC);

-- Hard guarantee: at most one running experiment per agent. Two
-- start_experiment calls landing concurrently will see one succeed and
-- the other 23505 — the usecase maps that to ErrExperimentAlreadyRunning.
CREATE UNIQUE INDEX uniq_prompt_experiments_one_running_per_agent
    ON prompt_experiments(agent_id) WHERE status = 'running';

CREATE TABLE experiment_assignments (
    experiment_id   CHAR(26) NOT NULL REFERENCES prompt_experiments(id) ON DELETE CASCADE,
    conversation_id CHAR(26) NOT NULL REFERENCES conversations(id)      ON DELETE CASCADE,
    variant         CHAR(1)  NOT NULL CHECK (variant IN ('A','B')),
    assigned_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (experiment_id, conversation_id)
);

-- Reverse-lookup hot path: given a conversation, which experiment is
-- it on? Used by the runner before each AI call.
CREATE INDEX idx_experiment_assignments_conv ON experiment_assignments(conversation_id);
