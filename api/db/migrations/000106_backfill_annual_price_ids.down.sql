-- Reverts the annual price-id backfill. Does NOT delete the Stripe price
-- objects (they're external + may have live subscriptions); only clears
-- the column so the app falls back to monthly.
UPDATE plans SET stripe_price_id_annual = NULL, updated_at = NOW()
  WHERE id IN (
    '01PLAN_SOLO_00000000000',
    '01PLAN_BRONZE_000000000',
    '01PLAN_SILVER_000000000',
    '01PLAN_GOLD_0000000000',
    '01PLAN_DIAMOND_00000000'
  );
