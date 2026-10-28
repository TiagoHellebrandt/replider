package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync/atomic"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubEmbeddingCache is a minimal in-memory EmbeddingCacheRepository for
// US-022 tests. It records hits/puts so assertions can prove the cache
// short-circuited the provider call.
type stubEmbeddingCache struct {
	store    map[string][]float32
	getCalls atomic.Int32
	putCalls atomic.Int32
}

func newStubEmbeddingCache() *stubEmbeddingCache {
	return &stubEmbeddingCache{store: map[string][]float32{}}
}

func (s *stubEmbeddingCache) key(textHash, model string) string { return model + ":" + textHash }

func (s *stubEmbeddingCache) Get(_ context.Context, textHash, model string) ([]float32, error) {
	s.getCalls.Add(1)
	if v, ok := s.store[s.key(textHash, model)]; ok {
		return v, nil
	}
	return nil, nil
}

func (s *stubEmbeddingCache) Put(_ context.Context, textHash, model string, embedding []float32) error {
	s.putCalls.Add(1)
	s.store[s.key(textHash, model)] = embedding
	return nil
}

func hashFor(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// On a cache hit the provider's GetEmbedding must NOT be invoked.
func TestApplyRAG_EmbeddingCacheHit_SkipsProvider(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)
	aiProvider := new(MockAIProvider)

	agent := makeAgent(nil) // Provider: "gemini"
	source := makeSource("src_001")
	embedding := []float32{0.5, 0.6}
	query := "Quais os horários?"

	cache := newStubEmbeddingCache()
	cache.store[cache.key(hashFor(query), "gemini")] = embedding

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{source}, nil)
	aiFactory.On("GetProvider", ctx, "gemini").Return(aiProvider, nil)
	knowledgeRepo.On("Search", ctx, "src_001", embedding, int32(5)).Return([]*domain.KnowledgeChunk{
		makeChunk("chunk_1", "src_001", "Abrimos das 9h às 18h."),
	}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{}, nil)
	// Deliberately register NO GetEmbedding expectation: a hit must not call it.

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	runner.SetEmbeddingCache(cache)

	result, err := runner.BuildSystemPrompt(ctx, agent, query, "", "pt")
	require.NoError(t, err)
	assert.Contains(t, result.SystemPrompt, "Abrimos das 9h às 18h.")

	aiProvider.AssertNotCalled(t, "GetEmbedding", ctx, query)
	assert.Equal(t, int32(1), cache.getCalls.Load(), "cache should be consulted once")
	assert.Equal(t, int32(0), cache.putCalls.Load(), "a hit must not write back")
	assert.Equal(t, 1.0, runner.EmbeddingCacheHitRate(), "single hit → 100% hit rate")
}

// On a cache miss the provider is called once and the result is stored.
func TestApplyRAG_EmbeddingCacheMiss_CallsProviderAndStores(t *testing.T) {
	ctx := context.Background()
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)
	aiFactory := new(MockAIFactory)
	aiProvider := new(MockAIProvider)

	agent := makeAgent(nil)
	source := makeSource("src_001")
	embedding := []float32{0.1, 0.2}
	query := "algo novo"

	cache := newStubEmbeddingCache()

	knowledgeRepo.On("ListSources", ctx, "agent_001").Return([]*domain.KnowledgeSource{source}, nil)
	aiFactory.On("GetProvider", ctx, "gemini").Return(aiProvider, nil)
	aiProvider.On("GetEmbedding", ctx, query).Return(embedding, nil)
	knowledgeRepo.On("Search", ctx, "src_001", embedding, int32(5)).Return([]*domain.KnowledgeChunk{}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agent_001").Return([]*domain.Calendar{}, nil)

	runner := NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	runner.SetEmbeddingCache(cache)

	_, err := runner.BuildSystemPrompt(ctx, agent, query, "", "pt")
	require.NoError(t, err)

	aiProvider.AssertCalled(t, "GetEmbedding", ctx, query)
	assert.Equal(t, int32(1), cache.putCalls.Load(), "miss must store the embedding")
	assert.Equal(t, embedding, cache.store[cache.key(hashFor(query), "gemini")])
	assert.Equal(t, 0.0, runner.EmbeddingCacheHitRate(), "single miss → 0% hit rate")
}
