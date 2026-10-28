-- WhatsApp billing + delivery metadata.
--
-- The Cloud API attaches a `conversation` and `pricing` envelope to the
-- first status update of every billable 24h window, plus per-status
-- timestamps and an error reason on failures. Without persisting these,
-- Replider can't:
--   - meter usage accurately to Stripe (different categories cost
--     different amounts: marketing ≫ utility > service)
--   - drive the 24h-window UI indicator that tells agents whether a
--     plain text reply is still allowed
--   - show the agent why a send failed (right now we just say
--     "delivery failed")
--
-- All columns are nullable so existing rows survive without backfill.
-- conversation_origin / pricing_category use TEXT (vs an enum) because
-- Meta keeps adding categories and we don't want a migration on each
-- one.

-- read_at is already added by migration 000008 (the inbox-read flag).
-- We reuse it for outbound read receipts as well — same semantic
-- ("customer's eyes on this row"), one column either way.
ALTER TABLE messages
    ADD COLUMN delivered_at         TIMESTAMPTZ,
    ADD COLUMN failed_reason        TEXT,
    ADD COLUMN wa_conversation_id   TEXT,
    ADD COLUMN wa_conversation_expires_at TIMESTAMPTZ,
    ADD COLUMN conversation_origin  TEXT,    -- marketing/utility/authentication/service/referral_conversion
    ADD COLUMN pricing_category     TEXT,
    ADD COLUMN pricing_billable     BOOLEAN;

-- Lookup index for the campaign worker / billing rollups that
-- aggregate by conversation. Partial because most messages don't carry
-- a wa_conversation_id (only the first one of a window does).
CREATE INDEX IF NOT EXISTS idx_messages_wa_conversation
    ON messages(wa_conversation_id)
    WHERE wa_conversation_id IS NOT NULL;
