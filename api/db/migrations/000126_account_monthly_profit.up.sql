-- US-006 (vendedores v3): per-account monthly net profit — the honest
-- base the commission ledger (US-007) multiplies its rates against.
--
-- One row per (account, cycle). A recompute OVERWRITES the row (upsert
-- on the UNIQUE), never duplicates — money math must be idempotent
-- under reprocessing. Negative net is allowed and expected (e.g. trial
-- accounts burning tokens with zero revenue); the owner panel derives
-- is_negative from net_profit_cents < 0.
--
-- cycle is the calendar month 'YYYY-MM' (UTC boundaries). All values
-- are integer BRL cents:
--   revenue    = Stripe invoices PAID inside the cycle (amount_paid)
--   stripe_fee = the REAL fee from each charge's balance_transaction
--   token_cost = ai tokens used in the cycle x provider_costs in force
--                x manual usd_brl_rate_cents (platform_settings)
--   tax        = profit_tax_bps (platform_settings) applied on revenue
--   infra      = flat per-account monthly constant (infra_cost_cents)
--   net        = revenue - stripe_fee - token_cost - tax - infra
CREATE TABLE IF NOT EXISTS account_monthly_profit (
    id                CHAR(26) PRIMARY KEY,
    account_id        CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    cycle             CHAR(7) NOT NULL CHECK (cycle ~ '^[0-9]{4}-[0-9]{2}$'),
    revenue_cents     BIGINT NOT NULL DEFAULT 0,
    stripe_fee_cents  BIGINT NOT NULL DEFAULT 0,
    token_cost_cents  BIGINT NOT NULL DEFAULT 0,
    tax_cents         BIGINT NOT NULL DEFAULT 0,
    infra_cents       BIGINT NOT NULL DEFAULT 0,
    net_profit_cents  BIGINT NOT NULL DEFAULT 0,
    computed_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, cycle)
);

CREATE INDEX IF NOT EXISTS idx_account_monthly_profit_cycle
    ON account_monthly_profit (cycle);

-- Profit-computation settings (owner-editable via PUT
-- /owner/profit-settings). Basis points for tax so fractional percents
-- stay integer; infra default R$5.00/account/month per the PRD.
INSERT INTO platform_settings (key, value) VALUES
    ('profit_tax_bps', '0'),
    ('infra_cost_cents', '500')
ON CONFLICT (key) DO NOTHING;
