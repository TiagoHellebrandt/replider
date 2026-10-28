-- Restore the pre-000061 hardcoded Partner caps from 000033.
UPDATE plans SET
  price                            = 549000,
  max_agents                       = 50,
  max_channels                     = 100,
  max_seats                        = 100,
  max_conversations_per_month      = 60000,
  max_ai_tokens_per_month          = 20000000,
  max_storage_mb                   = 2097152,
  max_broadcast_messages_per_month = 0,
  max_recipients_per_campaign      = 0,
  overage_conversation_cents       = 40,
  overage_cap_multiplier           = 1.50,
  updated_at                       = NOW()
WHERE id = '01PLAN_PARTNER_00000000';
