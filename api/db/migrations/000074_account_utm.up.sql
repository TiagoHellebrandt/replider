-- Capture first-touch acquisition attribution on signup. The marketing
-- site (web/) appends utm_* to the register CTA; the Flutter app forwards
-- them in the create-account payload. Nullable: social-auth and direct
-- signups arrive without UTM.
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS utm_source TEXT;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS utm_medium TEXT;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS utm_campaign TEXT;
