-- name: CreateCannedResponse :exec
INSERT INTO canned_responses (id, account_id, shortcut, title, content, category, visibility, owner_user_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- Visible set = account-wide rows + the caller's own personal rows (US-018).
-- name: ListVisibleCannedResponses :many
SELECT * FROM canned_responses
WHERE account_id = $1 AND (visibility = 'account' OR owner_user_id = $2)
ORDER BY category, title;

-- name: GetCannedResponseByID :one
SELECT * FROM canned_responses WHERE id = $1 LIMIT 1;

-- name: GetVisibleCannedResponseByShortcut :one
SELECT * FROM canned_responses
WHERE account_id = $1 AND shortcut = $3 AND (visibility = 'account' OR owner_user_id = $2)
LIMIT 1;

-- name: UpdateCannedResponse :exec
UPDATE canned_responses
SET shortcut = $2, title = $3, content = $4, category = $5, updated_at = NOW()
WHERE id = $1;

-- name: DeleteCannedResponse :exec
DELETE FROM canned_responses WHERE id = $1;
