ALTER TABLE agents
    DROP COLUMN IF EXISTS silent_on_unknown,
    DROP COLUMN IF EXISTS hide_ai_identity;
