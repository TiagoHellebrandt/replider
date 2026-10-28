-- Wire Stripe Price IDs into the plans table. Values came from the
-- MCP-driven create_product / create_price batch run on 2026-04-21
-- against the LIVE Stripe account acct_<redacted> (livemode=true).
--
-- Partner stays NULL on purpose — commercial handshake only; we don't
-- list it publicly and checkout flows for it go through a bespoke
-- invoice link, not this price id.
--
-- Never hardcode these ids in Go; the source of truth is this row.

UPDATE plans SET stripe_price_id = 'price_REDACTED01', updated_at = NOW()
  WHERE id = '01PLAN_SOLO_00000000000';
UPDATE plans SET stripe_price_id = 'price_REDACTED02', updated_at = NOW()
  WHERE id = '01PLAN_BRONZE_000000000';
UPDATE plans SET stripe_price_id = 'price_REDACTED03', updated_at = NOW()
  WHERE id = '01PLAN_SILVER_000000000';
UPDATE plans SET stripe_price_id = 'price_REDACTED04', updated_at = NOW()
  WHERE id = '01PLAN_GOLD_0000000000';
UPDATE plans SET stripe_price_id = 'price_REDACTED05', updated_at = NOW()
  WHERE id = '01PLAN_DIAMOND_00000000';
