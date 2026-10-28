package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestLearnPersonality_Success(t *testing.T) {
	ctx := context.Background()
	agentRepo := new(MockAgentRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	aiFactory := new(MockAIFactory)
	aiProvider := new(MockAIProvider)

	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Provider: "gemini"}
	agentRepo.On("GetByID", ctx, "agent-1").Return(agent, nil)

	channelRepo.On("ListByAgentID", ctx, "agent-1").Return([]*domain.Channel{
		{ID: "ch-1", AccountID: "acc-1"},
	}, nil)

	convRepo.On("ListByAccountID", ctx, "acc-1", "").Return([]*domain.Conversation{
		{ID: "conv-1", ChannelID: "ch-1"},
	}, nil)

	// Build 6 human outbound messages (>= 5 minimum)
	content1 := "Olá! Como posso ajudar?"
	content2 := "Vou verificar isso para você."
	content3 := "Pronto, está resolvido!"
	content4 := "Qualquer dúvida, estamos aqui."
	content5 := "Obrigado pela preferência!"
	content6 := "Bom dia, tudo bem?"
	msgRepo.On("ListByConversationID", ctx, "conv-1", int32(100), int32(0)).Return([]*domain.Message{
		{Direction: "outbound", From: "user-1", Content: &content1},
		{Direction: "outbound", From: "user-1", Content: &content2},
		{Direction: "outbound", From: "user-1", Content: &content3},
		{Direction: "outbound", From: "user-1", Content: &content4},
		{Direction: "outbound", From: "user-1", Content: &content5},
		{Direction: "outbound", From: "user-1", Content: &content6},
	}, nil)

	aiFactory.On("GetProvider", ctx, "gemini").Return(aiProvider, nil)
	aiProvider.On("GenerateResponse", ctx, mock.Anything, mock.Anything, mock.Anything).Return(&domain.AICallbackResponse{
		Content: "Tom informal, usa emojis, respostas curtas.",
	}, nil)

	agentRepo.On("Update", ctx, mock.MatchedBy(func(a *domain.Agent) bool {
		return a.PersonalityPrompt != nil && *a.PersonalityPrompt == "Tom informal, usa emojis, respostas curtas."
	})).Return(nil)

	uc := NewLearnPersonalityUseCase(agentRepo, convRepo, msgRepo, channelRepo, aiFactory)
	result, err := uc.Execute(ctx, "agent-1", "acc-1")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.PersonalityPrompt)
	assert.True(t, result.LearnFromHumans)
	agentRepo.AssertExpectations(t)
	aiProvider.AssertExpectations(t)
}

func TestLearnPersonality_AgentNotFound(t *testing.T) {
	ctx := context.Background()
	agentRepo := new(MockAgentRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	aiFactory := new(MockAIFactory)

	agentRepo.On("GetByID", ctx, "agent-404").Return((*domain.Agent)(nil), nil)

	uc := NewLearnPersonalityUseCase(agentRepo, convRepo, msgRepo, channelRepo, aiFactory)
	result, err := uc.Execute(ctx, "agent-404", "acc-1")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, domain.ErrAgentNotFound, err)
}

func TestLearnPersonality_UnauthorizedAccount(t *testing.T) {
	ctx := context.Background()
	agentRepo := new(MockAgentRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	aiFactory := new(MockAIFactory)

	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-other"}
	agentRepo.On("GetByID", ctx, "agent-1").Return(agent, nil)

	uc := NewLearnPersonalityUseCase(agentRepo, convRepo, msgRepo, channelRepo, aiFactory)
	result, err := uc.Execute(ctx, "agent-1", "acc-1")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, domain.ErrUnauthorized, err)
}

func TestLearnPersonality_InsufficientMessages(t *testing.T) {
	ctx := context.Background()
	agentRepo := new(MockAgentRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	aiFactory := new(MockAIFactory)

	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Provider: "gemini"}
	agentRepo.On("GetByID", ctx, "agent-1").Return(agent, nil)

	channelRepo.On("ListByAgentID", ctx, "agent-1").Return([]*domain.Channel{
		{ID: "ch-1", AccountID: "acc-1"},
	}, nil)

	convRepo.On("ListByAccountID", ctx, "acc-1", "").Return([]*domain.Conversation{
		{ID: "conv-1", ChannelID: "ch-1"},
	}, nil)

	// Only 2 human messages (< 5 minimum)
	content1 := "Olá"
	content2 := "Tudo bem?"
	msgRepo.On("ListByConversationID", ctx, "conv-1", int32(100), int32(0)).Return([]*domain.Message{
		{Direction: "outbound", From: "user-1", Content: &content1},
		{Direction: "outbound", From: "user-1", Content: &content2},
		{Direction: "inbound", From: "customer", Content: &content1}, // inbound, ignored
		{Direction: "outbound", From: "bot", Content: &content1},     // bot, ignored
	}, nil)

	uc := NewLearnPersonalityUseCase(agentRepo, convRepo, msgRepo, channelRepo, aiFactory)
	result, err := uc.Execute(ctx, "agent-1", "acc-1")

	assert.Error(t, err)
	assert.Nil(t, result)

	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "INSUFFICIENT_DATA", appErr.Code)
}

func TestLearnPersonality_AIProviderError(t *testing.T) {
	ctx := context.Background()
	agentRepo := new(MockAgentRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	aiFactory := new(MockAIFactory)

	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Provider: "gemini"}
	agentRepo.On("GetByID", ctx, "agent-1").Return(agent, nil)

	channelRepo.On("ListByAgentID", ctx, "agent-1").Return([]*domain.Channel{
		{ID: "ch-1", AccountID: "acc-1"},
	}, nil)

	convRepo.On("ListByAccountID", ctx, "acc-1", "").Return([]*domain.Conversation{
		{ID: "conv-1", ChannelID: "ch-1"},
	}, nil)

	msgs := buildHumanMessages(6)
	msgRepo.On("ListByConversationID", ctx, "conv-1", int32(100), int32(0)).Return(msgs, nil)

	aiFactory.On("GetProvider", ctx, "gemini").Return(nil, errors.New("provider unavailable"))

	uc := NewLearnPersonalityUseCase(agentRepo, convRepo, msgRepo, channelRepo, aiFactory)
	result, err := uc.Execute(ctx, "agent-1", "acc-1")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get AI provider")
}

func TestLearnPersonality_NoChannels(t *testing.T) {
	ctx := context.Background()
	agentRepo := new(MockAgentRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	aiFactory := new(MockAIFactory)

	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Provider: "gemini"}
	agentRepo.On("GetByID", ctx, "agent-1").Return(agent, nil)

	channelRepo.On("ListByAgentID", ctx, "agent-1").Return([]*domain.Channel{}, nil)

	uc := NewLearnPersonalityUseCase(agentRepo, convRepo, msgRepo, channelRepo, aiFactory)
	result, err := uc.Execute(ctx, "agent-1", "acc-1")

	assert.Error(t, err)
	assert.Nil(t, result)
	// No channels → no messages → INSUFFICIENT_DATA
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "INSUFFICIENT_DATA", appErr.Code)
}

func TestLearnPersonality_ChannelListError(t *testing.T) {
	ctx := context.Background()
	agentRepo := new(MockAgentRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	channelRepo := new(MockChannelRepo)
	aiFactory := new(MockAIFactory)

	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Provider: "gemini"}
	agentRepo.On("GetByID", ctx, "agent-1").Return(agent, nil)

	channelRepo.On("ListByAgentID", ctx, "agent-1").Return(nil, errors.New("db error"))

	uc := NewLearnPersonalityUseCase(agentRepo, convRepo, msgRepo, channelRepo, aiFactory)
	result, err := uc.Execute(ctx, "agent-1", "acc-1")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to list channels")
}

// buildHumanMessages creates n outbound human messages for testing.
func buildHumanMessages(n int) []*domain.Message {
	msgs := make([]*domain.Message, n)
	for i := 0; i < n; i++ {
		content := "Mensagem " + string(rune('A'+i))
		msgs[i] = &domain.Message{
			Direction: "outbound",
			From:      "user-1",
			Content:   &content,
		}
	}
	return msgs
}
