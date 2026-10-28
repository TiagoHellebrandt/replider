ALTER TABLE accounts
  DROP COLUMN IF EXISTS onboarding_preview_chat_tested,
  DROP COLUMN IF EXISTS onboarding_first_inbound_at,
  DROP COLUMN IF EXISTS onboarding_checklist_dismissed_at;
