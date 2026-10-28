ALTER TABLE accounts
  DROP COLUMN IF EXISTS onboarding_draft,
  DROP COLUMN IF EXISTS onboarding_current_step;
