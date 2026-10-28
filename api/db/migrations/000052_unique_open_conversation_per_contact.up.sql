-- Two parallel inbound webhooks for the same (channel_id, contact_id) used to
-- both pass FindByContact (returned nil) and both INSERT a fresh row, leaving
-- the dashboard with duplicate "open" conversations and bumping the monthly
-- conversations counter twice. SaveInbound's read-then-write path is fixed in
-- code (try INSERT, on conflict re-read), but the safety net lives here: a
-- partial unique index that the database itself enforces.
--
-- Partial — closed conversations are intentionally excluded so a contact who
-- was wrapped up and writes back later still gets a new row (closed history
-- is preserved). Two open rows for one contact never make sense.
--
-- IF NOT EXISTS so a re-run on environments that pre-applied the index by
-- hand (or via a hotfix in prod) doesn't fail the migration.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_conversations_open_per_contact
  ON conversations(channel_id, contact_id)
  WHERE status != 'closed';
