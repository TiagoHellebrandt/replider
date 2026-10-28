-- Restore the original single-column index, then drop the composite.
CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages (conversation_id);
DROP INDEX IF EXISTS idx_messages_conversation_created;
