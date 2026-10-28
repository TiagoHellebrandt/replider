DROP INDEX IF EXISTS idx_messages_channel_external_id;
ALTER TABLE messages DROP COLUMN external_message_id;
