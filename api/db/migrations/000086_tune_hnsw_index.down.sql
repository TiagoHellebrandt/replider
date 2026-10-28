-- Revert to the implicit-default hnsw index from 000001.
DROP INDEX IF EXISTS idx_knowledge_chunks_embedding;

CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_embedding
    ON knowledge_chunks USING hnsw (embedding vector_cosine_ops);
