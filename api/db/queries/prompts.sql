-- name: CreatePrompt :exec
INSERT INTO prompts (
    id, account_id, name, description, body, source, wizard_answers, created_by,
    created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW()
);

-- name: GetPromptByID :one
SELECT * FROM prompts WHERE id = $1 LIMIT 1;

-- name: ListPromptsByAccount :many
SELECT * FROM prompts
WHERE account_id = $1
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPromptsByAccount :one
SELECT COUNT(*) FROM prompts WHERE account_id = $1;

-- name: UpdatePrompt :exec
UPDATE prompts
SET name = $2,
    description = $3,
    body = $4,
    wizard_answers = $5,
    updated_at = NOW()
WHERE id = $1;

-- name: DeletePrompt :exec
DELETE FROM prompts WHERE id = $1;
