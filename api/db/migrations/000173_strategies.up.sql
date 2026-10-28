-- Sales strategies (§4, E2/E3).
--
-- The written-down version of "como a gente vende". An ENTITY rather than
-- prose in a document for one reason: attribution. Without a strategy id on
-- every approach and every lead, nothing downstream measures anything.
CREATE TABLE IF NOT EXISTS strategies (
    id             CHAR(26)    PRIMARY KEY,
    account_id     CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name           TEXT        NOT NULL,
    -- Bumped when the CONTENT changes. "Converte 14%" is meaningless if the
    -- arguments were rewritten twice since the number was earned.
    version        INTEGER     NOT NULL DEFAULT 1,
    status         TEXT        NOT NULL DEFAULT 'draft'
                   CHECK (status IN ('draft','testing','promoted','retired')),

    icp            TEXT        NOT NULL DEFAULT '',
    channel_mix    TEXT        NOT NULL DEFAULT '',
    sequence_id    CHAR(26),
    tone           TEXT        NOT NULL DEFAULT '',
    arguments      TEXT[]      NOT NULL DEFAULT '{}',
    -- Objection → response pairs. JSONB because they are always read and
    -- written together and never queried across strategies.
    objections     JSONB       NOT NULL DEFAULT '[]',
    triggers       TEXT[]      NOT NULL DEFAULT '{}',

    -- Recognition, and recognition is what makes a salesperson contribute
    -- one. No FK: a strategy outlives the person who wrote it, and a GDPR
    -- deletion must not erase the company's method.
    author_user_id CHAR(26),
    author_name    TEXT        NOT NULL DEFAULT '',

    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_strategies_account ON strategies (account_id, status);

-- E3: attribution. Every lead carries which strategy version reached it.
--
-- Nullable, and stays nullable: leads created before strategies existed, and
-- leads worked without one, are honest states — a default pointing at some
-- arbitrary strategy would make the first measurement a lie.
ALTER TABLE leads
    ADD COLUMN IF NOT EXISTS strategy_id      CHAR(26),
    ADD COLUMN IF NOT EXISTS strategy_version INTEGER;

-- The measurement query: results per strategy VERSION.
CREATE INDEX IF NOT EXISTS idx_leads_strategy
    ON leads (account_id, strategy_id, strategy_version)
    WHERE strategy_id IS NOT NULL;
