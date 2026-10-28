-- Webhook deduplication. Meta retries the same webhook when our handler
-- takes too long or returns non-2xx, so the same wamid can arrive multiple
-- times. Without a unique constraint, we'd save duplicate inbound messages
-- and — worse — run the AI twice and ship two replies to the same customer.
--
-- The column is nullable because historical rows and non-WhatsApp channels
-- (Telegram, webchat) may not carry a provider message id. The partial
-- unique index only enforces uniqueness where the id is actually set.
ALTER TABLE messages
    ADD COLUMN external_message_id TEXT;

CREATE UNIQUE INDEX idx_messages_channel_external_id
    ON messages(channel_id, external_message_id)
    WHERE external_message_id IS NOT NULL;
