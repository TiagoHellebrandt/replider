-- Revert: make channels dependent on agents again.
-- WARNING: channels with agent_id = NULL will fail this migration.
-- You must reassign them to an agent first.

DROP INDEX IF EXISTS idx_channels_account_id;

ALTER TABLE channels DROP CONSTRAINT IF EXISTS channels_agent_id_fkey;
ALTER TABLE channels
  ADD CONSTRAINT channels_agent_id_fkey
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE;

ALTER TABLE channels ALTER COLUMN agent_id SET NOT NULL;

ALTER TABLE channels DROP CONSTRAINT IF EXISTS fk_channels_account;
ALTER TABLE channels DROP COLUMN IF EXISTS account_id;
