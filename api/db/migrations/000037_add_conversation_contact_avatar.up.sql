-- Store the contact's profile picture URL on the conversation row so the
-- dashboard can render an avatar without a second roundtrip. Populated
-- for Telegram from getUserProfilePhotos+getFile at the moment the
-- conversation is first created. Nullable because WhatsApp / Instagram
-- don't expose profile photos via webhook and the picture is a best-
-- effort enhancement, not a requirement for messaging.
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS contact_avatar_url TEXT;
