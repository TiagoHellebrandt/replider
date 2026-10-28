-- name: CreateConversation :exec
-- contact_avatar_fetched_at mirrors contact_avatar_url: NOW() if an
-- avatar was captured at creation, NULL otherwise. The inbound pipeline
-- uses the timestamp to decide whether to re-fetch on later messages.
--
-- contact_id and contact_ref_id are two different things and both are
-- written here: contact_id is the sender as the CHANNEL wrote it (routing
-- the next inbound needs it verbatim), contact_ref_id is the FK to the
-- resolved contacts row. NULL on contact_ref_id is legitimate — identity
-- resolution is best-effort and must never block a customer's message.
INSERT INTO conversations (id, account_id, channel_id, agent_id, contact_id, contact_name, contact_username, contact_avatar_url, contact_avatar_fetched_at, contact_bio, status, is_bot_paused, last_customer_message_at, waiting_since, contact_ref_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CASE WHEN $8::text IS NOT NULL THEN NOW() END, $9, $10, $11, $12, $13, $14, NOW(), NOW());

-- name: SetConversationCustomerAccount :exec
-- US-002 (canal-interno): stamps the CUSTOMER account on a platform
-- internal-channel thread right after creation. Write-once — the guard
-- keeps a later caller from re-pointing an existing thread at another
-- account.
UPDATE conversations
SET customer_account_id = $2, updated_at = NOW()
WHERE id = $1 AND customer_account_id IS NULL;

-- name: SetConversationLanguage :exec
-- Persists the language detected from the first inbound message. Called
-- once per conversation; subsequent detections are no-ops (guarded by
-- the WHERE clause) so mid-conversation language switches don't flap.
UPDATE conversations
SET language = $2, updated_at = NOW()
WHERE id = $1 AND language IS NULL;

-- name: RefreshContactAvatar :exec
-- Used by the inbound Telegram handler to refresh the cached profile
-- fields (avatar, bio) and bump the fetched-at cursor. Safe to call
-- with the same values — fetched_at still advances.
UPDATE conversations
SET contact_avatar_url = $3,
    contact_bio = $4,
    contact_avatar_fetched_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND account_id = $2;

-- name: FindConversationByContact :one
SELECT * FROM conversations
WHERE channel_id = $1 AND contact_id = $2 AND status != 'closed'
ORDER BY created_at DESC LIMIT 1;

-- name: FindLatestConversationByContact :one
-- Same key as FindConversationByContact but ANY status. The inbound
-- pipeline asks this after the open lookup came back empty: a customer
-- answering a thread the attendant resolved must land back on that
-- thread (reopened), not on a fresh one without the history.
SELECT * FROM conversations
WHERE channel_id = $1 AND contact_id = $2
ORDER BY created_at DESC LIMIT 1;

-- name: ReopenConversation :execrows
-- Resolve's inverse. waiting_since restarts because whoever reopens (a
-- customer's reply, or an attendant) is asking for attention again.
-- Guarded on status so a double call is a no-op (0 rows), and the
-- partial unique index uniq_conversations_open_per_contact still rejects
-- a reopen while another thread for the same sender is open.
UPDATE conversations
SET status = 'open',
    resolved_at = NULL,
    waiting_since = NOW(),
    updated_at = NOW()
WHERE id = $1 AND account_id = $2 AND status = 'closed';

-- name: GetContactStats :one
-- Aggregates lifetime stats for a single contact (phone) inside the
-- account so the contact detail sheet can show "cliente desde X /
-- Y conversas / Z mensagens" without scanning client-side.
--
-- Scoped by account_id so cross-tenant phone collisions never leak
-- counts between workspaces.
SELECT
    COUNT(DISTINCT c.id)::bigint                   AS total_conversations,
    COUNT(m.id)::bigint                            AS total_messages,
    MIN(c.created_at)::timestamptz                 AS first_contact_at,
    MAX(c.last_customer_message_at)::timestamptz   AS last_message_at
FROM conversations c
LEFT JOIN messages m ON m.conversation_id = c.id
WHERE c.account_id = $1 AND c.contact_id = $2;

-- name: GetConversationByID :one
SELECT * FROM conversations
WHERE id = $1 AND account_id = $2 LIMIT 1;

-- name: UpdateConversationSummary :exec
-- US-024: store the rolling summary of older (aged-out) turns. Scoped by
-- account_id so a guessed conversation id can't overwrite another tenant's
-- summary.
UPDATE conversations
SET summary = $3,
    summary_updated_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND account_id = $2;

-- name: IncrementConversationCost :exec
-- US-030: fold a single AI turn's estimated provider cost into the
-- conversation's running total. Scoped by account_id so a guessed
-- conversation id can't poison another tenant's cost analytics.
UPDATE conversations
SET total_cost_usd = total_cost_usd + $3,
    updated_at = NOW()
WHERE id = $1 AND account_id = $2;

-- name: UpdateConversation :exec
UPDATE conversations
SET status = $2,
    is_bot_paused = $3,
    agent_id = $4,
    assigned_to = $5,
    priority = $6,
    first_response_at = $7,
    resolved_at = $8,
    updated_at = NOW()
WHERE id = $1 AND account_id = $9;

-- name: AssignConversation :exec
-- Routing-only assign: stamps assigned_to without touching is_bot_paused
-- or assigned_at. Used by auto-assign and by the existing "route to
-- teammate" flow where the bot keeps answering until a human actually
-- takes over via AssumeConversation.
UPDATE conversations
SET assigned_to = $2,
    updated_at = NOW()
WHERE id = $1 AND account_id = $3;

-- name: SetFirstInboundAt :exec
-- Mission 6: stamp the first-ever inbound timestamp used to compute
-- avg TTFR. Idempotent: the `IS NULL` guard means subsequent inbound
-- messages on the same conversation don't overwrite the first.
UPDATE conversations
SET first_inbound_at = COALESCE(first_inbound_at, NOW()),
    updated_at = NOW()
WHERE id = $1 AND account_id = $2 AND first_inbound_at IS NULL;

-- name: SetFirstResponse :exec
-- Sets first_response_at + first_response_by the first time anyone
-- (bot or human) answers. `IS NULL` guard keeps later replies from
-- overwriting the original TTFR datapoint.
UPDATE conversations
SET first_response_at = COALESCE(first_response_at, NOW()),
    first_response_by = COALESCE(first_response_by, $3),
    updated_at = NOW()
WHERE id = $1 AND account_id = $2 AND first_response_at IS NULL;

-- name: AssumeConversation :exec
-- Human Takeover: attendant explicitly takes the conversation. Stamps
-- assigned_at = NOW() and pauses the bot in one atomic write so the
-- next inbound message cannot race the AI path.
UPDATE conversations
SET assigned_to = $2,
    assigned_at = NOW(),
    is_bot_paused = TRUE,
    updated_at = NOW()
WHERE id = $1 AND account_id = $3;

-- name: ReleaseConversation :exec
-- Inverse of AssumeConversation: clears the human owner and resumes
-- the bot so it handles the next inbound. Only valid for the account's
-- own conversations (tenant isolation).
UPDATE conversations
SET assigned_to = NULL,
    assigned_at = NULL,
    is_bot_paused = FALSE,
    updated_at = NOW()
WHERE id = $1 AND account_id = $2;

-- name: GetConversationAssignment :one
-- Joins with users so the dashboard can render "Em atendimento por X"
-- without a second roundtrip. Returns NULLs for user fields when the
-- conversation has no human owner.
SELECT
    c.id AS conversation_id,
    c.assigned_to,
    c.assigned_at,
    c.is_bot_paused,
    u.name AS assigned_user_name,
    u.email AS assigned_user_email
FROM conversations c
LEFT JOIN users u ON u.id = c.assigned_to
WHERE c.id = $1 AND c.account_id = $2
LIMIT 1;

-- name: GetActiveAttendants :many
-- Auto-assign pool. Includes 'owner'/'gestor' (management roles that handle
-- conversations) — without 'owner' a solo account (only the creator, now
-- role='owner' since migration 000149) had zero assignable users and every
-- inbound conversation auto-assigned to nobody.
SELECT id FROM users
WHERE account_id = $1 AND role IN ('owner', 'admin', 'gestor', 'attendant') AND is_active = true
ORDER BY id;

-- name: ListConversationsByAccountAndStatus :many
SELECT * FROM conversations
WHERE account_id = $1 AND status = $2
ORDER BY waiting_since ASC NULLS LAST, created_at ASC;

-- name: ListConversationsEnriched :many
-- Default sort — most recent customer activity first. Matches the
-- WhatsApp Web mental model so the attendant scans top-down and the
-- threads they need are already at the top.
SELECT
    c.id,
    c.account_id,
    c.channel_id,
    c.agent_id,
    c.contact_id,
    COALESCE(c.contact_name, c.contact_id) AS contact_name,
    c.contact_username,
    c.contact_avatar_url,
    c.contact_avatar_fetched_at,
    c.contact_bio,
    c.status,
    c.is_bot_paused,
    c.assigned_to,
    c.assigned_at,
    c.priority,
    c.last_customer_message_at,
    c.waiting_since,
    c.created_at,
    c.updated_at,
    ch.provider AS channel_type,
    u.name AS assigned_user_name,
    c.sentiment,
    c.sentiment_score,
    (SELECT m.content FROM messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC LIMIT 1) AS last_message,
    COALESCE((SELECT m.type FROM messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC LIMIT 1), '')::TEXT AS last_message_type,
    -- Lead context for the inbox chip (US-M024). LATERAL rather than two
    -- correlated subqueries so the id and the stage always describe the
    -- SAME lead — independent subqueries could pick different rows when
    -- updated_at ties.
    COALESCE(ld.lead_id, '')::TEXT AS lead_id,
    COALESCE(ld.stage_name, '')::TEXT AS lead_stage_name,
    COALESCE(ld.status, '')::TEXT AS lead_status,
    -- Inbox triage (§6.10 camada 3). Read here rather than computed at
    -- render time so the ordering and the filter both run in SQL, before
    -- LIMIT/OFFSET.
    c.inbox_category,
    c.inbox_importance,
    -- Unread = customer messages nobody opened yet (messages.read_at is
    -- stamped by POST /conversations/:id/read). The tile used to fake it
    -- from status = pending_human, which is a different fact.
    (SELECT COUNT(*) FROM messages m
      WHERE m.conversation_id = c.id
        AND m.direction = 'inbound'
        AND m.read_at IS NULL)::int AS unread_count
FROM conversations c
JOIN channels ch ON ch.id = c.channel_id
LEFT JOIN users u ON u.id = c.assigned_to
LEFT JOIN LATERAL (
    SELECT l.id AS lead_id, l.status, fs.name AS stage_name
    FROM leads l
    LEFT JOIN funnel_stages fs ON fs.id = l.stage_id
    -- contact_ref_id, NOT contact_id: a lead holds the contact ULID and
    -- contact_id holds the raw channel sender, so the old join never
    -- matched and the inbox chip was blank for every lead. NULL ref
    -- (identity resolution is best-effort) simply yields no lead.
    WHERE l.account_id = c.account_id
      AND c.contact_ref_id IS NOT NULL
      AND l.contact_id = c.contact_ref_id
    ORDER BY l.updated_at DESC
    LIMIT 1
) ld ON TRUE
WHERE c.account_id = $1 AND c.status = $2
  AND (
    sqlc.narg('assigned_to')::text IS NULL
    OR (sqlc.narg('assigned_to')::text = 'none' AND c.assigned_to IS NULL)
    OR (sqlc.narg('assigned_to')::text <> 'none' AND c.assigned_to = sqlc.narg('assigned_to'))
  )
  -- Channel filter runs HERE, before LIMIT/OFFSET. Filtering the page in
  -- Go would drop every email thread that ranked past the page limit, and
  -- the inbox would answer "não há e-mails" while the account had dozens.
  AND (sqlc.narg('channel_type')::text IS NULL OR ch.provider = sqlc.narg('channel_type'))
  -- Same reason as the channel filter: filtering a page in Go shows the
  -- wrong 50 rows and answers "nada aqui" for an account that has plenty.
  AND (sqlc.narg('inbox_category')::text IS NULL OR c.inbox_category = sqlc.narg('inbox_category'))
  -- Search and priority run here as well. They used to filter the page in
  -- Go, which answered "nada encontrado" for a name sitting on page two.
  -- The caller escapes %, _ and \ in q (ILIKE's default escape is \).
  AND (
    sqlc.narg('q')::text IS NULL
    OR c.contact_name ILIKE '%' || sqlc.narg('q')::text || '%'
    OR c.contact_username ILIKE '%' || sqlc.narg('q')::text || '%'
    OR c.contact_id ILIKE '%' || sqlc.narg('q')::text || '%'
    OR (SELECT m.content FROM messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC LIMIT 1)
       ILIKE '%' || sqlc.narg('q')::text || '%'
  )
  AND (sqlc.narg('priority')::text IS NULL OR c.priority = sqlc.narg('priority'))
ORDER BY c.last_customer_message_at DESC NULLS LAST, c.created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListConversationsEnrichedByImportance :many
-- Triage sort (§6.10 camada 3): a lead who answered outranks a newsletter
-- that arrived a minute ago. Chronological order is right for a chat and
-- wrong for a commercial mailbox, where the newest thing is usually the
-- least important. Ties fall back to recency so the order stays stable.
SELECT
    c.id,
    c.account_id,
    c.channel_id,
    c.agent_id,
    c.contact_id,
    COALESCE(c.contact_name, c.contact_id) AS contact_name,
    c.contact_username,
    c.contact_avatar_url,
    c.contact_avatar_fetched_at,
    c.contact_bio,
    c.status,
    c.is_bot_paused,
    c.assigned_to,
    c.assigned_at,
    c.priority,
    c.last_customer_message_at,
    c.waiting_since,
    c.created_at,
    c.updated_at,
    ch.provider AS channel_type,
    u.name AS assigned_user_name,
    c.sentiment,
    c.sentiment_score,
    (SELECT m.content FROM messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC LIMIT 1) AS last_message,
    COALESCE((SELECT m.type FROM messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC LIMIT 1), '')::TEXT AS last_message_type,
    -- Lead context for the inbox chip (US-M024). LATERAL rather than two
    -- correlated subqueries so the id and the stage always describe the
    -- SAME lead — independent subqueries could pick different rows when
    -- updated_at ties.
    COALESCE(ld.lead_id, '')::TEXT AS lead_id,
    COALESCE(ld.stage_name, '')::TEXT AS lead_stage_name,
    COALESCE(ld.status, '')::TEXT AS lead_status,
    -- Inbox triage (§6.10 camada 3). Read here rather than computed at
    -- render time so the ordering and the filter both run in SQL, before
    -- LIMIT/OFFSET.
    c.inbox_category,
    c.inbox_importance,
    -- Unread = customer messages nobody opened yet (messages.read_at is
    -- stamped by POST /conversations/:id/read). The tile used to fake it
    -- from status = pending_human, which is a different fact.
    (SELECT COUNT(*) FROM messages m
      WHERE m.conversation_id = c.id
        AND m.direction = 'inbound'
        AND m.read_at IS NULL)::int AS unread_count
FROM conversations c
JOIN channels ch ON ch.id = c.channel_id
LEFT JOIN users u ON u.id = c.assigned_to
LEFT JOIN LATERAL (
    SELECT l.id AS lead_id, l.status, fs.name AS stage_name
    FROM leads l
    LEFT JOIN funnel_stages fs ON fs.id = l.stage_id
    -- contact_ref_id, NOT contact_id: a lead holds the contact ULID and
    -- contact_id holds the raw channel sender, so the old join never
    -- matched and the inbox chip was blank for every lead. NULL ref
    -- (identity resolution is best-effort) simply yields no lead.
    WHERE l.account_id = c.account_id
      AND c.contact_ref_id IS NOT NULL
      AND l.contact_id = c.contact_ref_id
    ORDER BY l.updated_at DESC
    LIMIT 1
) ld ON TRUE
WHERE c.account_id = $1 AND c.status = $2
  AND (
    sqlc.narg('assigned_to')::text IS NULL
    OR (sqlc.narg('assigned_to')::text = 'none' AND c.assigned_to IS NULL)
    OR (sqlc.narg('assigned_to')::text <> 'none' AND c.assigned_to = sqlc.narg('assigned_to'))
  )
  -- Channel filter runs HERE, before LIMIT/OFFSET. Filtering the page in
  -- Go would drop every email thread that ranked past the page limit, and
  -- the inbox would answer "não há e-mails" while the account had dozens.
  AND (sqlc.narg('channel_type')::text IS NULL OR ch.provider = sqlc.narg('channel_type'))
  -- Same reason as the channel filter: filtering a page in Go shows the
  -- wrong 50 rows and answers "nada aqui" for an account that has plenty.
  AND (sqlc.narg('inbox_category')::text IS NULL OR c.inbox_category = sqlc.narg('inbox_category'))
  -- Search and priority run here as well. They used to filter the page in
  -- Go, which answered "nada encontrado" for a name sitting on page two.
  -- The caller escapes %, _ and \ in q (ILIKE's default escape is \).
  AND (
    sqlc.narg('q')::text IS NULL
    OR c.contact_name ILIKE '%' || sqlc.narg('q')::text || '%'
    OR c.contact_username ILIKE '%' || sqlc.narg('q')::text || '%'
    OR c.contact_id ILIKE '%' || sqlc.narg('q')::text || '%'
    OR (SELECT m.content FROM messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC LIMIT 1)
       ILIKE '%' || sqlc.narg('q')::text || '%'
  )
  AND (sqlc.narg('priority')::text IS NULL OR c.priority = sqlc.narg('priority'))
ORDER BY c.inbox_importance DESC, c.last_customer_message_at DESC NULLS LAST, c.created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListConversationsEnrichedByFirstInConv :many
-- "Oldest waiting customer first". Surfaces threads where the
-- customer messaged Monday and got forgotten when new traffic arrived
-- Friday. Uses first_inbound_at with a created_at fallback for rows
-- where the timestamp was never stamped.
SELECT
    c.id,
    c.account_id,
    c.channel_id,
    c.agent_id,
    c.contact_id,
    COALESCE(c.contact_name, c.contact_id) AS contact_name,
    c.contact_username,
    c.contact_avatar_url,
    c.contact_avatar_fetched_at,
    c.contact_bio,
    c.status,
    c.is_bot_paused,
    c.assigned_to,
    c.assigned_at,
    c.priority,
    c.last_customer_message_at,
    c.waiting_since,
    c.created_at,
    c.updated_at,
    ch.provider AS channel_type,
    u.name AS assigned_user_name,
    c.sentiment,
    c.sentiment_score,
    (SELECT m.content FROM messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC LIMIT 1) AS last_message,
    COALESCE((SELECT m.type FROM messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC LIMIT 1), '')::TEXT AS last_message_type,
    -- Lead context for the inbox chip (US-M024). LATERAL rather than two
    -- correlated subqueries so the id and the stage always describe the
    -- SAME lead — independent subqueries could pick different rows when
    -- updated_at ties.
    COALESCE(ld.lead_id, '')::TEXT AS lead_id,
    COALESCE(ld.stage_name, '')::TEXT AS lead_stage_name,
    COALESCE(ld.status, '')::TEXT AS lead_status,
    -- Inbox triage (§6.10 camada 3). Read here rather than computed at
    -- render time so the ordering and the filter both run in SQL, before
    -- LIMIT/OFFSET.
    c.inbox_category,
    c.inbox_importance,
    -- Unread = customer messages nobody opened yet (messages.read_at is
    -- stamped by POST /conversations/:id/read). The tile used to fake it
    -- from status = pending_human, which is a different fact.
    (SELECT COUNT(*) FROM messages m
      WHERE m.conversation_id = c.id
        AND m.direction = 'inbound'
        AND m.read_at IS NULL)::int AS unread_count
FROM conversations c
JOIN channels ch ON ch.id = c.channel_id
LEFT JOIN users u ON u.id = c.assigned_to
LEFT JOIN LATERAL (
    SELECT l.id AS lead_id, l.status, fs.name AS stage_name
    FROM leads l
    LEFT JOIN funnel_stages fs ON fs.id = l.stage_id
    -- contact_ref_id, NOT contact_id: a lead holds the contact ULID and
    -- contact_id holds the raw channel sender, so the old join never
    -- matched and the inbox chip was blank for every lead. NULL ref
    -- (identity resolution is best-effort) simply yields no lead.
    WHERE l.account_id = c.account_id
      AND c.contact_ref_id IS NOT NULL
      AND l.contact_id = c.contact_ref_id
    ORDER BY l.updated_at DESC
    LIMIT 1
) ld ON TRUE
WHERE c.account_id = $1 AND c.status = $2
  AND (
    sqlc.narg('assigned_to')::text IS NULL
    OR (sqlc.narg('assigned_to')::text = 'none' AND c.assigned_to IS NULL)
    OR (sqlc.narg('assigned_to')::text <> 'none' AND c.assigned_to = sqlc.narg('assigned_to'))
  )
  -- Channel filter runs HERE, before LIMIT/OFFSET. Filtering the page in
  -- Go would drop every email thread that ranked past the page limit, and
  -- the inbox would answer "não há e-mails" while the account had dozens.
  AND (sqlc.narg('channel_type')::text IS NULL OR ch.provider = sqlc.narg('channel_type'))
  -- Same reason as the channel filter: filtering a page in Go shows the
  -- wrong 50 rows and answers "nada aqui" for an account that has plenty.
  AND (sqlc.narg('inbox_category')::text IS NULL OR c.inbox_category = sqlc.narg('inbox_category'))
  -- Search and priority run here as well. They used to filter the page in
  -- Go, which answered "nada encontrado" for a name sitting on page two.
  -- The caller escapes %, _ and \ in q (ILIKE's default escape is \).
  AND (
    sqlc.narg('q')::text IS NULL
    OR c.contact_name ILIKE '%' || sqlc.narg('q')::text || '%'
    OR c.contact_username ILIKE '%' || sqlc.narg('q')::text || '%'
    OR c.contact_id ILIKE '%' || sqlc.narg('q')::text || '%'
    OR (SELECT m.content FROM messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC LIMIT 1)
       ILIKE '%' || sqlc.narg('q')::text || '%'
  )
  AND (sqlc.narg('priority')::text IS NULL OR c.priority = sqlc.narg('priority'))
ORDER BY COALESCE(c.first_inbound_at, c.created_at) ASC
LIMIT $3 OFFSET $4;

-- name: ListConversationsEnrichedByOldest :many
-- "Fila por antiguidade" — raw conversation.created_at. Useful for
-- supervisors auditing how long a thread has existed regardless of
-- whether the customer kept messaging.
SELECT
    c.id,
    c.account_id,
    c.channel_id,
    c.agent_id,
    c.contact_id,
    COALESCE(c.contact_name, c.contact_id) AS contact_name,
    c.contact_username,
    c.contact_avatar_url,
    c.contact_avatar_fetched_at,
    c.contact_bio,
    c.status,
    c.is_bot_paused,
    c.assigned_to,
    c.assigned_at,
    c.priority,
    c.last_customer_message_at,
    c.waiting_since,
    c.created_at,
    c.updated_at,
    ch.provider AS channel_type,
    u.name AS assigned_user_name,
    c.sentiment,
    c.sentiment_score,
    (SELECT m.content FROM messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC LIMIT 1) AS last_message,
    COALESCE((SELECT m.type FROM messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC LIMIT 1), '')::TEXT AS last_message_type,
    -- Lead context for the inbox chip (US-M024). LATERAL rather than two
    -- correlated subqueries so the id and the stage always describe the
    -- SAME lead — independent subqueries could pick different rows when
    -- updated_at ties.
    COALESCE(ld.lead_id, '')::TEXT AS lead_id,
    COALESCE(ld.stage_name, '')::TEXT AS lead_stage_name,
    COALESCE(ld.status, '')::TEXT AS lead_status,
    -- Inbox triage (§6.10 camada 3). Read here rather than computed at
    -- render time so the ordering and the filter both run in SQL, before
    -- LIMIT/OFFSET.
    c.inbox_category,
    c.inbox_importance,
    -- Unread = customer messages nobody opened yet (messages.read_at is
    -- stamped by POST /conversations/:id/read). The tile used to fake it
    -- from status = pending_human, which is a different fact.
    (SELECT COUNT(*) FROM messages m
      WHERE m.conversation_id = c.id
        AND m.direction = 'inbound'
        AND m.read_at IS NULL)::int AS unread_count
FROM conversations c
JOIN channels ch ON ch.id = c.channel_id
LEFT JOIN users u ON u.id = c.assigned_to
LEFT JOIN LATERAL (
    SELECT l.id AS lead_id, l.status, fs.name AS stage_name
    FROM leads l
    LEFT JOIN funnel_stages fs ON fs.id = l.stage_id
    -- contact_ref_id, NOT contact_id: a lead holds the contact ULID and
    -- contact_id holds the raw channel sender, so the old join never
    -- matched and the inbox chip was blank for every lead. NULL ref
    -- (identity resolution is best-effort) simply yields no lead.
    WHERE l.account_id = c.account_id
      AND c.contact_ref_id IS NOT NULL
      AND l.contact_id = c.contact_ref_id
    ORDER BY l.updated_at DESC
    LIMIT 1
) ld ON TRUE
WHERE c.account_id = $1 AND c.status = $2
  AND (
    sqlc.narg('assigned_to')::text IS NULL
    OR (sqlc.narg('assigned_to')::text = 'none' AND c.assigned_to IS NULL)
    OR (sqlc.narg('assigned_to')::text <> 'none' AND c.assigned_to = sqlc.narg('assigned_to'))
  )
  -- Channel filter runs HERE, before LIMIT/OFFSET. Filtering the page in
  -- Go would drop every email thread that ranked past the page limit, and
  -- the inbox would answer "não há e-mails" while the account had dozens.
  AND (sqlc.narg('channel_type')::text IS NULL OR ch.provider = sqlc.narg('channel_type'))
  -- Same reason as the channel filter: filtering a page in Go shows the
  -- wrong 50 rows and answers "nada aqui" for an account that has plenty.
  AND (sqlc.narg('inbox_category')::text IS NULL OR c.inbox_category = sqlc.narg('inbox_category'))
  -- Search and priority run here as well. They used to filter the page in
  -- Go, which answered "nada encontrado" for a name sitting on page two.
  -- The caller escapes %, _ and \ in q (ILIKE's default escape is \).
  AND (
    sqlc.narg('q')::text IS NULL
    OR c.contact_name ILIKE '%' || sqlc.narg('q')::text || '%'
    OR c.contact_username ILIKE '%' || sqlc.narg('q')::text || '%'
    OR c.contact_id ILIKE '%' || sqlc.narg('q')::text || '%'
    OR (SELECT m.content FROM messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC LIMIT 1)
       ILIKE '%' || sqlc.narg('q')::text || '%'
  )
  AND (sqlc.narg('priority')::text IS NULL OR c.priority = sqlc.narg('priority'))
ORDER BY c.created_at ASC
LIMIT $3 OFFSET $4;

-- name: SetBotPaused :exec
UPDATE conversations
SET is_bot_paused = $2,
    updated_at = NOW()
WHERE id = $1 AND account_id = $3;

-- name: AutoResumeStuckConversations :execrows
-- Auto-resume conversations that have been bot-paused for too long
-- without any human reply. The asker scenario: an attendant clicked
-- "take over", got distracted, never came back; the conversation
-- sits in pending_human forever and the bot can't help. After
-- `idle_minutes` we flip is_bot_paused back to FALSE so the next
-- inbound triggers AI again. We DO NOT reopen explicitly closed
-- conversations and we skip already-resolved ones.
--
-- Excludes contacts that opted out so the resume doesn't violate
-- LGPD: those stay paused forever.
--
-- Also excludes email channels: the AI never answers email, so a
-- paused email thread is a human who deliberately assumed it, not a
-- takeover that went stale. Un-pausing it only erased that decision.
UPDATE conversations c
SET is_bot_paused = FALSE,
    updated_at = NOW()
WHERE c.is_bot_paused = TRUE
  AND c.status NOT IN ('closed', 'resolved')
  AND c.updated_at < NOW() - make_interval(mins => sqlc.arg(idle_minutes)::int)
  AND NOT EXISTS (
    SELECT 1 FROM contacts ct
    WHERE ct.id = c.contact_ref_id
      AND ct.opted_out_at IS NOT NULL
  )
  AND NOT EXISTS (
    SELECT 1 FROM channels ch
    WHERE ch.id = c.channel_id
      AND ch.provider = 'email'
  );

-- name: UpdateLastCustomerMessageAt :exec
UPDATE conversations
SET last_customer_message_at = NOW(),
    waiting_since = COALESCE(waiting_since, NOW()),
    updated_at = NOW()
WHERE id = $1 AND account_id = $2;

-- name: ClearWaitingSince :exec
UPDATE conversations
SET waiting_since = NULL,
    updated_at = NOW()
WHERE id = $1 AND account_id = $2;

-- name: UpdateConversationSentiment :exec
-- Records the analyzer's verdict on the rolling sentiment of inbound
-- messages. Called from a detached goroutine after SaveInbound so a
-- slow LLM call doesn't push the request past the 20s Meta budget.
-- COALESCE on the score keeps the previous value when the new run
-- returns 0 (analyzer didn't have enough signal).
UPDATE conversations
SET sentiment            = sqlc.arg(sentiment)::text,
    sentiment_score      = sqlc.arg(sentiment_score)::real,
    sentiment_updated_at = NOW(),
    updated_at           = NOW()
WHERE id = $1 AND account_id = $2;

-- name: ListConversationsForOwner :many
-- Owner-panel listing for a target account (no status filter; surfaces
-- the most recent threads regardless of state). Bypasses the standard
-- account-scoping that drives the dashboard reader queries — only the
-- /v1/owner/* routes are allowed to call this.
SELECT
    c.id,
    c.account_id,
    c.channel_id,
    c.agent_id,
    COALESCE(c.contact_name, c.contact_id) AS contact_name,
    c.status,
    c.is_bot_paused,
    c.last_customer_message_at,
    c.created_at,
    ch.provider AS channel_type,
    (SELECT COUNT(*) FROM messages m WHERE m.conversation_id = c.id) AS message_count
FROM conversations c
JOIN channels ch ON ch.id = c.channel_id
WHERE c.account_id = sqlc.arg(account_id)::char(26)
ORDER BY c.last_customer_message_at DESC NULLS LAST, c.created_at DESC
LIMIT sqlc.arg(lim)::int OFFSET sqlc.arg(off)::int;

-- name: ListPendingReminders :many
-- Reply-reminder worker scanner. Returns conversations where the
-- customer is waiting and the bot can't help (paused, or never had an
-- agent), the per-account opt-in is on, and the next escalation tick is
-- due. Escalation is `threshold` -> `2*threshold` -> `4*threshold`
-- (e.g. 30m/1h/2h for the default 30) so a single configured value
-- drives the full ladder. Caps at sent_count<3 so the row stops being
-- a candidate after the 3rd reminder; the worker is expected to leave
-- the conversation alone after that — the human either replied or
-- the takeover was abandoned, in which case ConversationResumeWorker
-- eventually flips is_bot_paused back to FALSE.
SELECT
    c.account_id,
    c.id                                       AS conversation_id,
    c.channel_id,
    COALESCE(c.contact_name, c.contact_id)     AS contact_name,
    c.assigned_to,
    a.reply_reminder_threshold_minutes         AS threshold_minutes,
    c.reply_reminder_sent_count                AS sent_count,
    c.waiting_since
FROM conversations c
JOIN accounts a ON a.id = c.account_id
JOIN channels ch ON ch.id = c.channel_id
WHERE a.reply_reminder_enabled = TRUE
  AND c.waiting_since IS NOT NULL
  AND c.reply_reminder_sent_count < 3
  AND c.status <> 'closed'
  AND (c.is_bot_paused = TRUE OR ch.agent_id IS NULL)
  AND CASE
        WHEN c.reply_reminder_sent_count = 0 THEN
            NOW() >= c.waiting_since + (a.reply_reminder_threshold_minutes || ' minutes')::interval
        ELSE
            NOW() >= COALESCE(c.reply_reminder_last_at, c.waiting_since)
                   + (a.reply_reminder_threshold_minutes
                      * POWER(2, c.reply_reminder_sent_count)::int
                      || ' minutes')::interval
      END
ORDER BY c.waiting_since ASC
LIMIT sqlc.arg(lim)::int;

-- name: MarkReminderSent :exec
-- Stamps reply_reminder_last_at + bumps the counter. Called by the
-- worker right after dispatch so the escalation ladder advances.
UPDATE conversations
SET reply_reminder_sent_count = reply_reminder_sent_count + 1,
    reply_reminder_last_at    = NOW(),
    updated_at                = NOW()
WHERE id = $1;

-- name: FindConversationByAccountContact :one
-- "Which thread is this contact in", scoped by account rather than by
-- channel. The lead panel needs it: a lead knows its contact but not which
-- channel the conversation happened on, and asking it to know would make the
-- panel depend on how the customer was first reached.
--
-- Joins on contact_ref_id, NOT contact_id: the caller holds a contact ULID
-- and contact_id holds the raw channel sender, so the two never matched and
-- the panel reported "no conversation" for every lead that had one.
SELECT id FROM conversations
 WHERE account_id = $1 AND contact_ref_id = $2
 ORDER BY updated_at DESC
 LIMIT 1;

-- name: CountWaitingConversations :one
-- The shell badge: customers whose last message has no human answer yet.
SELECT COUNT(*) FROM conversations
WHERE account_id = $1 AND status = 'open' AND waiting_since IS NOT NULL;

-- name: CountUnreadConversations :one
-- The other badge: threads holding a customer message nobody opened
-- (messages.read_at IS NULL). Closed threads are left out — a resolved
-- conversation's stale unread must not haunt the navigation bar.
SELECT
    COUNT(DISTINCT m.conversation_id)::bigint AS conversations,
    COUNT(*)::bigint                          AS messages
FROM messages m
JOIN conversations c ON c.id = m.conversation_id
WHERE c.account_id = $1
  AND c.status <> 'closed'
  AND m.direction = 'inbound'
  AND m.read_at IS NULL;
