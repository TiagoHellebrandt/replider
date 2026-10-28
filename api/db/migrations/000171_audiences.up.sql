-- Target audiences (§2, D3).
--
-- A saved answer to "who are we selling to". Prospecting without one is a
-- keyword typed from memory every morning, which is how a supplier works the
-- same three cities for a year without noticing the fourth.
CREATE TABLE IF NOT EXISTS audiences (
    id             CHAR(26)    PRIMARY KEY,
    account_id     CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name           TEXT        NOT NULL,
    -- The kind of business in the customer's own words ("clínicas de
    -- diagnóstico por imagem"), not a CNAE code.
    segment        TEXT        NOT NULL DEFAULT '',
    -- Free text: every market measures size differently and a dropdown would
    -- be wrong for most of them.
    size           TEXT        NOT NULL DEFAULT '',
    -- The ROLE to reach, which decides how the approach is written far more
    -- than the segment does.
    decision_maker TEXT        NOT NULL DEFAULT '',
    regions        TEXT[]      NOT NULL DEFAULT '{}',
    keywords       TEXT[]      NOT NULL DEFAULT '{}',
    -- Kept visible: a model's suggestion presented as the customer's own
    -- decision is a small lie that erodes trust in every other suggestion.
    suggested      BOOLEAN     NOT NULL DEFAULT FALSE,
    -- The "você talvez não viu" slot: adjacent rather than obvious. Exactly
    -- one per suggestion round.
    unexplored     BOOLEAN     NOT NULL DEFAULT FALSE,
    is_active      BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audiences_account
    ON audiences (account_id) WHERE is_active;
