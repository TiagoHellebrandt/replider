-- name: CreatePendingCSAT :exec
-- Called when a conversation closes. The survey message is shipped
-- separately by the channel adapter; this row just reserves the slot
-- so customer responses can be matched later.
INSERT INTO csat_ratings (id, account_id, conversation_id, contact_id, rating, sent_at, created_at)
VALUES ($1, $2, $3, $4, 0, NOW(), NOW())
ON CONFLICT (conversation_id) DO NOTHING;

-- name: RecordCSATAnswer :exec
-- Updates the row with the customer's rating + free-text feedback. We
-- match on conversation_id because the customer's reply arrives on the
-- same conversation. rating must be 1..5 (enforced by the CHECK).
UPDATE csat_ratings
SET rating = $2, feedback = $3, answered_at = NOW()
WHERE conversation_id = $1;

-- name: GetCSATStats :one
-- Aggregates for the admin dashboard: average rating (1-5), how many
-- surveys were sent, how many got an answer. Filtered to the last 30
-- days to stay snappy even as the table grows.
SELECT
  COALESCE(AVG(rating) FILTER (WHERE rating > 0), 0)::float4 AS avg_rating,
  COUNT(*) FILTER (WHERE sent_at IS NOT NULL)                    AS sent,
  COUNT(*) FILTER (WHERE answered_at IS NOT NULL AND rating > 0) AS answered
FROM csat_ratings
WHERE account_id = $1
  AND created_at >= NOW() - INTERVAL '30 days';
