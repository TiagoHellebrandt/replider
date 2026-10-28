-- name: CreateAgent :exec
INSERT INTO agents (
    id, account_id, name, system_prompt, provider, model, timezone, is_active, created_at, updated_at,
    hide_ai_identity, silent_on_unknown
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
);

-- name: GetAgentByID :one
SELECT * FROM agents
WHERE id = $1 LIMIT 1;

-- name: UpdateAgent :exec
UPDATE agents
SET name = $2,
    system_prompt = $3,
    provider = $4,
    model = $5,
    timezone = $6,
    is_active = $7,
    updated_at = $8,
    personality_prompt = $9,
    learn_from_humans = $10,
    business_hours_start = $11,
    business_hours_end   = $12,
    business_hours_days  = $13,
    hide_ai_identity     = $14,
    silent_on_unknown    = $15
WHERE id = $1;

-- name: ListAgentsByAccountID :many
SELECT * FROM agents
WHERE account_id = $1;

-- name: DeleteAgent :exec
DELETE FROM agents WHERE id = $1;

-- name: ListAllAgentTimezones :many
SELECT id, account_id, name, timezone FROM agents
ORDER BY account_id, id;
