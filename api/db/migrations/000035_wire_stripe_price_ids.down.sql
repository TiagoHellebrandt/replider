-- Revert: clear stripe_price_id on the five public tiers. The Stripe
-- Product + Price rows themselves are not deleted — they can be
-- archived in the dashboard if the rollback is permanent.

UPDATE plans SET stripe_price_id = NULL, updated_at = NOW()
  WHERE id IN (
    '01PLAN_SOLO_00000000000',
    '01PLAN_BRONZE_000000000',
    '01PLAN_SILVER_000000000',
    '01PLAN_GOLD_0000000000',
    '01PLAN_DIAMOND_00000000'
  );
