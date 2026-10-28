-- =============================================================================
-- Replider — EXPLAIN ANALYZE perf-gate seed (US-062)
-- =============================================================================
-- Loads enough rows into the hot read-path tables that a *missing index* shows
-- up as a Seq Scan over >1k rows. The CI gate (db/perf/check.sh) runs every
-- query under db/perf/queries/ through EXPLAIN (ANALYZE, FORMAT JSON) and fails
-- the build if any Seq Scan touches more than 1000 rows.
--
-- Data shape (chosen so indexed predicates stay selective):
--   * 50 accounts      → a single-account filter returns ~2% of conversations,
--                        so the planner MUST pick idx_conversations_account_status
--                        (a full scan would touch all 2000 rows and trip the gate).
--   * 1 agent + 1 channel (FKs only require existence, not account-consistency).
--   * 2000 conversations spread across the 50 accounts.
--   * 5000 messages spread across the conversations.
--
-- IDs are plain CHAR(26) strings (no ULID validation on these columns); the
-- 'PERF…' prefixes keep them from colliding with the demo seed (migration 5).
-- Idempotent: safe to re-run (ON CONFLICT DO NOTHING + fixed key space).
-- =============================================================================

-- Plan ------------------------------------------------------------------------
INSERT INTO plans (id, name)
VALUES ('PLANPERF000000000000000000', 'Perf Gate')
ON CONFLICT (id) DO NOTHING;

-- 50 accounts: ids 'PERFACCT' + 18-digit counter -> 26 chars ------------------
INSERT INTO accounts (id, name, plan_id)
SELECT 'PERFACCT' || lpad(g::text, 18, '0'),
       'Perf Account ' || g,
       'PLANPERF000000000000000000'
FROM generate_series(1, 50) g
ON CONFLICT (id) DO NOTHING;

-- 1 agent (owned by account #1) ----------------------------------------------
INSERT INTO agents (id, account_id, name)
VALUES ('PERFAGNT000000000000000001', 'PERFACCT000000000000000001', 'Perf Agent')
ON CONFLICT (id) DO NOTHING;

-- 1 channel (FK to the agent; conversations reference it by id) ---------------
INSERT INTO channels (id, account_id, agent_id, provider, credentials, webhook_token)
VALUES ('PERFCHAN000000000000000001', 'PERFACCT000000000000000001',
        'PERFAGNT000000000000000001',
        'telegram', '{}'::jsonb, 'perf-gate-token')
ON CONFLICT (id) DO NOTHING;

-- 2000 conversations across the 50 accounts -----------------------------------
--   account_id   = account (g mod 50) + 1  -> ~40 conversations / account
--   status       = 2/3 'open', 1/3 'closed'
--   waiting_since = set on every 200th row  -> ~10 rows for the partial index
INSERT INTO conversations
    (id, account_id, channel_id, agent_id, contact_id, status,
     last_customer_message_at, waiting_since, created_at)
SELECT 'PERFCONV' || lpad(g::text, 18, '0'),
       'PERFACCT' || lpad(((g % 50) + 1)::text, 18, '0'),
       'PERFCHAN000000000000000001',
       'PERFAGNT000000000000000001',
       'contact-' || g,
       CASE WHEN g % 3 = 0 THEN 'closed' ELSE 'open' END,
       NOW() - (g || ' minutes')::interval,
       CASE WHEN g % 200 = 0 THEN NOW() - (g || ' minutes')::interval ELSE NULL END,
       NOW() - (g || ' minutes')::interval
FROM generate_series(1, 2000) g
ON CONFLICT (id) DO NOTHING;

-- 5000 messages spread round-robin over the conversations ---------------------
INSERT INTO messages
    (id, channel_id, conversation_id, sender, type, content, direction, created_at)
SELECT 'PERFMSG0' || lpad(g::text, 18, '0'),
       'PERFCHAN000000000000000001',
       'PERFCONV' || lpad(((g % 2000) + 1)::text, 18, '0'),
       'contact',
       'text',
       'perf message ' || g,
       'inbound',
       NOW() - (g || ' seconds')::interval
FROM generate_series(1, 5000) g
ON CONFLICT (id) DO NOTHING;

-- Refresh planner statistics so EXPLAIN ANALYZE reflects the real row counts. --
ANALYZE plans, accounts, agents, channels, conversations, messages;
