-- name: CreateTag :exec
INSERT INTO tags (id, account_id, name, color, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: ListTagsByAccount :many
SELECT * FROM tags WHERE account_id = $1 ORDER BY name;

-- name: GetTagByID :one
SELECT * FROM tags WHERE id = $1 LIMIT 1;

-- name: GetTagByAccountAndName :one
-- Used by the auto-tag pipeline to dedupe before creating: the LLM
-- classifier always returns one of a fixed set of intent labels, and
-- we want them to map to a single tag per account regardless of how
-- many conversations land on it.
SELECT * FROM tags
WHERE account_id = $1 AND LOWER(name) = LOWER($2)
LIMIT 1;

-- name: UpdateTag :exec
UPDATE tags SET name = $2, color = $3 WHERE id = $1;

-- name: DeleteTag :exec
DELETE FROM tags WHERE id = $1;

-- name: AddTagToConversation :exec
INSERT INTO conversation_tags (conversation_id, tag_id, created_at)
VALUES ($1, $2, NOW())
ON CONFLICT DO NOTHING;

-- name: RemoveTagFromConversation :exec
DELETE FROM conversation_tags WHERE conversation_id = $1 AND tag_id = $2;

-- name: ListTagsByConversation :many
SELECT t.* FROM tags t
JOIN conversation_tags ct ON ct.tag_id = t.id
WHERE ct.conversation_id = $1
ORDER BY t.name;

-- name: ListTagsByConversationIDs :many
-- Batch variant used by the dashboard list so we can enrich N
-- conversations with their tags in a single round-trip instead of N
-- queries (one per conversation).
SELECT ct.conversation_id, t.id, t.account_id, t.name, t.color, t.created_at
FROM tags t
JOIN conversation_tags ct ON ct.tag_id = t.id
WHERE ct.conversation_id = ANY($1::text[])
ORDER BY t.name;

-- name: ListConversationsByTag :many
SELECT ct.conversation_id FROM conversation_tags ct
WHERE ct.tag_id = $1;
