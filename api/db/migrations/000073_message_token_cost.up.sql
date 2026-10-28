-- US-030: per-conversation token cost attribution.
--
-- Token counts + estimated provider cost are stamped on every AI-generated
-- outbound message so we can bill overages and analyze cost-per-customer.
-- input_tokens / output_tokens come from the provider's usage report
-- (fallback len(text)/4 when the provider omits them). provider_cost_usd is
-- a coarse list-price estimate derived from the agent's provider+model.
--
-- DOUBLE PRECISION (not NUMERIC) on purpose: these are cost *estimates* for
-- attribution analytics, not authoritative ledger entries — float8 maps to a
-- plain float64 in sqlc/pgx and avoids pgtype.Numeric ceremony at every call
-- site. The conversation aggregate is incremented per AI turn.
ALTER TABLE messages ADD COLUMN IF NOT EXISTS input_tokens INTEGER;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS output_tokens INTEGER;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS provider_cost_usd DOUBLE PRECISION;

ALTER TABLE conversations ADD COLUMN IF NOT EXISTS total_cost_usd DOUBLE PRECISION NOT NULL DEFAULT 0;

-- Powers the "top 10 most-expensive conversations" dashboard endpoint.
CREATE INDEX IF NOT EXISTS idx_conversations_account_cost
    ON conversations (account_id, total_cost_usd DESC);
