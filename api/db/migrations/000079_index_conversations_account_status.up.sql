-- The dashboard conversation list always filters by (account_id, status) and,
-- on its default tab, orders by last_customer_message_at DESC NULLS LAST then
-- created_at DESC:
--   SELECT ... FROM conversations c
--   WHERE c.account_id = $1 AND c.status = $2
--   ORDER BY c.last_customer_message_at DESC NULLS LAST, c.created_at DESC   (ListConversationsEnriched)
--
-- The existing idx_conversations_account_status (account_id, status) from
-- 000001 satisfies the equality filter but forces a separate sort on every
-- page load. A composite that appends the ORDER BY columns (in the exact
-- direction/NULLS ordering the query uses) lets Postgres serve the filter AND
-- the ordering from one index — an index scan with no Sort node.
--
-- EXPLAIN confirms use, e.g.:
--   EXPLAIN SELECT * FROM conversations
--   WHERE account_id = '...' AND status = 'open'
--   ORDER BY last_customer_message_at DESC NULLS LAST, created_at DESC LIMIT 50;
--   -> Index Scan using idx_conversations_account_status_recent  (no Sort)
CREATE INDEX IF NOT EXISTS idx_conversations_account_status_recent
    ON conversations (account_id, status, last_customer_message_at DESC NULLS LAST, created_at DESC);

-- The old (account_id, status) index is now a strict leading-column prefix of
-- the composite above, so it is fully covered. Drop it to avoid redundant
-- write amplification — the other account+status queries (oldest, first-in-conv,
-- by-waiting) still use the composite's (account_id, status) prefix for filtering.
DROP INDEX IF EXISTS idx_conversations_account_status;
