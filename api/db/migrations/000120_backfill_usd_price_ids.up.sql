-- Backfill Stripe Price ids for the USD plans (created 2026-07-08 in the
-- live Stripe account via MCP; products prod_REDACTED01 / prod_REDACTED02 /
-- prod_REDACTED03, one monthly recurring USD price each).
-- Same pattern as 000106 (annual price backfill). Numbered with a gap
-- (last on main was 000109) to avoid colliding with the vendedores-v3
-- branch, which allocates 000110+.

UPDATE plans SET stripe_price_id = 'price_REDACTED04', updated_at = NOW()
WHERE id = '01PLAN_SOLO_USD_0000000' AND currency = 'usd';

UPDATE plans SET stripe_price_id = 'price_REDACTED05', updated_at = NOW()
WHERE id = '01PLAN_CORE_USD_0000000' AND currency = 'usd';

UPDATE plans SET stripe_price_id = 'price_REDACTED06', updated_at = NOW()
WHERE id = '01PLAN_PRO_USD_00000000' AND currency = 'usd';
