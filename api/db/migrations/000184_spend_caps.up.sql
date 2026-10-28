-- Per-plan ceilings on the AI work that is not a message (§6.14, K4).
--
-- Prospecting (max_leads_per_month), enrichment (max_enrichments_per_month) and
-- outbound sends (max_broadcast_messages_per_month) already have ceilings. Two
-- expensive things did not:
--
--   * AI-written approaches. `draft_approaches_batch` fills a queue in one call,
--     so a single request can spend a month's margin before anyone approves a
--     word of it.
--   * Document extraction. The scanned-contract path sends whole PDFs to a
--     multimodal model — the single most expensive operation in the product,
--     priced per page.
--
-- Both were bounded only by the daily token cap, which is an anti-runaway rail
-- and not a per-plan allowance: it lets one account burn the same daily budget
-- every day of the month regardless of what they pay.
--
-- 0 keeps its meaning from every other plan column: "not allowed" for a
-- self-serve plan, "use the account override" for a contract plan (resolveLimit
-- in domain/account.go).
ALTER TABLE plans
    ADD COLUMN IF NOT EXISTS max_ai_drafts_per_month INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_document_extractions_per_month INTEGER NOT NULL DEFAULT 0;

-- The SDR ladder.
--
-- Drafts are sized at 2x the lead allowance: a salesperson rewrites, and a cap
-- that equals the lead count would punish exactly the careful use of the
-- feature. Extractions are sized off the real workflow (a contract arrives,
-- gets read once, occasionally re-read after an amendment) with headroom —
-- 40/100/300 rather than a number derived from the lead count, because
-- contracts and prospects have nothing to do with each other.
UPDATE plans SET
    max_ai_drafts_per_month = 400,
    max_document_extractions_per_month = 40,
    updated_at = NOW()
WHERE id = '01PLAN_SDR_VENDEDOR_000';

UPDATE plans SET
    max_ai_drafts_per_month = 1000,
    max_document_extractions_per_month = 100,
    updated_at = NOW()
WHERE id = '01PLAN_SDR_EQUIPE_00000';

UPDATE plans SET
    max_ai_drafts_per_month = 3000,
    max_document_extractions_per_month = 300,
    updated_at = NOW()
WHERE id = '01PLAN_SDR_ESCALA_00000';

-- The retired attendance ladder and the trial keep a working allowance rather
-- than inheriting 0.
--
-- 0 would mean "not allowed", which would BREAK drafting for every account
-- still on one of those plans the moment this migration lands — a cap
-- introduced as a regression is a cap that gets reverted. Sized off each plan's
-- existing broadcast allowance, which is the closest thing it already had to a
-- statement about outbound volume.
UPDATE plans SET
    max_ai_drafts_per_month = GREATEST(max_broadcast_messages_per_month, 100),
    max_document_extractions_per_month = 40,
    updated_at = NOW()
WHERE max_ai_drafts_per_month = 0
  AND id NOT IN (
    '01PLAN_SDR_VENDEDOR_000',
    '01PLAN_SDR_EQUIPE_00000',
    '01PLAN_SDR_ESCALA_00000'
  );

-- The counting index. Both caps read the AI cost ledger rather than keeping
-- their own counters (one source of truth, per migration 000153's reasoning),
-- so the hot query is "this account's rows of these operations since a date".
CREATE INDEX IF NOT EXISTS idx_ai_costs_account_operation
    ON ai_costs (account_id, operation, occurred_at DESC);
