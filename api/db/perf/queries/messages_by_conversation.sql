-- TAG: message thread (GET /v1/conversations/:id/messages)
-- Mirrors ListByConversationID in db/queries/messages.sql.
-- Must be served by idx_messages_conversation_id; a missing index would
-- Seq Scan all 5000 seeded messages.
SELECT m.id, m.content, m.created_at
FROM messages m
WHERE m.conversation_id = 'PERFCONV000000000000000050'
ORDER BY m.created_at DESC
LIMIT 50 OFFSET 0
