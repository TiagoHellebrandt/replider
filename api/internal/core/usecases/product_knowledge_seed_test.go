package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	pksPlatformID = "01PLATFORMACCOUNT000000000"
	pksAgentID    = "01AGENTPLATFORM0000000000A"
)

func pksInternalChannel(agentID string) *domain.Channel {
	ch := &domain.Channel{
		ID:        "chan-internal",
		AccountID: pksPlatformID,
		Type:      domain.ChannelTypeInternal,
		IsActive:  true,
	}
	if agentID != "" {
		ch.AgentID = &agentID
	}
	return ch
}

func newProductKnowledgeFixture() (*EnsureProductKnowledgeUseCase, *MockChannelRepo, *MockKnowledgeRepo, *MockAIProvider) {
	channelRepo := new(MockChannelRepo)
	knowledgeRepo := new(MockKnowledgeRepo)
	ai := new(MockAIProvider)
	return NewEnsureProductKnowledgeUseCase(channelRepo, knowledgeRepo, ai), channelRepo, knowledgeRepo, ai
}

func TestEnsureProductKnowledge_SeedsSourceAndAllChunks(t *testing.T) {
	uc, channelRepo, knowledgeRepo, ai := newProductKnowledgeFixture()
	channelRepo.On("ListByAccountID", mock.Anything, pksPlatformID).
		Return([]*domain.Channel{pksInternalChannel(pksAgentID)}, nil)
	knowledgeRepo.On("ListSources", mock.Anything, pksAgentID).
		Return([]*domain.KnowledgeSource{}, nil)
	knowledgeRepo.On("CreateSource", mock.Anything, mock.MatchedBy(func(s *domain.KnowledgeSource) bool {
		return s.AgentID == pksAgentID && s.Name == ProductFAQSourceName
	})).Return(nil)
	knowledgeRepo.On("ListChunksBySource", mock.Anything, mock.Anything).
		Return([]*domain.KnowledgeChunk{}, nil)
	ai.On("GetEmbedding", mock.Anything, mock.Anything).Return([]float32{0.1}, nil)
	knowledgeRepo.On("CreateChunk", mock.Anything, mock.MatchedBy(func(c *domain.KnowledgeChunk) bool {
		return c.Content != "" && c.SourceName != nil && *c.SourceName == ProductFAQSourceName
	})).Return(nil)

	created, err := uc.Execute(context.Background(), pksPlatformID)
	assert.NoError(t, err)
	assert.Equal(t, len(productFAQChunks), created)
	knowledgeRepo.AssertNumberOfCalls(t, "CreateChunk", len(productFAQChunks))
}

func TestEnsureProductKnowledge_IdempotentWhenFullySeeded(t *testing.T) {
	uc, channelRepo, knowledgeRepo, ai := newProductKnowledgeFixture()
	channelRepo.On("ListByAccountID", mock.Anything, pksPlatformID).
		Return([]*domain.Channel{pksInternalChannel(pksAgentID)}, nil)
	knowledgeRepo.On("ListSources", mock.Anything, pksAgentID).
		Return([]*domain.KnowledgeSource{{ID: "src-faq", AgentID: pksAgentID, Name: ProductFAQSourceName}}, nil)
	existing := make([]*domain.KnowledgeChunk, 0, len(productFAQChunks))
	for _, content := range productFAQChunks {
		existing = append(existing, &domain.KnowledgeChunk{Content: content})
	}
	knowledgeRepo.On("ListChunksBySource", mock.Anything, "src-faq").Return(existing, nil)

	created, err := uc.Execute(context.Background(), pksPlatformID)
	assert.NoError(t, err)
	assert.Equal(t, 0, created)
	knowledgeRepo.AssertNotCalled(t, "CreateSource", mock.Anything, mock.Anything)
	knowledgeRepo.AssertNotCalled(t, "CreateChunk", mock.Anything, mock.Anything)
	ai.AssertNotCalled(t, "GetEmbedding", mock.Anything, mock.Anything)
}

// A boot that died mid-seed (or a new FAQ entry shipped later) only
// ingests what's missing — content-based idempotency.
func TestEnsureProductKnowledge_ResumesPartialSeed(t *testing.T) {
	uc, channelRepo, knowledgeRepo, ai := newProductKnowledgeFixture()
	channelRepo.On("ListByAccountID", mock.Anything, pksPlatformID).
		Return([]*domain.Channel{pksInternalChannel(pksAgentID)}, nil)
	knowledgeRepo.On("ListSources", mock.Anything, pksAgentID).
		Return([]*domain.KnowledgeSource{{ID: "src-faq", AgentID: pksAgentID, Name: ProductFAQSourceName}}, nil)
	// First two chunks already ingested.
	knowledgeRepo.On("ListChunksBySource", mock.Anything, "src-faq").
		Return([]*domain.KnowledgeChunk{
			{Content: productFAQChunks[0]},
			{Content: productFAQChunks[1]},
		}, nil)
	ai.On("GetEmbedding", mock.Anything, mock.Anything).Return([]float32{0.1}, nil)
	knowledgeRepo.On("CreateChunk", mock.Anything, mock.Anything).Return(nil)

	created, err := uc.Execute(context.Background(), pksPlatformID)
	assert.NoError(t, err)
	assert.Equal(t, len(productFAQChunks)-2, created)
}

func TestEnsureProductKnowledge_SkipsWithoutPlatformOrAgent(t *testing.T) {
	// Feature off: blank platform account.
	uc, _, _, _ := newProductKnowledgeFixture()
	created, err := uc.Execute(context.Background(), "")
	assert.NoError(t, err)
	assert.Equal(t, 0, created)

	// Internal channel exists but no agent assigned yet.
	uc2, channelRepo2, knowledgeRepo2, _ := newProductKnowledgeFixture()
	channelRepo2.On("ListByAccountID", mock.Anything, pksPlatformID).
		Return([]*domain.Channel{pksInternalChannel("")}, nil)
	created, err = uc2.Execute(context.Background(), pksPlatformID)
	assert.NoError(t, err)
	assert.Equal(t, 0, created)
	knowledgeRepo2.AssertNotCalled(t, "ListSources", mock.Anything, mock.Anything)

	// No internal channel at all.
	uc3, channelRepo3, _, _ := newProductKnowledgeFixture()
	channelRepo3.On("ListByAccountID", mock.Anything, pksPlatformID).
		Return([]*domain.Channel{{ID: "chan-wa", AccountID: pksPlatformID, Type: "whatsapp", IsActive: true}}, nil)
	created, err = uc3.Execute(context.Background(), pksPlatformID)
	assert.NoError(t, err)
	assert.Equal(t, 0, created)
}

// An embedding failure aborts the run but keeps what already landed —
// the next boot resumes from the missing chunks.
func TestEnsureProductKnowledge_EmbeddingErrorAbortsAndIsRetryable(t *testing.T) {
	uc, channelRepo, knowledgeRepo, ai := newProductKnowledgeFixture()
	channelRepo.On("ListByAccountID", mock.Anything, pksPlatformID).
		Return([]*domain.Channel{pksInternalChannel(pksAgentID)}, nil)
	knowledgeRepo.On("ListSources", mock.Anything, pksAgentID).
		Return([]*domain.KnowledgeSource{{ID: "src-faq", AgentID: pksAgentID, Name: ProductFAQSourceName}}, nil)
	knowledgeRepo.On("ListChunksBySource", mock.Anything, "src-faq").
		Return([]*domain.KnowledgeChunk{}, nil)
	// First embedding succeeds, second blows up.
	ai.On("GetEmbedding", mock.Anything, productFAQChunks[0]).Return([]float32{0.1}, nil).Once()
	ai.On("GetEmbedding", mock.Anything, mock.Anything).Return(nil, assert.AnError)
	knowledgeRepo.On("CreateChunk", mock.Anything, mock.Anything).Return(nil)

	created, err := uc.Execute(context.Background(), pksPlatformID)
	assert.Error(t, err)
	assert.Equal(t, 1, created)
	knowledgeRepo.AssertNumberOfCalls(t, "CreateChunk", 1)
}
