-- Collapse the plan catalogue to a single contract tier: Enterprise.
--
-- The product is contact-sales only ("o preço nasce na conversa") — there are
-- no published price tiers. The SDR tiers Vendedor/Equipe/Escala carried made-up
-- fixed prices (R$797/1497/2997) that never reflected a real sale, and Partner
-- is dead v1 reseller config. All four have ZERO accounts / invites / Stripe
-- subscriptions (verified 2026-09-03), so no billing history is lost. Every
-- contract is now Enterprise, with per-account limits set via the owner panel's
-- override fields.
--
-- Unlike the self-serve attendance ladders (whose checkout e2e tests still
-- exercise their rows), nothing depends on these four rows existing beyond the
-- spend-caps ladder test, updated alongside this migration.

-- 1. Move any FK holder (accounts, account_invites) onto Enterprise first so the
--    DELETE can't orphan a row. A no-op in prod today (all four are empty).
UPDATE accounts SET plan_id = '01PLAN_ENTERPRISE_00000', updated_at = NOW()
WHERE plan_id IN (
  SELECT id FROM plans
  WHERE currency = 'brl' AND name IN ('Partner', 'Vendedor', 'Equipe', 'Escala')
);
UPDATE account_invites SET plan_id = '01PLAN_ENTERPRISE_00000'
WHERE plan_id IN (
  SELECT id FROM plans
  WHERE currency = 'brl' AND name IN ('Partner', 'Vendedor', 'Equipe', 'Escala')
);

-- 2. Make Enterprise the generous single tier so it never throttles an SDR
--    account: lift the SDR caps to the old top tier's numbers (Escala). The
--    columns left at 0 (leads/enrichments/mailboxes/funnels) already resolve to
--    UNLIMITED for a contract plan; drafts/docs/daily-tokens must be a real
--    non-zero number (the spend-caps guard forbids 0), so set them explicitly.
UPDATE plans SET
  max_ai_drafts_per_month = 3000,
  max_document_extractions_per_month = 300,
  max_daily_ai_tokens = 40000000,
  updated_at = NOW()
WHERE id = '01PLAN_ENTERPRISE_00000';

-- 3. Delete the retired tiers.
DELETE FROM plans
WHERE currency = 'brl' AND name IN ('Partner', 'Vendedor', 'Equipe', 'Escala');
