-- Client-facing commission rules (§6.6, G1).
--
-- NOT commission_ledger. That one pays Replider's own account managers a
-- share of Replider's profit under a fixed internal policy. This is the
-- CUSTOMER's rule for the CUSTOMER's salespeople, applied to the customer's
-- own sales — today it lives in a spreadsheet, recalculated by hand every
-- month, and argued about every time somebody disagrees with a number.
CREATE TABLE IF NOT EXISTS commission_rules (
    id         CHAR(26)    PRIMARY KEY,
    account_id CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name       TEXT        NOT NULL,

    -- Which sales this governs. scope_id points at a user, a team or a
    -- product depending on scope; NULL for the global fallback. No FK: a
    -- rule outlives the salesperson who left, and losing the rule when a
    -- product is deleted would silently repay every future sale at a
    -- different rate.
    scope      TEXT        NOT NULL DEFAULT 'global'
               CHECK (scope IN ('global','seller','team','product')),
    scope_id   TEXT,

    -- What the percentage applies to, and what comes off gross first.
    base       TEXT        NOT NULL DEFAULT 'margin'
               CHECK (base IN ('gross','net','margin')),
    deduct_tax  BOOLEAN    NOT NULL DEFAULT FALSE,
    deduct_cost BOOLEAN    NOT NULL DEFAULT FALSE,

    -- The flat rate. Bounded here as well as in the domain: a 5000% typo
    -- reaching the table would repay every sale in the period.
    rate_percent NUMERIC(6,3) NOT NULL DEFAULT 0
                 CHECK (rate_percent >= 0 AND rate_percent <= 100),

    -- NULL means "same as a new sale", which is deliberately different from
    -- 0 ("we pay nothing on repurchase").
    repurchase_rate_percent NUMERIC(6,3)
                 CHECK (repurchase_rate_percent IS NULL
                        OR (repurchase_rate_percent >= 0 AND repurchase_rate_percent <= 100)),

    -- Tiers, milestones and splits are lists whose shape is validated in the
    -- domain before the write. JSONB rather than three child tables: they are
    -- always read and written as one rule, never queried across rules, and
    -- three joins to render one form is a lot of machinery for a list of
    -- three numbers.
    tiers      JSONB       NOT NULL DEFAULT '[]',
    milestones JSONB       NOT NULL DEFAULT '[]',
    splits     JSONB       NOT NULL DEFAULT '[]',

    active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- A scoped rule must say what it is scoped to, and a global rule must
    -- not: a global rule with a target applies to nobody, which is the
    -- silent kind of broken.
    CONSTRAINT commission_rule_scope_target
      CHECK ((scope = 'global' AND scope_id IS NULL)
             OR (scope <> 'global' AND scope_id IS NOT NULL AND scope_id <> ''))
);

CREATE INDEX IF NOT EXISTS idx_commission_rules_account
    ON commission_rules (account_id, active);

-- Only ONE active global rule per account. Two would make the fallback
-- ambiguous, and the tie-break (newest wins) would silently change the payout
-- the day somebody adds a second one "just to test".
CREATE UNIQUE INDEX IF NOT EXISTS uniq_commission_rule_global
    ON commission_rules (account_id)
    WHERE scope = 'global' AND active;
