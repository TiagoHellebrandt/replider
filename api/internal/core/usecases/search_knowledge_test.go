package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestSearchKnowledgeUseCase_Execute(t *testing.T) {
	ctx := context.Background()
	queryEmbedding := []float32{0.1, 0.2, 0.3}
	provider := "openai"
	providerPtr := &provider

	t.Run("should generate embedding, find chunks and return them", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		aiProvider := new(MockAIProvider)

		chunks := []*domain.KnowledgeChunk{
			{ID: "c1", SourceID: "src1", Content: "Hello world", Provider: providerPtr},
			{ID: "c2", SourceID: "src1", Content: "Goodbye world", Provider: providerPtr},
		}

		aiProvider.On("GetEmbedding", ctx, "my query").Return(queryEmbedding, nil)
		knowledgeRepo.On("Search", ctx, "src1", queryEmbedding, int32(5)).Return(chunks, nil)

		uc := NewSearchKnowledgeUseCase(knowledgeRepo, aiProvider)
		result, err := uc.Execute(ctx, "src1", "my query", 5, provider)

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "c1", result[0].ID)
		assert.Equal(t, "c2", result[1].ID)
		knowledgeRepo.AssertExpectations(t)
		aiProvider.AssertExpectations(t)
	})

	t.Run("should return empty slice when no chunks found", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		aiProvider := new(MockAIProvider)

		aiProvider.On("GetEmbedding", ctx, "obscure query").Return(queryEmbedding, nil)
		knowledgeRepo.On("Search", ctx, "src1", queryEmbedding, int32(5)).Return([]*domain.KnowledgeChunk{}, nil)

		uc := NewSearchKnowledgeUseCase(knowledgeRepo, aiProvider)
		result, err := uc.Execute(ctx, "src1", "obscure query", 5, provider)

		assert.NoError(t, err)
		assert.Empty(t, result)
		knowledgeRepo.AssertExpectations(t)
		aiProvider.AssertExpectations(t)
	})

	t.Run("should return error when embedding generation fails", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		aiProvider := new(MockAIProvider)

		embeddingErr := errors.New("ai provider unavailable")
		aiProvider.On("GetEmbedding", ctx, "my query").Return(nil, embeddingErr)

		uc := NewSearchKnowledgeUseCase(knowledgeRepo, aiProvider)
		result, err := uc.Execute(ctx, "src1", "my query", 5, provider)

		assert.Nil(t, result)
		assert.Equal(t, embeddingErr, err)
		aiProvider.AssertExpectations(t)
	})

	t.Run("should truncate results when total content exceeds 2000 chars", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		aiProvider := new(MockAIProvider)

		longContent := strings.Repeat("a", 1500)
		chunks := []*domain.KnowledgeChunk{
			{ID: "c1", SourceID: "src1", Content: longContent, Provider: providerPtr},
			{ID: "c2", SourceID: "src1", Content: strings.Repeat("b", 600), Provider: providerPtr},
			{ID: "c3", SourceID: "src1", Content: "short", Provider: providerPtr},
		}

		aiProvider.On("GetEmbedding", ctx, "big query").Return(queryEmbedding, nil)
		knowledgeRepo.On("Search", ctx, "src1", queryEmbedding, int32(10)).Return(chunks, nil)

		uc := NewSearchKnowledgeUseCase(knowledgeRepo, aiProvider)
		result, err := uc.Execute(ctx, "src1", "big query", 10, provider)

		assert.NoError(t, err)
		// First chunk is 1500 chars, second is 600 -> total 2100 > 2000, so only first is kept
		assert.Len(t, result, 1)
		assert.Equal(t, "c1", result[0].ID)
		knowledgeRepo.AssertExpectations(t)
		aiProvider.AssertExpectations(t)
	})

	t.Run("should lazily re-embed only the mismatched chunks, not the whole source", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		aiProvider := new(MockAIProvider)

		oldProvider := "google"
		oldProviderPtr := &oldProvider

		// Search returns one stale chunk (c1) and one already-current (c2).
		searchChunks := []*domain.KnowledgeChunk{
			{ID: "c1", SourceID: "src1", Content: "chunk one", Provider: oldProviderPtr},
			{ID: "c2", SourceID: "src1", Content: "chunk two", Provider: providerPtr},
		}

		newEmbedding := []float32{0.4, 0.5, 0.6}

		aiProvider.On("GetEmbedding", ctx, "my query").Return(queryEmbedding, nil)
		knowledgeRepo.On("Search", ctx, "src1", queryEmbedding, int32(5)).Return(searchChunks, nil).Once()
		// Only the stale chunk is re-embedded — no full-source ListChunksBySource.
		aiProvider.On("GetEmbedding", ctx, "chunk one").Return(newEmbedding, nil)
		knowledgeRepo.On("UpdateEmbedding", ctx, "c1", newEmbedding, provider).Return(nil)

		uc := NewSearchKnowledgeUseCase(knowledgeRepo, aiProvider)
		result, err := uc.Execute(ctx, "src1", "my query", 5, provider)

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		knowledgeRepo.AssertNotCalled(t, "ListChunksBySource", ctx, "src1")
		knowledgeRepo.AssertCalled(t, "UpdateEmbedding", ctx, "c1", newEmbedding, provider)
		knowledgeRepo.AssertNotCalled(t, "UpdateEmbedding", ctx, "c2", newEmbedding, provider)
		// The matching chunk is never re-embedded.
		aiProvider.AssertNotCalled(t, "GetEmbedding", ctx, "chunk two")
		// Returned stale chunk reflects the fresh provider, no re-search needed.
		assert.Equal(t, provider, *result[0].Provider)
		assert.EqualValues(t, 1, uc.RegenCount())
		aiProvider.AssertExpectations(t)
	})

	t.Run("should re-embed a chunk with nil provider", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		aiProvider := new(MockAIProvider)

		chunks := []*domain.KnowledgeChunk{
			{ID: "c1", SourceID: "src1", Content: "old chunk", Provider: nil},
		}

		newEmbedding := []float32{0.7, 0.8, 0.9}

		aiProvider.On("GetEmbedding", ctx, "test query").Return(queryEmbedding, nil)
		knowledgeRepo.On("Search", ctx, "src1", queryEmbedding, int32(5)).Return(chunks, nil).Once()
		aiProvider.On("GetEmbedding", ctx, "old chunk").Return(newEmbedding, nil)
		knowledgeRepo.On("UpdateEmbedding", ctx, "c1", newEmbedding, provider).Return(nil)

		uc := NewSearchKnowledgeUseCase(knowledgeRepo, aiProvider)
		result, err := uc.Execute(ctx, "src1", "test query", 5, provider)

		assert.NoError(t, err)
		assert.Len(t, result, 1)
		knowledgeRepo.AssertNotCalled(t, "ListChunksBySource", ctx, "src1")
		knowledgeRepo.AssertCalled(t, "UpdateEmbedding", ctx, "c1", newEmbedding, provider)
		assert.EqualValues(t, 1, uc.RegenCount())
	})

	t.Run("should throttle re-embedding to the per-minute cap", func(t *testing.T) {
		knowledgeRepo := new(MockKnowledgeRepo)
		aiProvider := new(MockAIProvider)

		oldProvider := "google"
		oldProviderPtr := &oldProvider

		chunks := []*domain.KnowledgeChunk{
			{ID: "c1", SourceID: "src1", Content: "chunk one", Provider: oldProviderPtr},
			{ID: "c2", SourceID: "src1", Content: "chunk two", Provider: oldProviderPtr},
			{ID: "c3", SourceID: "src1", Content: "chunk three", Provider: oldProviderPtr},
		}

		newEmbedding := []float32{0.4, 0.5, 0.6}

		aiProvider.On("GetEmbedding", ctx, "my query").Return(queryEmbedding, nil)
		knowledgeRepo.On("Search", ctx, "src1", queryEmbedding, int32(5)).Return(chunks, nil).Once()
		// Only the first chunk is allowed through the throttle.
		aiProvider.On("GetEmbedding", ctx, "chunk one").Return(newEmbedding, nil)
		knowledgeRepo.On("UpdateEmbedding", ctx, "c1", newEmbedding, provider).Return(nil)

		uc := NewSearchKnowledgeUseCase(knowledgeRepo, aiProvider)
		uc.maxRegenPerMin = 1 // cap at one regen per minute

		result, err := uc.Execute(ctx, "src1", "my query", 5, provider)

		assert.NoError(t, err)
		assert.Len(t, result, 3)
		assert.EqualValues(t, 1, uc.RegenCount())
		knowledgeRepo.AssertCalled(t, "UpdateEmbedding", ctx, "c1", newEmbedding, provider)
		// c2/c3 deferred — never embedded or persisted this turn.
		aiProvider.AssertNotCalled(t, "GetEmbedding", ctx, "chunk two")
		aiProvider.AssertNotCalled(t, "GetEmbedding", ctx, "chunk three")
		knowledgeRepo.AssertNotCalled(t, "UpdateEmbedding", ctx, "c2", newEmbedding, provider)
		knowledgeRepo.AssertNotCalled(t, "UpdateEmbedding", ctx, "c3", newEmbedding, provider)
	})
}
