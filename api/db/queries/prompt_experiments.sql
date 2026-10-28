-- name: CreatePromptExperiment :exec
INSERT INTO prompt_experiments (
    id, account_id, agent_id, name, status,
    variant_a_body, variant_b_body, variant_a_label, variant_b_label,
    split_pct_b, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW()
);

-- name: GetPromptExperimentByID :one
SELECT * FROM prompt_experiments WHERE id = $1 LIMIT 1;

-- name: GetRunningExperimentByAgent :one
SELECT * FROM prompt_experiments
WHERE agent_id = $1 AND status = 'running'
LIMIT 1;

-- name: ListExperimentsByAccount :many
SELECT * FROM prompt_experiments
WHERE account_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListExperimentsByAgent :many
SELECT * FROM prompt_experiments
WHERE agent_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateExperimentStatus :exec
UPDATE prompt_experiments
SET status = $2,
    started_at = COALESCE($3, started_at),
    stopped_at = COALESCE($4, stopped_at),
    updated_at = NOW()
WHERE id = $1;

-- name: SetExperimentWinner :exec
UPDATE prompt_experiments
SET winner = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: DeletePromptExperiment :exec
DELETE FROM prompt_experiments WHERE id = $1;
