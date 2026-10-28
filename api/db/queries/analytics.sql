-- name: GetConversationStats :one
SELECT
  COUNT(*) as total_conversations,
  COUNT(CASE WHEN status = 'open' THEN 1 END) as open_conversations,
  COUNT(CASE WHEN status = 'pending_human' THEN 1 END) as pending_human,
  COUNT(CASE WHEN status = 'closed' THEN 1 END) as closed_conversations,
  COUNT(CASE WHEN created_at >= NOW() - INTERVAL '24 hours' THEN 1 END) as last_24h,
  COUNT(CASE WHEN created_at >= NOW() - INTERVAL '7 days' THEN 1 END) as last_7d
FROM conversations
WHERE account_id = $1;

-- name: GetMessageStats :one
-- Who sent what. Split by DIRECTION, never by a literal sender value: an
-- inbound row stores the contact's own phone/external id in `sender`, and the
-- string 'customer' is never written in production (only old seeds do). The
-- previous version counted `sender = 'customer'` as the contact and everything
-- that wasn't 'bot'/'customer' as a human attendant, so on a live account every
-- customer message was filed under "Humano" and "Contato" read 0 — on a
-- dashboard whose whole content is customer conversations. See the
-- `messages.sender` note in api/CLAUDE.md.
SELECT
  COUNT(*) as total_messages,
  COUNT(CASE WHEN m.direction = 'outbound' AND m.sender = 'bot' THEN 1 END) as bot_messages,
  COUNT(CASE WHEN m.direction = 'outbound' AND m.sender <> 'bot' THEN 1 END) as human_messages,
  COUNT(CASE WHEN m.direction = 'inbound' THEN 1 END) as contact_messages,
  COUNT(CASE WHEN m.created_at >= NOW() - INTERVAL '24 hours' THEN 1 END) as last_24h
FROM messages m
JOIN conversations c ON c.id = m.conversation_id
WHERE c.account_id = $1;

-- name: GetDailyConversationCounts :many
SELECT
  DATE(created_at) as date,
  COUNT(*) as count
FROM conversations
WHERE account_id = $1
  AND created_at >= NOW() - INTERVAL '30 days'
GROUP BY DATE(created_at)
ORDER BY date;

-- name: GetSLAMetrics :one
SELECT
  -- Average first response time in seconds (only for conversations that got a response)
  COALESCE(
    EXTRACT(EPOCH FROM AVG(first_response_at - created_at))::int,
    0
  ) as avg_first_response_seconds,
  -- Average resolution time in seconds
  COALESCE(
    EXTRACT(EPOCH FROM AVG(resolved_at - created_at))::int,
    0
  ) as avg_resolution_seconds,
  -- Percentage of conversations responded to within 5 minutes
  COALESCE(
    (COUNT(CASE WHEN first_response_at - created_at <= INTERVAL '5 minutes' THEN 1 END) * 100)
    / NULLIF(COUNT(first_response_at), 0),
    0
  )::int as pct_responded_within_5min,
  -- Total resolved conversations
  COUNT(CASE WHEN resolved_at IS NOT NULL THEN 1 END)::int as total_resolved,
  -- Total conversations with a first response
  COUNT(CASE WHEN first_response_at IS NOT NULL THEN 1 END)::int as total_responded
FROM conversations
WHERE account_id = $1
  AND created_at >= NOW() - INTERVAL '30 days';

-- name: GetAgentPerformance :many
SELECT
  m.sender as agent_id,
  COUNT(DISTINCT m.conversation_id) as conversations_handled,
  COUNT(*) as messages_sent
FROM messages m
JOIN conversations c ON c.id = m.conversation_id
WHERE c.account_id = $1
  AND m.direction = 'outbound'
  AND m.sender != 'bot'
  AND m.sender != 'customer'
  AND m.created_at >= NOW() - INTERVAL '30 days'
GROUP BY m.sender
ORDER BY conversations_handled DESC;

-- name: GetAttendantPerformance :many
-- Per-human-attendant metrics for the Settings → Time screen. Each row
-- is one user in the account; aggregates are computed over a caller-
-- supplied [from_ts, to_ts] window so the UI can switch between
-- Hoje / 7d / 30d / Mês corrente without extra endpoints.
--
-- Shape decisions:
--   * LEFT JOINs off the users table so an attendant with zero activity
--     in the window still shows up (with zeros) — important for "quem
--     do time ficou ocioso nesta semana?" visibility.
--   * Three CTEs (conv_agg, msg_agg, csat_agg) keep each slice O(n)
--     instead of an N×M*K join blowup.
--   * user_id is narg so the same query powers the team list view
--     (NULL = all attendants) and the per-member drill-down screen.
--
-- Metric semantics:
--   * conversations_assigned: conversations where this user is the
--     current assignee AND the conversation was created in the window.
--   * avg_first_response_seconds: derived from first_response_at, which
--     is stamped on first outbound message regardless of who sent it.
--     Good-enough proxy for attendant TTFR until we wire a takeover
--     event (commit 3 in the plan).
--   * csat_avg: averages over csat_ratings answered in the window,
--     attributed via conversations.assigned_to.
WITH conv_agg AS (
  SELECT
    c.assigned_to AS attendant_id,
    COUNT(*) AS conversations_assigned,
    COUNT(*) FILTER (WHERE c.resolved_at IS NOT NULL) AS conversations_resolved,
    COALESCE(EXTRACT(EPOCH FROM AVG(c.first_response_at - c.created_at)
      FILTER (WHERE c.first_response_at IS NOT NULL))::int, 0) AS avg_first_response_seconds,
    COALESCE(EXTRACT(EPOCH FROM AVG(c.resolved_at - c.created_at)
      FILTER (WHERE c.resolved_at IS NOT NULL))::int, 0) AS avg_resolution_seconds
  FROM conversations c
  WHERE c.account_id = $1
    AND c.assigned_to IS NOT NULL
    AND c.created_at >= $2
    AND c.created_at <= $3
    AND (sqlc.narg('user_id')::text IS NULL OR c.assigned_to = sqlc.narg('user_id'))
  GROUP BY c.assigned_to
),
msg_agg AS (
  SELECT
    m.sender AS attendant_id,
    COUNT(*) AS messages_sent
  FROM messages m
  JOIN conversations c ON c.id = m.conversation_id
  WHERE c.account_id = $1
    AND m.direction = 'outbound'
    AND m.sender NOT IN ('bot', 'customer')
    AND m.created_at >= $2
    AND m.created_at <= $3
    AND (sqlc.narg('user_id')::text IS NULL OR m.sender = sqlc.narg('user_id'))
  GROUP BY m.sender
),
csat_agg AS (
  SELECT
    c.assigned_to AS attendant_id,
    COALESCE(AVG(r.rating) FILTER (WHERE r.rating > 0), 0)::float4 AS csat_avg,
    COUNT(*) FILTER (WHERE r.answered_at IS NOT NULL AND r.rating > 0) AS csat_count
  FROM csat_ratings r
  JOIN conversations c ON c.id = r.conversation_id
  WHERE c.account_id = $1
    AND c.assigned_to IS NOT NULL
    AND r.answered_at IS NOT NULL
    AND r.answered_at >= $2
    AND r.answered_at <= $3
    AND (sqlc.narg('user_id')::text IS NULL OR c.assigned_to = sqlc.narg('user_id'))
  GROUP BY c.assigned_to
)
SELECT
  u.id AS user_id,
  u.name,
  u.email,
  u.role,
  u.last_seen_at,
  COALESCE(ca.conversations_assigned, 0)::bigint AS conversations_assigned,
  COALESCE(ca.conversations_resolved, 0)::bigint AS conversations_resolved,
  COALESCE(ca.avg_first_response_seconds, 0)::int AS avg_first_response_seconds,
  COALESCE(ca.avg_resolution_seconds, 0)::int AS avg_resolution_seconds,
  COALESCE(ma.messages_sent, 0)::bigint AS messages_sent,
  COALESCE(cs.csat_avg, 0)::float4 AS csat_avg,
  COALESCE(cs.csat_count, 0)::bigint AS csat_count
FROM users u
LEFT JOIN conv_agg ca ON ca.attendant_id = u.id
LEFT JOIN msg_agg ma ON ma.attendant_id = u.id
LEFT JOIN csat_agg cs ON cs.attendant_id = u.id
WHERE u.account_id = $1
  AND (sqlc.narg('user_id')::text IS NULL OR u.id = sqlc.narg('user_id'))
ORDER BY conversations_assigned DESC, u.name ASC;

-- name: GetAttendantLiveSnapshot :many
-- Right-now supervisor view: per attendant, count of open conversations
-- they currently own + the timestamp of the oldest unanswered customer
-- message across that set. Admin can spot "who is swamped" and "which
-- conversation is rotting" without opening every row.
--
-- Shape decisions:
--   * LEFT JOIN users so attendants with zero open convs still show up
--     (they're "available"). Ordering by open_convs desc puts the
--     loaded agents at the top of the panel.
--   * Only counts open + pending_human conversations. Closed
--     conversations obviously don't need supervision, and a 'resolved'
--     status (if ever added) should also filter out by design.
--   * oldest_waiting_since comes from the conversations.waiting_since
--     column, which is set when the customer last spoke without a reply
--     and cleared by ClearWaitingSince when someone answers. Perfect
--     SLA-risk signal.
SELECT
  u.id                                    AS user_id,
  u.name,
  u.role,
  u.last_seen_at,
  COALESCE(agg.open_convs, 0)::bigint     AS open_convs,
  agg.oldest_waiting_since::timestamptz   AS oldest_waiting_since
FROM users u
LEFT JOIN (
  SELECT
    c.assigned_to,
    COUNT(*)                              AS open_convs,
    MIN(c.waiting_since)                  AS oldest_waiting_since
  FROM conversations c
  WHERE c.account_id = $1
    AND c.assigned_to IS NOT NULL
    AND c.status IN ('open', 'pending_human')
  GROUP BY c.assigned_to
) agg ON agg.assigned_to = u.id
WHERE u.account_id = $1
  AND u.is_active = true
ORDER BY open_convs DESC, u.name ASC;

-- name: GetUnassignedQueueSnapshot :one
-- Queue summary for the supervisor panel: total open/pending conversations
-- that currently have no assigned attendant, plus the oldest
-- waiting_since among them. A non-zero count with an old timestamp is
-- the canonical "nobody picked this up" SLA alert.
SELECT
  COUNT(*)::bigint                          AS unassigned_count,
  MIN(waiting_since)::timestamptz           AS oldest_waiting_since
FROM conversations
WHERE account_id = $1
  AND assigned_to IS NULL
  AND status IN ('open', 'pending_human');

-- name: GetAIPerformance :one
-- Autonomous-resolution ratio over the last 30 days. A conversation is
-- "autonomous" when the bot handled it without a human ever unpausing
-- it — proxy we use: at least one bot message AND zero human-outbound
-- messages on that conversation.
SELECT
  COUNT(DISTINCT c.id) AS total_conversations,
  COUNT(DISTINCT CASE
    WHEN EXISTS (SELECT 1 FROM messages mb WHERE mb.conversation_id = c.id AND mb.sender = 'bot')
     AND NOT EXISTS (SELECT 1 FROM messages mh WHERE mh.conversation_id = c.id AND mh.direction = 'outbound' AND mh.sender != 'bot')
    THEN c.id END) AS autonomous_conversations,
  (SELECT COUNT(*) FROM messages m
    JOIN conversations cc ON cc.id = m.conversation_id
    WHERE cc.account_id = $1 AND m.sender = 'bot' AND m.created_at >= NOW() - INTERVAL '30 days') AS bot_messages,
  (SELECT COUNT(*) FROM messages m
    JOIN conversations cc ON cc.id = m.conversation_id
    WHERE cc.account_id = $1 AND m.direction = 'outbound' AND m.sender != 'bot' AND m.created_at >= NOW() - INTERVAL '30 days') AS human_messages
FROM conversations c
WHERE c.account_id = $1
  AND c.created_at >= NOW() - INTERVAL '30 days';

-- name: GetOverviewStats :one
-- Mission 6: operational analytics MVP. Single-row aggregate over the
-- caller's window. total_conversations is the denominator everything
-- else is ratio'd against; avg_first_response_seconds is null-safe via
-- NULLIF so an empty period returns 0 instead of division errors.
SELECT
  COUNT(*)::int AS total_conversations,
  COALESCE(
    (COUNT(CASE
      WHEN first_response_by = 'bot' AND is_bot_paused = FALSE THEN 1
    END)::float * 100.0)
      / NULLIF(COUNT(*)::float, 0),
    0
  )::float AS resolved_by_ai_pct,
  COALESCE(
    EXTRACT(EPOCH FROM AVG(first_response_at - first_inbound_at))::int,
    0
  )::int AS avg_first_response_seconds
FROM conversations
WHERE account_id = $1
  AND created_at >= NOW() - make_interval(days => $2::int);

-- name: GetChannelBreakdown :many
-- Provider counts scoped to the same window. We group on channels.provider
-- rather than the conversation's channel_id so "whatsapp" spanning multiple
-- numbers rolls up to one bar.
SELECT
  ch.provider AS channel,
  COUNT(c.id)::int AS count
FROM conversations c
JOIN channels ch ON ch.id = c.channel_id
WHERE c.account_id = $1
  AND c.created_at >= NOW() - make_interval(days => $2::int)
GROUP BY ch.provider
ORDER BY count DESC;

-- name: GetTopQuestions :many
-- MVP clustering: lowercase + trim of the first inbound message per
-- conversation. Noisy by design (no NLP / stemming) — the copy on the
-- dashboard frames it as "perguntas mais comuns" so users don't
-- over-interpret. Filters out empty strings and one-char inputs.
SELECT
  LOWER(TRIM(m.content)) AS text,
  COUNT(*)::int AS count
FROM messages m
JOIN conversations c ON c.id = m.conversation_id
WHERE c.account_id = $1
  AND m.direction = 'inbound'
  AND m.created_at >= NOW() - make_interval(days => $2::int)
  AND m.content IS NOT NULL
  AND LENGTH(TRIM(m.content)) > 1
  -- One row per conversation: the earliest inbound. MIN(id) is a
  -- cheap proxy for "oldest" when IDs are ULIDs (time-ordered).
  AND m.id = (
    SELECT MIN(m2.id)
    FROM messages m2
    WHERE m2.conversation_id = c.id
      AND m2.direction = 'inbound'
  )
GROUP BY LOWER(TRIM(m.content))
ORDER BY count DESC, text ASC
LIMIT 10;

-- name: GetCSATDistribution :many
-- Per-score bucket count (1..5). Empty buckets are returned as
-- zero so the handler can emit a stable {"1":..,"5":..} object
-- without post-processing.
SELECT
  rating,
  COUNT(*)::int AS count
FROM csat_ratings
WHERE account_id = $1
  AND rating BETWEEN 1 AND 5
  AND answered_at IS NOT NULL
  AND answered_at >= NOW() - make_interval(days => $2::int)
GROUP BY rating
ORDER BY rating;

-- name: GetTopExpensiveConversations :many
-- US-030: top-N conversations by accumulated estimated AI provider cost.
-- Powers the cost-attribution dashboard so ops can spot the customers
-- driving spend. Scoped by account_id; cost rolls up from per-message
-- token accounting into conversations.total_cost_usd.
SELECT
  id,
  contact_id,
  contact_name,
  total_cost_usd,
  created_at
FROM conversations
WHERE account_id = $1
  AND total_cost_usd > 0
ORDER BY total_cost_usd DESC, id DESC
LIMIT $2;

-- name: ListIdleConversationsForCSAT :many
-- Mission 6 CSAT asker: conversations whose latest bot message is
-- older than the idle threshold, have never been asked for CSAT, and
-- aren't already closed. We look at MAX(created_at) on outbound bot
-- messages to avoid triggering on human-only conversations.
SELECT
  c.id, c.account_id, c.channel_id, c.contact_id, c.status, c.is_bot_paused,
  c.last_customer_message_at, c.waiting_since, c.created_at, c.updated_at,
  c.agent_id, c.contact_name, c.contact_username,
  c.assigned_to, c.priority, c.first_response_at, c.resolved_at,
  c.contact_ref_id, c.assigned_at,
  c.first_inbound_at, c.first_response_by,
  c.language
FROM conversations c
JOIN accounts a ON a.id = c.account_id
WHERE c.status != 'closed'
  AND c.is_bot_paused = FALSE
  AND a.csat_enabled = TRUE
  -- Skip contacts that explicitly opted out — sending a CSAT prompt
  -- to them is exactly the unsolicited outbound LGPD/CAN-SPAM
  -- forbids. Inbound flagging via STOP keywords stamps
  -- contacts.opted_out_at; this filter joins via contact_ref_id so
  -- legacy rows missing the link still slide through.
  AND NOT EXISTS (
    SELECT 1 FROM contacts ct
    WHERE ct.id = c.contact_ref_id
      AND ct.opted_out_at IS NOT NULL
  )
  AND NOT EXISTS (
    SELECT 1 FROM csat_ratings r
    WHERE r.conversation_id = c.id
  )
  AND EXISTS (
    SELECT 1 FROM messages m
    WHERE m.conversation_id = c.id
      AND m.sender = 'bot'
      AND m.created_at < NOW() - make_interval(mins => $1::int)
  )
  AND NOT EXISTS (
    -- Skip if the customer said anything after the last bot message —
    -- conversation still active, too early to ask.
    SELECT 1 FROM messages m
    WHERE m.conversation_id = c.id
      AND m.direction = 'inbound'
      AND m.created_at > (
        SELECT MAX(mb.created_at) FROM messages mb
        WHERE mb.conversation_id = c.id AND mb.sender = 'bot'
      )
  )
ORDER BY c.updated_at ASC
LIMIT $2;

-- name: GetPendingCSATForConversation :one
-- Used by the inbound pipeline to detect "is this number a CSAT
-- reply?". Returns the pending row so we can record the answer
-- instead of letting the bot respond to a bare "5".
SELECT id, account_id, conversation_id, contact_id, rating, sent_at
FROM csat_ratings
WHERE conversation_id = $1
  AND rating = 0
  AND sent_at IS NOT NULL
  AND answered_at IS NULL
LIMIT 1;
