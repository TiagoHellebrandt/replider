-- name: InsertWAWebhookEvent :exec
-- Persists the raw signed payload before any parsing. Returning a
-- pre-generated ULID lets the caller correlate the row in logs without
-- a follow-up SELECT.
INSERT INTO wa_webhook_events (id, payload, signature)
VALUES ($1, $2, $3);

-- name: MarkWAWebhookEventProcessed :exec
-- Final state when every side effect (DB persist, AI fanout, WS
-- broadcast) committed. processed_at is the wall clock at success so
-- the janitor can prune by age.
UPDATE wa_webhook_events
SET status = 'processed', processed_at = NOW(), error = NULL
WHERE id = $1;

-- name: MarkWAWebhookEventSkipped :exec
-- Recognised but intentionally not actionable (Meta test webhook,
-- system events the AI shouldn't run on, deduped retries). We close
-- them out so the worker doesn't keep re-attempting.
UPDATE wa_webhook_events
SET status = 'skipped', processed_at = NOW(), error = NULL
WHERE id = $1;

-- name: MarkWAWebhookEventFailed :exec
-- Records a transient failure. attempts increments so the worker can
-- promote rows past max_attempts to status='dead' for human review.
-- Flips status back to 'pending' so the next worker tick re-claims
-- the row — without this the row would stay stuck in 'processing'
-- after the multi-replica claim refactor (rev 000049 ListPending).
UPDATE wa_webhook_events
SET status = 'pending',
    error = $2,
    attempts = attempts + 1,
    last_attempt_at = NOW()
WHERE id = $1;

-- name: MarkWAWebhookEventDead :exec
-- Promotion when attempts >= max_attempts. Stops the worker from
-- re-trying forever; ops needs to triage.
UPDATE wa_webhook_events
SET status = 'dead', last_attempt_at = NOW()
WHERE id = $1;

-- name: ListPendingWAWebhookEvents :many
-- Drain feed for the outbox worker, multi-replica safe.
--
-- Locks a batch of pending rows with FOR UPDATE SKIP LOCKED so two
-- worker replicas running simultaneously claim disjoint sets. We
-- flip to status='processing' AND bump last_attempt_at so the row
-- is also invisible to the 30s cooldown check on the next select.
-- Terminal status (Processed/Skipped/Failed) is written by the
-- worker after the replay; a stale 'processing' row gets recovered
-- back to 'pending' by the sweeper after `stale_minutes`.
WITH claimed AS (
    SELECT id
    FROM wa_webhook_events
    WHERE status = 'pending'
      AND (last_attempt_at IS NULL OR last_attempt_at < NOW() - INTERVAL '30 seconds')
    ORDER BY received_at ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE wa_webhook_events e
SET status = 'processing',
    last_attempt_at = NOW()
FROM claimed
WHERE e.id = claimed.id
RETURNING e.id, e.payload, e.signature, e.attempts;

-- name: RecoverStaleProcessingWAWebhookEvents :execrows
-- Crash recovery for rows the worker claimed but never marked
-- terminal — usually because the replica that owned them died.
-- Flips status back to pending so the next worker tick picks them.
UPDATE wa_webhook_events
SET status = 'pending'
WHERE status = 'processing'
  AND last_attempt_at < NOW() - make_interval(mins => sqlc.arg(stale_minutes)::int);

-- name: ListWAWebhookEventsByStatus :many
-- Ops triage feed. Lets the admin dashboard / runbook commands pull
-- recent rows of any status (pending, processed, skipped, failed,
-- dead) for inspection. Status filter uses simple equality so the
-- `'all'` case is handled in the repo (separate query).
SELECT id, status, attempts, received_at, processed_at, error, last_attempt_at
FROM wa_webhook_events
WHERE status = sqlc.arg(status)::text
ORDER BY received_at DESC
LIMIT $1;

-- name: ListAllWAWebhookEvents :many
-- Sibling of ListWAWebhookEventsByStatus for the `'all'` case.
SELECT id, status, attempts, received_at, processed_at, error, last_attempt_at
FROM wa_webhook_events
ORDER BY received_at DESC
LIMIT $1;

-- name: ResetWAWebhookEventForRetry :exec
-- Manual ops escape hatch: re-enqueue a row that was promoted to dead
-- (or a stuck pending row). Sets status back to pending and clears
-- last_attempt_at so ListPending picks it on the next worker tick.
-- attempts is reset so the dead-letter cap doesn't fire again on the
-- second mistake.
UPDATE wa_webhook_events
SET status = 'pending',
    last_attempt_at = NULL,
    attempts = 0,
    error = NULL
WHERE id = $1;

-- name: PruneProcessedWAWebhookEvents :execrows
-- Janitor: delete processed rows older than `older_than`. Kept as a
-- query (not a hard-coded interval) so the worker can sweep
-- aggressively in tests and conservatively in prod.
DELETE FROM wa_webhook_events
WHERE status IN ('processed', 'skipped')
  AND processed_at IS NOT NULL
  AND processed_at < sqlc.arg(older_than)::timestamptz;
