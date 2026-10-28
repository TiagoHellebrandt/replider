-- Backfill the annual Stripe price ids (US-169 / US-145). The five
-- self-serve annual prices (20% off 12× the monthly rate, recurring
-- year, BRL, livemode) were created directly in the live Stripe account
-- (acct_<redacted>) — this persists their ids so checkout's
-- plan.ResolveStripePriceID("annual") finds them. Partner/Enterprise are
-- contract tiers and intentionally keep stripe_price_id_annual NULL.
--
-- Idempotent: only sets the column when it's still empty, so it never
-- clobbers a value the seeder (cmd/seed_stripe_prices) may have written.
UPDATE plans SET stripe_price_id_annual = 'price_REDACTED01', updated_at = NOW()
  WHERE id = '01PLAN_SOLO_00000000000'   AND (stripe_price_id_annual IS NULL OR stripe_price_id_annual = '');
UPDATE plans SET stripe_price_id_annual = 'price_REDACTED02', updated_at = NOW()
  WHERE id = '01PLAN_BRONZE_000000000'   AND (stripe_price_id_annual IS NULL OR stripe_price_id_annual = '');
UPDATE plans SET stripe_price_id_annual = 'price_REDACTED03', updated_at = NOW()
  WHERE id = '01PLAN_SILVER_000000000'   AND (stripe_price_id_annual IS NULL OR stripe_price_id_annual = '');
UPDATE plans SET stripe_price_id_annual = 'price_REDACTED04', updated_at = NOW()
  WHERE id = '01PLAN_GOLD_0000000000'    AND (stripe_price_id_annual IS NULL OR stripe_price_id_annual = '');
UPDATE plans SET stripe_price_id_annual = 'price_REDACTED05', updated_at = NOW()
  WHERE id = '01PLAN_DIAMOND_00000000'   AND (stripe_price_id_annual IS NULL OR stripe_price_id_annual = '');
