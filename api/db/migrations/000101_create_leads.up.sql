-- Outbound Sales + CRM (US-003): Leads. A lead ties a contact to a sales
-- funnel and carries the metadata the AI invisibly collected during the
-- conversation, optional enrichment data (decision-maker, company info),
-- a drafted outbound approach, and a status tracking it through the pipeline.
--
-- UNIQUE(funnel_id, contact_id) makes the pair idempotent: re-processing the
-- same contact on the same funnel upserts the existing lead instead of
-- creating duplicates.
--
-- funnel_id / contact_id cascade-delete with their parents. account_id is
-- denormalised from the funnel so leads can be listed/scoped without a join,
-- mirroring every other account-scoped table.
--
-- `collected_metadata` is a JSONB object keyed by the funnel's metadata field
-- keys; `enrichment` is a free-form JSONB blob populated by prospecting /
-- Gemini Grounding (US-021..US-024).
CREATE TABLE IF NOT EXISTS leads (
    id                 TEXT PRIMARY KEY,
    account_id         TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    funnel_id          TEXT NOT NULL REFERENCES sales_funnels(id) ON DELETE CASCADE,
    contact_id         TEXT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    status             TEXT NOT NULL DEFAULT 'new',
    source             TEXT NOT NULL DEFAULT '',
    score              INTEGER NOT NULL DEFAULT 0,
    collected_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    enrichment         JSONB NOT NULL DEFAULT '{}'::jsonb,
    approach_draft     TEXT NOT NULL DEFAULT '',
    buy_intent_at      TIMESTAMPTZ,
    qualified_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (funnel_id, contact_id)
);

CREATE INDEX IF NOT EXISTS idx_leads_account ON leads(account_id);
CREATE INDEX IF NOT EXISTS idx_leads_funnel_status ON leads(funnel_id, status);
