DELETE FROM plans WHERE id = '01PLAN_ENTERPRISE_00000';

ALTER TABLE accounts
  DROP COLUMN IF EXISTS openai_api_key_encrypted,
  DROP COLUMN IF EXISTS gemini_api_key_encrypted,
  DROP COLUMN IF EXISTS max_agents_override,
  DROP COLUMN IF EXISTS max_channels_override,
  DROP COLUMN IF EXISTS max_seats_override,
  DROP COLUMN IF EXISTS max_conversations_per_month_override,
  DROP COLUMN IF EXISTS max_storage_mb_override;
