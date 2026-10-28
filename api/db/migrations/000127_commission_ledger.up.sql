-- US-007 (vendedores v3): commission ledger — tracks 50/20 + activity
-- clause. Applies rates to account_monthly_profit (US-006).
--
-- seller_settings: one row per platform-account user acting as seller.
-- Absent row = defaults (own 50%, inbound 20%, active). Rates are
-- percents (NUMERIC so fractional percents work); the activity clause
-- (PRD: inactive AM keeps 15% residual for 12 months, then nothing) is
-- driven by is_active_am + residual_until.
CREATE TABLE IF NOT EXISTS seller_settings (
    user_id                 CHAR(26) PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    commission_rate_own     NUMERIC(5,2) NOT NULL DEFAULT 50,
    commission_rate_inbound NUMERIC(5,2) NOT NULL DEFAULT 20,
    is_active_am            BOOLEAN NOT NULL DEFAULT TRUE,
    deactivated_at          TIMESTAMPTZ,
    residual_until          DATE,
    pix_key                 TEXT,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- commission_ledger: one entry per (seller, account, cycle, reason).
-- Recomputes upsert on the UNIQUE — idempotent, never duplicated — and
-- NEVER touch rows already paid. amount_cents is integer BRL cents;
-- base_net_profit_cents records the profit the rate was applied to
-- (kept even when negative — the entry then pays 0, never negative).
-- 'standard'/'residual'/'sla_reduced' are the mutually-exclusive
-- profit-share reasons managed by the cycle recompute; 'bonus_*' rows
-- (US-008) are additive and live alongside.
CREATE TABLE IF NOT EXISTS commission_ledger (
    id                     CHAR(26) PRIMARY KEY,
    seller_user_id         CHAR(26) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id             CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    cycle                  CHAR(7) NOT NULL CHECK (cycle ~ '^[0-9]{4}-[0-9]{2}$'),
    base_net_profit_cents  BIGINT NOT NULL DEFAULT 0,
    rate_applied           NUMERIC(5,2) NOT NULL,
    amount_cents           BIGINT NOT NULL DEFAULT 0 CHECK (amount_cents >= 0),
    reason                 TEXT NOT NULL CHECK (reason IN ('standard','residual','sla_reduced','bonus_upgrade','bonus_nps','bonus_retention')),
    status                 TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','paid')),
    paid_at                TIMESTAMPTZ,
    note                   TEXT,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (seller_user_id, account_id, cycle, reason)
);

CREATE INDEX IF NOT EXISTS idx_commission_ledger_cycle
    ON commission_ledger (cycle);
CREATE INDEX IF NOT EXISTS idx_commission_ledger_seller
    ON commission_ledger (seller_user_id, cycle);
