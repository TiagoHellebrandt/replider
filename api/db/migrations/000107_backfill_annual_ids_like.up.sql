-- Self-healing backfill of the annual Stripe price ids. Migration 000106
-- used `id = '<unpadded>'`; this re-runs the same intent with
-- rtrim(id) = '<unpadded>' so it's immune to any char(26) padding
-- surprise. Guarded (writes only when the column is still empty) so it's
-- a no-op if 000106 already populated it. Ids are live Stripe prices.
UPDATE plans SET stripe_price_id_annual = 'price_REDACTED01', updated_at = NOW()
  WHERE rtrim(id) = '01PLAN_SOLO_00000000000' AND COALESCE(stripe_price_id_annual,'') = '';
UPDATE plans SET stripe_price_id_annual = 'price_REDACTED02', updated_at = NOW()
  WHERE rtrim(id) = '01PLAN_BRONZE_000000000' AND COALESCE(stripe_price_id_annual,'') = '';
UPDATE plans SET stripe_price_id_annual = 'price_REDACTED03', updated_at = NOW()
  WHERE rtrim(id) = '01PLAN_SILVER_000000000' AND COALESCE(stripe_price_id_annual,'') = '';
UPDATE plans SET stripe_price_id_annual = 'price_REDACTED04', updated_at = NOW()
  WHERE rtrim(id) = '01PLAN_GOLD_0000000000' AND COALESCE(stripe_price_id_annual,'') = '';
UPDATE plans SET stripe_price_id_annual = 'price_REDACTED05', updated_at = NOW()
  WHERE rtrim(id) = '01PLAN_DIAMOND_00000000' AND COALESCE(stripe_price_id_annual,'') = '';
