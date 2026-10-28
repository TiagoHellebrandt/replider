-- TAG: waiting-since sweep (SLA / "needs reply" worker)
-- Mirrors the ORDER BY waiting_since query in db/queries/conversations.sql.
-- Must be served by the partial idx_conversations_waiting_since
-- (WHERE waiting_since IS NOT NULL); a missing index would Seq Scan all
-- 2000 seeded conversations to find the ~10 waiting ones.
SELECT c.id, c.waiting_since
FROM conversations c
WHERE c.waiting_since IS NOT NULL
ORDER BY c.waiting_since ASC NULLS LAST, c.created_at ASC
LIMIT 50
