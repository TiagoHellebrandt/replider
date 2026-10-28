-- Hands-off toggles on agents. Both default TRUE so every existing
-- agent immediately adopts the "indistinguishable from a human
-- attendant" behavior — the owner opts out if they want the bot to
-- openly admit it's a virtual assistant or to tell the customer
-- "I don't know, an attendant will follow up".
ALTER TABLE agents
    ADD COLUMN IF NOT EXISTS hide_ai_identity  BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS silent_on_unknown BOOLEAN NOT NULL DEFAULT TRUE;
