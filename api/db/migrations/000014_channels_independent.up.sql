-- Make channels independent of agents. A channel now belongs to an account
-- directly, and optionally has an agent assigned for AI responses.
--
-- Before: channels.agent_id NOT NULL → channels always need an agent
-- After:  channels.account_id NOT NULL + channels.agent_id nullable

-- 1. Add account_id (nullable first so we can backfill)
ALTER TABLE channels ADD COLUMN account_id CHAR(26);

-- 2. Backfill from the agent's account
UPDATE channels c
SET account_id = a.account_id
FROM agents a
WHERE c.agent_id = a.id;

-- 3. Enforce NOT NULL + FK now that every row has a value
ALTER TABLE channels
  ALTER COLUMN account_id SET NOT NULL;

ALTER TABLE channels
  ADD CONSTRAINT fk_channels_account
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE;

-- 4. Make agent_id nullable
ALTER TABLE channels ALTER COLUMN agent_id DROP NOT NULL;

-- 5. Change cascade: deleting an agent no longer deletes its channels,
-- it just unlinks them (agent_id → NULL). The channel stays operational
-- for human-only support.
ALTER TABLE channels DROP CONSTRAINT channels_agent_id_fkey;
ALTER TABLE channels
  ADD CONSTRAINT channels_agent_id_fkey
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL;

-- 6. Index for the new primary lookup pattern
CREATE INDEX idx_channels_account_id ON channels(account_id);
