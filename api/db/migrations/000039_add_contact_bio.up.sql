-- Stores the Telegram user's bio fetched via getChat alongside the
-- profile picture. Refreshed on the same 24h cadence as the avatar.
-- WhatsApp / Instagram don't expose a bio through their bot APIs, so
-- this stays NULL for those channels.
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS contact_bio TEXT;
