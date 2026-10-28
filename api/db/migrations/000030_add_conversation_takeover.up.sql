-- Human Takeover: stamp the moment an attendant assumed the conversation.
-- The existing columns already cover the takeover state:
--   * conversations.is_bot_paused   — whether the AI should short-circuit
--   * conversations.assigned_to     — which user is responsible (FK users)
-- Only the "when" piece was missing; add it so the UI can show
-- "Em atendimento por X desde HH:MM" and so analytics can compute
-- takeover duration without scanning the audit log.
ALTER TABLE conversations
    ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMPTZ NULL;
