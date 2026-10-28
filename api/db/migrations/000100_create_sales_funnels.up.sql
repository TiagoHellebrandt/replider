-- Outbound Sales + CRM (US-002): Sales funnels. A funnel ties together a
-- product, an AI agent and a messaging channel, plus a dynamic metadata
-- schema describing the fields the AI should invisibly collect during the
-- conversation, and the webhook configuration fired when a lead qualifies or
-- shows buy intent.
--
-- product_id / agent_id / channel_id are nullable with ON DELETE SET NULL so
-- deleting any referenced resource detaches it from the funnel instead of
-- cascading the funnel away.
--
-- `metadata_schema` is a JSONB array of field definitions
-- ([{key,label,type,required,ai_hint,enum_options}]); validated in the domain
-- layer (snake_case unique keys, allowed types).
CREATE TABLE IF NOT EXISTS sales_funnels (
    id                           TEXT PRIMARY KEY,
    account_id                   TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name                         TEXT NOT NULL,
    product_id                   TEXT REFERENCES products(id) ON DELETE SET NULL,
    agent_id                     TEXT REFERENCES agents(id) ON DELETE SET NULL,
    channel_id                   TEXT REFERENCES channels(id) ON DELETE SET NULL,
    persona_prompt               TEXT NOT NULL DEFAULT '',
    metadata_schema              JSONB NOT NULL DEFAULT '[]'::jsonb,
    webhook_url                  TEXT NOT NULL DEFAULT '',
    webhook_secret               TEXT NOT NULL DEFAULT '',
    trigger_on_required_complete BOOLEAN NOT NULL DEFAULT FALSE,
    trigger_on_buy_intent        BOOLEAN NOT NULL DEFAULT FALSE,
    active                       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at                   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sales_funnels_account ON sales_funnels(account_id);
