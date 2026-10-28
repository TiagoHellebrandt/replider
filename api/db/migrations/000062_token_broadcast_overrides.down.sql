UPDATE plans SET max_broadcast_messages_per_month = 0, max_recipients_per_campaign = 0, updated_at = NOW() WHERE id IN ('01PLAN_GOLD_0000000000','01PLAN_DIAMOND_00000000');
ALTER TABLE accounts
  DROP COLUMN max_recipients_per_campaign_override,
  DROP COLUMN max_broadcast_messages_per_month_override,
  DROP COLUMN max_ai_tokens_per_month_override;
