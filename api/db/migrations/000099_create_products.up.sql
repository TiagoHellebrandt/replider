-- Outbound Sales + CRM (US-001): Products catalog. A product belongs to an
-- account and can be referenced by a sales funnel. `attributes` is a free-form
-- JSONB bag so different verticals can describe SKUs without schema churn
-- (e.g. {"sku":"A1","tags":["promo"],"warranty_months":12}).
CREATE TABLE IF NOT EXISTS products (
    id          TEXT PRIMARY KEY,
    account_id  TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price_cents BIGINT NOT NULL DEFAULT 0,           -- minor units; 0 = unpriced
    currency    TEXT NOT NULL DEFAULT 'BRL',         -- ISO 4217
    attributes  JSONB NOT NULL DEFAULT '{}'::jsonb,  -- vertical-specific bag
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_products_account ON products(account_id);
