-- ALLOW-DESTRUCTIVE: removes the tiers and columns this migration added.
-- The old ladder comes back to the catalogue exactly as it was.
UPDATE plans
SET is_listed = TRUE, updated_at = NOW()
WHERE id IN (
  '01PLAN_SOLO_00000000000',
  '01PLAN_BRONZE_000000000',
  '01PLAN_SILVER_000000000',
  '01PLAN_GOLD_0000000000',
  '01PLAN_DIAMOND_00000000'
);

DELETE FROM plans WHERE id IN (
  '01PLAN_SDR_VENDEDOR_000',
  '01PLAN_SDR_EQUIPE_00000',
  '01PLAN_SDR_ESCALA_00000'
);

ALTER TABLE plans
    DROP COLUMN IF EXISTS max_leads_per_month,
    DROP COLUMN IF EXISTS max_enrichments_per_month,
    DROP COLUMN IF EXISTS max_email_mailboxes,
    DROP COLUMN IF EXISTS max_funnels;
