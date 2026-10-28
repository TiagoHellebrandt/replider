-- Conversation message loads always read by conversation_id ordered by created_at:
--   SELECT * FROM messages WHERE conversation_id = $1 ORDER BY created_at ASC   (ListByConversation)
--   SELECT * FROM messages WHERE conversation_id = $1 AND created_at > $2 ...   (ListSince)
-- The single-column idx_messages_conversation_id (000001) satisfies the equality
-- but forces a sort on created_at. A composite (conversation_id, created_at) index
-- serves both the filter AND the ordering, making 10k-message loads <100ms.
CREATE INDEX IF NOT EXISTS idx_messages_conversation_created
    ON messages (conversation_id, created_at);

-- The standalone conversation_id index is now fully covered by the composite
-- (a leading-column prefix), so drop it to avoid redundant write amplification.
DROP INDEX IF EXISTS idx_messages_conversation_id;
