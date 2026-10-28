-- name: AssignExperimentVariant :exec
INSERT INTO experiment_assignments (experiment_id, conversation_id, variant, assigned_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (experiment_id, conversation_id) DO NOTHING;

-- name: GetExperimentAssignment :one
SELECT * FROM experiment_assignments
WHERE experiment_id = $1 AND conversation_id = $2
LIMIT 1;

-- name: GetExperimentAssignmentByConversation :one
SELECT * FROM experiment_assignments
WHERE conversation_id = $1
ORDER BY assigned_at DESC
LIMIT 1;
