package domain

import (
	"context"
	"time"
)

type KnowledgeSource struct {
	ID          string    `json:"id"`
	AgentID     string    `json:"agent_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type KnowledgeChunk struct {
	ID         string    `json:"id"`
	SourceID   string    `json:"source_id"`
	Content    string    `json:"content"`
	Embedding  []float32 `json:"-"`
	Provider   *string   `json:"provider"`
	SourceName *string   `json:"source_name"`
	Metadata   any       `json:"metadata" swaggertype:"object"`
	// Similarity is the cosine similarity of this chunk against the query
	// embedding, in [0, 1]. Only populated by Search(); zero for listings.
	// The AgentRunner drops chunks under a threshold so low-relevance
	// results don't confidently hallucinate their way into the prompt.
	Similarity float32   `json:"similarity,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// RAGSimilarityFloor is the minimum cosine similarity required for a
// chunk to be considered relevant enough to include in the agent's
// system prompt. Empirically below ~0.55 the chunks are semantic noise
// and cause more harm than good (wrong answers with high confidence).
const RAGSimilarityFloor float32 = 0.55

type KnowledgeRepository interface {
	// Source methods
	CreateSource(ctx context.Context, source *KnowledgeSource) error
	GetSource(ctx context.Context, id string, agentID string) (*KnowledgeSource, error)
	GetSourceByID(ctx context.Context, id string) (*KnowledgeSource, error)
	ListSources(ctx context.Context, agentID string) ([]*KnowledgeSource, error)
	UpdateSource(ctx context.Context, source *KnowledgeSource) error
	DeleteSource(ctx context.Context, id string, agentID string) error

	// Chunk methods
	CreateChunk(ctx context.Context, chunk *KnowledgeChunk) error
	GetChunkByID(ctx context.Context, id string) (*KnowledgeChunk, error)
	Search(ctx context.Context, sourceID string, embedding []float32, limit int32) ([]*KnowledgeChunk, error)
	ListChunksBySource(ctx context.Context, sourceID string) ([]*KnowledgeChunk, error)
	UpdateEmbedding(ctx context.Context, chunkID string, embedding []float32, provider string) error
	DeleteChunk(ctx context.Context, chunkID string) error
}

// EmbeddingCacheRepository caches query embeddings keyed by the SHA-256
// of the input text and the embedding model/provider (US-022). The hot
// RAG path embeds the customer's message on every inbound turn; repeated
// inputs (greetings, common questions) would otherwise hit the provider
// each time. Get returns (nil, nil) on a miss — callers treat a missing
// row as "embed and Put", never as an error.
type EmbeddingCacheRepository interface {
	// Get returns the cached embedding for (textHash, model), or
	// (nil, nil) when absent or expired past the 30-day TTL.
	Get(ctx context.Context, textHash, model string) ([]float32, error)
	// Put stores (or refreshes) the embedding for (textHash, model).
	Put(ctx context.Context, textHash, model string, embedding []float32) error
}
