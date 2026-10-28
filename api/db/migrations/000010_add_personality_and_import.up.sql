-- Add personality_prompt to agents: auto-generated from analyzing human messages
-- This is separate from system_prompt so it can be auto-updated without touching manual config
ALTER TABLE agents ADD COLUMN IF NOT EXISTS personality_prompt TEXT;

-- Add learn_from_humans flag: when true, the system analyzes human replies to build personality
ALTER TABLE agents ADD COLUMN IF NOT EXISTS learn_from_humans BOOLEAN DEFAULT false;
