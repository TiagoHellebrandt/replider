-- US-005 (vendedores v3): real API token costs per provider/model so
-- monthly net profit (US-006) is computed on actual cost, not the
-- billing markup multipliers.
--
-- History is preserved: a price change is a NEW row with a later
-- effective_from — never an UPDATE of the old one. The cost in force
-- on a given date is the row with the greatest effective_from <= date
-- (see ProviderCostRepository.GetEffective). Costs are integer cents
-- of USD per 1M tokens (money always in cents).
CREATE TABLE IF NOT EXISTS provider_costs (
    id                            CHAR(26) PRIMARY KEY,
    provider                      TEXT NOT NULL,
    model                         TEXT NOT NULL,
    input_cost_per_1m_cents_usd   BIGINT NOT NULL CHECK (input_cost_per_1m_cents_usd >= 0),
    output_cost_per_1m_cents_usd  BIGINT NOT NULL CHECK (output_cost_per_1m_cents_usd >= 0),
    effective_from                DATE NOT NULL,
    created_at                    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, model, effective_from)
);

CREATE INDEX IF NOT EXISTS idx_provider_costs_lookup
    ON provider_costs (provider, model, effective_from DESC);

-- Key-value settings the owner edits at runtime (first consumer: the
-- manual USD->BRL rate used by the profit computation — deliberately
-- NOT a live FX API). Values are TEXT; each consumer parses its own.
CREATE TABLE IF NOT EXISTS platform_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_by TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed: July/2026 price list (cents USD per 1M tokens). Fixed ULIDs so
-- the migration is deterministic; ON CONFLICT keeps it idempotent.
INSERT INTO provider_costs (id, provider, model, input_cost_per_1m_cents_usd, output_cost_per_1m_cents_usd, effective_from) VALUES
    ('01K0000000000000000PC0ST01', 'google', 'gemini-3.1-flash-lite',         25, 150, '2026-07-01'),
    ('01K0000000000000000PC0ST02', 'google', 'gemini-2.5-flash',              30, 250, '2026-07-01'),
    ('01K0000000000000000PC0ST03', 'google', 'gemini-3.1-flash-live-preview', 75, 450, '2026-07-01'),
    ('01K0000000000000000PC0ST04', 'openai', 'gpt-5.4-mini',                  75, 450, '2026-07-01')
ON CONFLICT (provider, model, effective_from) DO NOTHING;

-- Default USD->BRL rate: 550 BRL cents per 1 USD (R$5.50). Owner edits
-- via PUT /owner/exchange-rate.
INSERT INTO platform_settings (key, value) VALUES ('usd_brl_rate_cents', '550')
ON CONFLICT (key) DO NOTHING;
