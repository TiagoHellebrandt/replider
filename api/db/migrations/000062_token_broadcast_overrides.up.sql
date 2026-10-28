-- Two follow-ups in one migration:
--
-- 1. Per-account override columns for the three caps that 000060 left
--    behind (tokens, broadcast messages, recipients per campaign). The
--    existing override columns covered agents/channels/seats/conv/storage;
--    contract customers (Partner / Enterprise) were stuck at the plan-row
--    defaults for the rest, which forced new `plans` rows for every
--    bespoke deal — exactly what the override table was meant to avoid.
--
-- 2. Backfill broadcast caps for Business + Scale. Migration 000017
--    seeded broadcast limits for Trial / Starter / Pro / Enterprise (the
--    tier names that existed at the time), then 000033 renamed and
--    repriced the grid without re-seeding broadcast — so Business + Scale
--    silently shipped with cap = 0, which CreateCampaignUseCase treats as
--    "broadcasts not allowed" and rejects with BROADCAST_NOT_ALLOWED.
--    Customers paying R$ 1,990 / R$ 3,990 could not run a single
--    campaign, while Pro at R$ 899 could. Restoring the expected ladder.
ALTER TABLE accounts
  ADD COLUMN max_ai_tokens_per_month_override          INT,
  ADD COLUMN max_broadcast_messages_per_month_override INT,
  ADD COLUMN max_recipients_per_campaign_override      INT;

UPDATE plans
   SET max_broadcast_messages_per_month = 25000,
       max_recipients_per_campaign      = 10000,
       updated_at                       = NOW()
 WHERE id = '01PLAN_GOLD_0000000000';

UPDATE plans
   SET max_broadcast_messages_per_month = 50000,
       max_recipients_per_campaign      = 25000,
       updated_at                       = NOW()
 WHERE id = '01PLAN_DIAMOND_00000000';
