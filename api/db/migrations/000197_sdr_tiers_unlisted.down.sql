-- Restores what 000154 left: the three tiers back in the public catalogue.
--
-- Reverting this re-creates the defect on purpose — the tiers still have no
-- stripe_price_id, so the catalogue would again advertise a price every checkout
-- path refuses. That is what "undo the previous migration" means here; the fix
-- is to configure the Stripe Prices, not to roll this back.
UPDATE plans
SET is_listed = TRUE, updated_at = NOW()
WHERE id IN (
  '01PLAN_SDR_VENDEDOR_000',
  '01PLAN_SDR_EQUIPE_00000',
  '01PLAN_SDR_ESCALA_00000'
);
