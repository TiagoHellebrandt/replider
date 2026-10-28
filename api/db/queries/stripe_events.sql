-- name: InsertProcessedStripeEvent :exec
INSERT INTO processed_stripe_events (event_id, event_type)
VALUES ($1, $2)
ON CONFLICT (event_id) DO NOTHING;

-- name: IsStripeEventProcessed :one
SELECT EXISTS(SELECT 1 FROM processed_stripe_events WHERE event_id = $1) AS processed;

-- name: CleanupOldStripeEvents :exec
DELETE FROM processed_stripe_events WHERE created_at < NOW() - INTERVAL '7 days';
