-- Tracks when the contact's profile picture was last fetched from the
-- provider (Telegram today). The inbound webhook uses this to decide
-- whether to re-call getUserProfilePhotos — avoiding an API roundtrip on
-- every message while still catching photo changes within a day.
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS contact_avatar_fetched_at TIMESTAMPTZ;

-- Backfill: existing rows with a stored avatar are assumed to be fresh
-- so the first inbound after the migration doesn't immediately re-fetch
-- everything. They'll refresh on the 24h cadence from here on.
UPDATE conversations
SET contact_avatar_fetched_at = NOW()
WHERE contact_avatar_url IS NOT NULL
  AND contact_avatar_fetched_at IS NULL;
