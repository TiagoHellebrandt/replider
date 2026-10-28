-- Enforce one channel per WhatsApp phone_number_id across the whole
-- platform. Embedded Signup can race (user double-clicks the popup,
-- or runs it again after a network blip) and produce duplicate channel
-- rows pointing at the same Cloud API number; without uniqueness the
-- inbound webhook GetByPhoneNumberID lookup picks one arbitrarily and
-- the other receives nothing — silent message loss.
--
-- Partial index: only enforced on `provider = 'whatsapp'` rows, so
-- Telegram/Webchat aren't constrained, and only when the phone id is
-- present (NULL allowed for legacy or partially configured rows).
CREATE UNIQUE INDEX IF NOT EXISTS idx_channels_unique_whatsapp_phone_number_id
    ON channels((credentials->>'phone_number_id'))
    WHERE provider = 'whatsapp'
      AND credentials ? 'phone_number_id'
      AND credentials->>'phone_number_id' <> '';
