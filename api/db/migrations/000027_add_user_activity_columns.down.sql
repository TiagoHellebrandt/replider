ALTER TABLE users
  DROP COLUMN IF EXISTS last_seen_at,
  DROP COLUMN IF EXISTS invite_sent_at;
