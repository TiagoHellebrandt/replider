-- TAG: conversation list page (GET /v1/conversations, default sort)
-- Mirrors ListConversationsEnriched WHERE/ORDER in db/queries/conversations.sql.
-- Must be served by idx_conversations_account_status (account_id, status);
-- a missing index would Seq Scan all 2000 seeded conversations.
SELECT c.id, c.status, c.last_customer_message_at, c.created_at
FROM conversations c
WHERE c.account_id = 'PERFACCT000000000000000001'
  AND c.status = 'open'
ORDER BY c.last_customer_message_at DESC NULLS LAST, c.created_at DESC
LIMIT 50 OFFSET 0
