-- US-M008: Kanban columns.
--
-- Leads already have a canonical `status` (new → prospected → … → won/lost)
-- that the whole pipeline reasons about: the AI advances it, the approval
-- queue gates on it, analytics group by it. A Kanban board, though, is the
-- CUSTOMER's mental model of their own sales process — "Diagnóstico",
-- "Proposta enviada", "Aguardando comodato" — and forcing them onto our
-- enum makes the board useless for them.
--
-- So the two live side by side: a stage is a presentation-layer column that
-- MAPS ONTO a canonical status. Moving a card sets both — the stage for the
-- board, the derived status for everything else — which keeps the pipeline
-- working while the customer names their own columns.
CREATE TABLE IF NOT EXISTS funnel_stages (
    id         CHAR(26) PRIMARY KEY,
    account_id CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    funnel_id  CHAR(26) NOT NULL REFERENCES sales_funnels(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    -- Board order. Gaps are allowed and expected: reordering rewrites
    -- positions, and leaving room avoids renumbering every row on an insert.
    position   INT NOT NULL,
    -- The canonical status this column means. Several stages may map to the
    -- same status (a customer with three "negotiating" columns is normal);
    -- the reverse is what would break the pipeline.
    maps_to_status TEXT NOT NULL,
    -- Terminal columns (won/lost) are rendered differently and excluded from
    -- "stuck lead" alerts. Derived from maps_to_status, stored for query
    -- convenience.
    is_terminal BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_funnel_stages_funnel
    ON funnel_stages(funnel_id, position);
-- Two columns with the same name in one funnel is a UI mistake, not a
-- feature: the board becomes ambiguous to drag onto.
CREATE UNIQUE INDEX IF NOT EXISTS idx_funnel_stages_funnel_name
    ON funnel_stages(funnel_id, lower(name));

-- The lead's board position. NULL means "not placed yet" — every lead
-- created before this migration, and every lead created by a path that does
-- not know about stages (inbound, prospecting). The list query resolves
-- those onto the stage matching their canonical status, so a board is never
-- missing cards.
ALTER TABLE leads ADD COLUMN IF NOT EXISTS stage_id CHAR(26)
    REFERENCES funnel_stages(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_leads_stage ON leads(stage_id) WHERE stage_id IS NOT NULL;
