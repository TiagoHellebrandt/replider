-- Granular onboarding tracking. The legacy `onboarding_completed` boolean
-- only captures whether the user clicked the final "Go to dashboard"
-- button — it can't power the persistent dashboard checklist or the new
-- preview-chat aha-moment step. We add three columns:
--
--  * onboarding_preview_chat_tested:
--      Set to true the first time the user sends a message to their
--      newly-created agent via the preview-chat endpoint. Powers the
--      "Você experimentou seu agente" item in the dashboard checklist.
--
--  * onboarding_first_inbound_at:
--      Timestamp of the first real inbound message ever received by any
--      channel of the account. The "first time it actually worked"
--      milestone — also unlocks the celebratory animation in the
--      checklist card. Idempotent (only written when NULL).
--
--  * onboarding_checklist_dismissed_at:
--      When the user clicks "Dispensar por enquanto" on the checklist
--      card. Card stays hidden for 24h after this timestamp. Cleared
--      automatically the next time progress changes (handled in the
--      use-case layer, not at the DB level — keeps the schema dumb).
ALTER TABLE accounts
  ADD COLUMN onboarding_preview_chat_tested boolean NOT NULL DEFAULT false,
  ADD COLUMN onboarding_first_inbound_at timestamptz NULL,
  ADD COLUMN onboarding_checklist_dismissed_at timestamptz NULL;
