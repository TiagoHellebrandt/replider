-- Overage metering (prep only — enforcement lives in a separate Go PR).
-- See docs/OVERAGE_METERING_DESIGN.md for the billing rules.
--
-- plans gains the per-conversation overage rate (centavos) and a cap
-- multiplier so "never pay more than Nx the plan" is a single lookup.
-- account_usages gains per-cycle counters the worker updates once
-- conversations_count exceeds max_conversations_per_month, plus a
-- cap_reached_at marker so we stop double-charging on the same cycle.

ALTER TABLE plans
  ADD COLUMN IF NOT EXISTS overage_conversation_cents INT          NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS overage_cap_multiplier     NUMERIC(4,2) NOT NULL DEFAULT 2.00;

ALTER TABLE account_usages
  ADD COLUMN IF NOT EXISTS overage_conversations_count INT         NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS overage_cents_charged        INT         NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS cap_reached_at               TIMESTAMPTZ NULL;

-- Per-plan overage rates (centavos BRL per conversation, per grid):
--   Solo:      R$ 0,90 =  90¢
--   Starter:   R$ 0,70 =  70¢
--   Pro:       R$ 0,50 =  50¢
--   Business:  R$ 0,40 =  40¢
--   Scale:     R$ 0,35 =  35¢
--   Partner:   R$ 0,40 =  40¢  (cap multiplier 1.5× — handshake deal)

UPDATE plans SET overage_conversation_cents = 90, overage_cap_multiplier = 2.00
  WHERE id = '01PLAN_SOLO_00000000000';
UPDATE plans SET overage_conversation_cents = 70, overage_cap_multiplier = 2.00
  WHERE id = '01PLAN_BRONZE_000000000';
UPDATE plans SET overage_conversation_cents = 50, overage_cap_multiplier = 2.00
  WHERE id = '01PLAN_SILVER_000000000';
UPDATE plans SET overage_conversation_cents = 40, overage_cap_multiplier = 2.00
  WHERE id = '01PLAN_GOLD_0000000000';
UPDATE plans SET overage_conversation_cents = 35, overage_cap_multiplier = 2.00
  WHERE id = '01PLAN_DIAMOND_00000000';
UPDATE plans SET overage_conversation_cents = 40, overage_cap_multiplier = 1.50
  WHERE id = '01PLAN_PARTNER_00000000';
