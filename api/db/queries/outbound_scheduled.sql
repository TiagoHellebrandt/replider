-- name: CreateOutboundScheduled :exec
INSERT INTO outbound_scheduled (
    id, account_id, channel_id, conversation_id, payload, recipient, send_at, status
) VALUES (
    sqlc.arg(id)::char(26),
    sqlc.arg(account_id)::char(26),
    sqlc.arg(channel_id)::char(26),
    sqlc.narg(conversation_id),
    sqlc.arg(payload)::jsonb,
    sqlc.arg(recipient)::text,
    sqlc.arg(send_at)::timestamptz,
    'pending'
);

-- name: ClaimDueOutboundScheduled :many
-- Drain feed for the dispatcher worker, multi-replica safe.
--
-- The CTE locks a batch of due rows with FOR UPDATE SKIP LOCKED so
-- two scheduler replicas running simultaneously claim disjoint sets
-- (no double-dispatch). The outer UPDATE flips the row to status
-- 'processing' inside the same transaction so a third replica
-- spinning up later won't pick the same rows. The dispatcher then
-- calls MarkOutboundScheduledSent / Failed / TouchAttempt to move
-- the row to its terminal state.
--
-- A separate sweeper query (RecoverStaleProcessingScheduled) flips
-- rows stuck in 'processing' back to 'pending' after a deadline so
-- a crashed replica's claims aren't lost forever.
WITH claimed AS (
    SELECT id
    FROM outbound_scheduled
    WHERE status = 'pending'
      AND send_at <= NOW()
    ORDER BY send_at ASC
    LIMIT sqlc.arg(batch_size)::int
    FOR UPDATE SKIP LOCKED
)
UPDATE outbound_scheduled o
SET status = 'processing',
    updated_at = NOW()
FROM claimed
WHERE o.id = claimed.id
RETURNING o.id, o.account_id, o.channel_id, o.conversation_id, o.payload,
          o.recipient, o.send_at, o.status, o.attempts, o.last_error,
          o.sent_message_id, o.created_at, o.updated_at;

-- name: RecoverStaleProcessingScheduled :execrows
-- Crash recovery: rows stuck in 'processing' for too long mean the
-- claiming replica died before completing the dispatch. Flip them
-- back to 'pending' so another replica picks them up. Run on a
-- separate ticker so the dispatcher itself doesn't have to manage
-- the recovery cycle.
UPDATE outbound_scheduled
SET status = 'pending',
    updated_at = NOW()
WHERE status = 'processing'
  AND updated_at < NOW() - make_interval(mins => sqlc.arg(stale_minutes)::int);

-- name: MarkOutboundScheduledSent :exec
UPDATE outbound_scheduled
SET status = 'sent',
    sent_message_id = sqlc.arg(message_id)::char(26),
    updated_at = NOW()
WHERE id = sqlc.arg(id)::char(26);

-- name: MarkOutboundScheduledFailed :exec
UPDATE outbound_scheduled
SET status = 'failed',
    attempts = attempts + 1,
    last_error = sqlc.arg(reason)::text,
    updated_at = NOW()
WHERE id = sqlc.arg(id)::char(26);

-- name: TouchOutboundScheduledAttempt :exec
-- Bumps attempts + last_error and flips status back to 'pending' so
-- the next ClaimDue tick re-claims the row. Used for transient
-- failures (Meta 5xx, network blip) where the worker wants to retry
-- without consuming the failed-final terminal state.
UPDATE outbound_scheduled
SET status = 'pending',
    attempts = attempts + 1,
    last_error = sqlc.arg(reason)::text,
    updated_at = NOW()
WHERE id = sqlc.arg(id)::char(26);

-- name: CancelOutboundScheduled :exec
-- Manual ops abort. Idempotent — already-sent rows aren't touched.
UPDATE outbound_scheduled
SET status = 'cancelled',
    updated_at = NOW()
WHERE id = sqlc.arg(id)::char(26)
  AND account_id = sqlc.arg(account_id)::char(26)
  AND status = 'pending';

-- name: ListAccountScheduledOutbound :many
SELECT id, account_id, channel_id, conversation_id, recipient,
       send_at, status, attempts, last_error, created_at
FROM outbound_scheduled
WHERE account_id = sqlc.arg(account_id)::char(26)
ORDER BY send_at ASC
LIMIT sqlc.arg(limit_n)::int OFFSET sqlc.arg(offset_n)::int;
