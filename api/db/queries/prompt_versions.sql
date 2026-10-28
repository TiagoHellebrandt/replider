-- name: CreatePromptVersion :exec
INSERT INTO prompt_versions (
    id, agent_id, prompt_id, body, applied_by, applied_at, note
) VALUES (
    $1, $2, $3, $4, $5, NOW(), $6
);

-- name: GetPromptVersionByID :one
SELECT * FROM prompt_versions WHERE id = $1 LIMIT 1;

-- name: ListPromptVersionsByAgent :many
SELECT * FROM prompt_versions
WHERE agent_id = $1
ORDER BY applied_at DESC
LIMIT $2 OFFSET $3;
