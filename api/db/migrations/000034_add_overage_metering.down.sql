ALTER TABLE account_usages
  DROP COLUMN IF EXISTS cap_reached_at,
  DROP COLUMN IF EXISTS overage_cents_charged,
  DROP COLUMN IF EXISTS overage_conversations_count;

ALTER TABLE plans
  DROP COLUMN IF EXISTS overage_cap_multiplier,
  DROP COLUMN IF EXISTS overage_conversation_cents;
