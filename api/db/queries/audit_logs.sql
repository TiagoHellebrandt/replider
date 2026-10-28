-- name: CreateAuditLog :exec
INSERT INTO audit_logs (
    id, account_id, actor_id, action, entity_type, entity_id, payload, ip, user_agent, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
);

-- name: ListAuditLogsByAccount :many
-- Admin-only listing for LGPD accountability / GDPR records-of-processing.
-- Cursor-free pagination (offset/limit); the index on (account_id,
-- created_at DESC) keeps this O(log N) even with millions of rows.
SELECT id, account_id, actor_id, action, entity_type, entity_id, payload, ip, user_agent, created_at
FROM audit_logs
WHERE account_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountAuditLogsByAccount :one
SELECT COUNT(*) FROM audit_logs WHERE account_id = $1;

-- name: ListAuditLogsFiltered :many
-- Filtered listing used by the Settings → Conformidade filter bar. Each
-- filter is a nullable param; pass NULL to match everything. Same
-- (account_id, created_at DESC) index serves this query because the
-- other predicates are selective refinements.
SELECT id, account_id, actor_id, action, entity_type, entity_id, payload, ip, user_agent, created_at
FROM audit_logs
WHERE account_id = $1
  AND (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action'))
  AND (sqlc.narg('entity_type')::text IS NULL OR entity_type = sqlc.narg('entity_type'))
  AND (sqlc.narg('actor_id')::text IS NULL OR actor_id = sqlc.narg('actor_id'))
  AND (sqlc.narg('from_ts')::timestamptz IS NULL OR created_at >= sqlc.narg('from_ts'))
  AND (sqlc.narg('to_ts')::timestamptz IS NULL OR created_at <= sqlc.narg('to_ts'))
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count') OFFSET sqlc.arg('offset_count');

-- name: CountAuditLogsFiltered :one
SELECT COUNT(*) FROM audit_logs
WHERE account_id = $1
  AND (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action'))
  AND (sqlc.narg('entity_type')::text IS NULL OR entity_type = sqlc.narg('entity_type'))
  AND (sqlc.narg('actor_id')::text IS NULL OR actor_id = sqlc.narg('actor_id'))
  AND (sqlc.narg('from_ts')::timestamptz IS NULL OR created_at >= sqlc.narg('from_ts'))
  AND (sqlc.narg('to_ts')::timestamptz IS NULL OR created_at <= sqlc.narg('to_ts'));
