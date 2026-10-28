-- name: CreateKnowledgeSource :exec
INSERT INTO knowledge_sources (
    id, agent_id, name, description, created_at
) VALUES (
    $1, $2, $3, $4, $5
);

-- name: GetKnowledgeSource :one
SELECT * FROM knowledge_sources WHERE id = $1 AND agent_id = $2;

-- name: GetKnowledgeSourceByID :one
SELECT * FROM knowledge_sources WHERE id = $1;

-- name: ListKnowledgeSources :many
SELECT * FROM knowledge_sources WHERE agent_id = $1 ORDER BY created_at DESC;

-- name: UpdateKnowledgeSource :exec
UPDATE knowledge_sources
SET name = $2, description = $3
WHERE id = $1 AND agent_id = $4;

-- name: DeleteKnowledgeSource :exec
DELETE FROM knowledge_sources WHERE id = $1 AND agent_id = $2;

-- name: CreateKnowledgeChunk :exec
INSERT INTO knowledge_chunks (
    id, source_id, content, embedding, metadata, provider, source_name, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
);

-- name: SearchKnowledge :many
-- Using cosine distance as it is standard for text embeddings
-- The <=> operator is for cosine distance in pgvector
SELECT id, source_id, content, metadata, provider, source_name, (1 - (embedding <=> $2))::float4 AS similarity
FROM knowledge_chunks
WHERE source_id = $1
ORDER BY embedding <=> $2 ASC
LIMIT $3;

-- name: ListKnowledgeChunksBySource :many
SELECT id, source_id, content, provider, source_name, metadata
FROM knowledge_chunks
WHERE source_id = $1;

-- name: UpdateKnowledgeChunkEmbedding :exec
UPDATE knowledge_chunks
SET embedding = $2, provider = $3
WHERE id = $1;

-- name: GetKnowledgeChunkByID :one
SELECT id, source_id, content, provider, source_name, metadata, created_at
FROM knowledge_chunks WHERE id = $1;

-- name: DeleteKnowledgeChunk :exec
DELETE FROM knowledge_chunks WHERE id = $1;

-- name: ListKnowledgeSourcesByAgent :many
SELECT * FROM knowledge_sources WHERE agent_id = $1 ORDER BY created_at ASC;
