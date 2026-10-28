-- Vendedores PRD v3 (US-001): the Partner plan is dead — the sales
-- pivot replaces white-label partners with commissioned sellers on the
-- Replider platform account. The row stays (existing FK references,
-- historical invoices) but must never surface in the public catalogue.
--
-- is_listed gates GET /v1/plans: FALSE hides the plan from the public
-- listing while GetPlanByID keeps working for accounts already on it.
-- Both statements are idempotent — the UPDATE is a no-op when the
-- Partner row doesn't exist (fresh installs seed without it).
ALTER TABLE plans ADD COLUMN IF NOT EXISTS is_listed BOOLEAN NOT NULL DEFAULT TRUE;

UPDATE plans SET
  is_listed  = FALSE,
  updated_at = NOW()
WHERE id = '01PLAN_PARTNER_00000000'
  AND is_listed IS DISTINCT FROM FALSE;
