-- Transparent token metering (US-M015).
--
-- The per-message stamp (messages.provider_cost_usd) only covers AI that
-- produced a message. Drafts, enrichment and prompt tooling spend the same
-- provider budget and were invisible — so an invoice line that says "IA
-- deste mês: R$X" built from messages alone would UNDER-charge, which is
-- the wrong kind of wrong when the whole promise is "cost passed through at
-- provider price, no markup".
--
-- Append-only, one row per AI call. cost_usd is stamped at call time and
-- never recomputed: the owner-editable price table can change, and a past
-- month's invoice must not move because of it.
CREATE TABLE IF NOT EXISTS ai_costs (
    id         CHAR(26) PRIMARY KEY,
    account_id CHAR(26) NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    -- What was being done ('approach_draft', 'enrichment', …). Free-form:
    -- a new AI-spending feature must not need a migration to be billed.
    operation  VARCHAR(48) NOT NULL,
    provider   VARCHAR(32) NOT NULL DEFAULT '',
    model      VARCHAR(64) NOT NULL DEFAULT '',
    input_tokens  BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    -- DOUBLE PRECISION, not NUMERIC: this is an estimate summed for a
    -- display line, and sqlc maps NUMERIC to pgtype.Numeric (see
    -- api/CLAUDE.md on money columns).
    cost_usd   DOUBLE PRECISION NOT NULL DEFAULT 0,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The only access pattern: one account, one billing period.
CREATE INDEX IF NOT EXISTS idx_ai_costs_account_time
    ON ai_costs (account_id, occurred_at DESC);
