-- TAG: last-message subquery (per-row scalar in the conversation list)
-- Mirrors the `(SELECT ... FROM messages WHERE conversation_id = c.id
-- ORDER BY created_at DESC LIMIT 1)` correlated subqueries.
-- Must be served by idx_messages_conversation_id; a missing index would
-- Seq Scan all 5000 seeded messages on every conversation row.
SELECT m.content, m.type
FROM messages m
WHERE m.conversation_id = 'PERFCONV000000000000000050'
ORDER BY m.created_at DESC
LIMIT 1
