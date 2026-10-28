-- Sales (§5, F1).
--
-- The one record three features read: the portfolio (what this customer buys
-- and when they need it again), commissions (what the salesperson earned) and
-- contracts (what was agreed). Getting the fields wrong here costs all three.
CREATE TABLE IF NOT EXISTS sales (
    id             CHAR(26)    PRIMARY KEY,
    account_id     CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    -- Who bought. A sale with no customer cannot feed a portfolio, which is
    -- half the reason this table exists.
    contact_id     TEXT        NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    -- The deal it came from, when there was one. Plenty of sales come from a
    -- phone call nobody logged.
    lead_id        CHAR(26),
    -- Who gets the commission. NULL is a house-account sale; inventing a
    -- salesperson would pay somebody for work they did not do. No FK, for the
    -- same reason commission_ledger has none: the record outlives the person.
    seller_user_id CHAR(26),

    items          JSONB       NOT NULL DEFAULT '[]',

    -- The three amounts kept SEPARATE, because a commission paid on gross is
    -- a commission paid on money the company never had: in distribution the
    -- cost of goods is most of the invoice, and 5% of gross can exceed 100%
    -- of margin.
    --
    -- BIGINT cents, never NUMERIC or float: this is authoritative money that
    -- feeds a payout, and a cent lost per sale is a monthly argument with a
    -- salesperson.
    gross_cents    BIGINT      NOT NULL DEFAULT 0 CHECK (gross_cents >= 0),
    cost_cents     BIGINT      NOT NULL DEFAULT 0 CHECK (cost_cents >= 0),
    tax_cents      BIGINT      NOT NULL DEFAULT 0 CHECK (tax_cents >= 0),
    currency       TEXT        NOT NULL DEFAULT 'brl',

    sold_at        TIMESTAMPTZ NOT NULL,

    -- How long until this customer needs it again. The single field the
    -- repurchase engine runs on; a sale without it is one it never resurfaces.
    cycle_days     INTEGER     NOT NULL DEFAULT 0,

    source         TEXT        NOT NULL DEFAULT 'manual'
                   CHECK (source IN ('manual','ai_extracted','imported')),
    -- Who approved an AI-extracted sale. Empty on a manual one, where the
    -- person entering it IS the confirmation.
    confirmed_by_user_id CHAR(26),

    notes          TEXT        NOT NULL DEFAULT '',

    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The period report, newest first.
CREATE INDEX IF NOT EXISTS idx_sales_account_date
    ON sales (account_id, sold_at DESC);

-- The portfolio view, and the inbox-triage "has this contact ever bought?".
CREATE INDEX IF NOT EXISTS idx_sales_contact
    ON sales (account_id, contact_id, sold_at DESC);

-- The commission statement.
CREATE INDEX IF NOT EXISTS idx_sales_seller
    ON sales (account_id, seller_user_id, sold_at DESC)
    WHERE seller_user_id IS NOT NULL;
