-- Commission entries: the customer's own monthly statement (§6.6, G2).
--
-- One row is one line somebody can point at — this sale, this rule, this base,
-- this rate, this amount. The statement is those lines added up for one month
-- and one salesperson.
--
-- NOT commission_ledger, which pays Replider's own account managers under a
-- fixed internal policy.
CREATE TABLE IF NOT EXISTS commission_lines (
    id         CHAR(26)    PRIMARY KEY,
    account_id CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    -- "YYYY-MM", resolved in the ACCOUNT's timezone: a deal closed at 21:00
    -- on the 31st has to land in the month the salesperson thinks it did.
    cycle      CHAR(7)     NOT NULL,

    -- Who gets paid. No FK to users: the line is a financial snapshot that has
    -- to survive the salesperson leaving (the platform's own ledger learned
    -- this the hard way in migration 000136 — a CASCADE erased paid history).
    seller_user_id CHAR(26) NOT NULL,

    kind    TEXT NOT NULL
            CHECK (kind IN ('commission','milestone','adjustment')),

    -- No FK on either: a rule may be deleted and a sale may be corrected, but
    -- "why was I paid this?" outlives both.
    sale_id CHAR(26),
    rule_id CHAR(26),
    -- The split role this line pays; empty when the whole commission goes to
    -- the sale's own salesperson.
    role    TEXT NOT NULL DEFAULT '',

    base_cents   BIGINT       NOT NULL DEFAULT 0,
    rate_percent NUMERIC(6,3) NOT NULL DEFAULT 0,
    -- Negative allowed ONLY on an adjustment: a clawback is a real event, and
    -- a commission line that pays a negative amount is a bug.
    amount_cents BIGINT       NOT NULL
                 CHECK (kind = 'adjustment' OR amount_cents >= 0),

    -- The sentence the engine wrote when it calculated this. Stored, not
    -- recomputed: the rule may have changed since, and the answer to "why this
    -- number" must be the reason it was ACTUALLY paid.
    explanation TEXT NOT NULL DEFAULT '',
    note        TEXT NOT NULL DEFAULT '',

    status  TEXT NOT NULL DEFAULT 'pending'
            CHECK (status IN ('pending','paid')),
    paid_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- A paid line must say when. A pending one must not.
    CONSTRAINT commission_line_paid_stamp
      CHECK ((status = 'paid' AND paid_at IS NOT NULL)
             OR (status = 'pending' AND paid_at IS NULL))
);

-- The statement read: one cycle, one account, optionally one seller.
CREATE INDEX IF NOT EXISTS idx_commission_lines_cycle
    ON commission_lines (account_id, cycle, seller_user_id);

-- The recomputation deletes exactly this set, so it gets its own index.
CREATE INDEX IF NOT EXISTS idx_commission_lines_pending
    ON commission_lines (account_id, cycle)
    WHERE status = 'pending';

-- One PENDING commission line per (sale, seller, role). Partial, like the
-- platform ledger's index (000135): paid rows must not arbitrate, or the
-- first settled adjustment would silently block every later correction.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_commission_line_pending_sale
    ON commission_lines (account_id, cycle, sale_id, seller_user_id, role)
    WHERE status = 'pending' AND kind = 'commission' AND sale_id IS NOT NULL;
