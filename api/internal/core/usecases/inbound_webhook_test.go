package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newInboundWebhookUseCase(
	channelRepo *MockChannelRepo,
	convRepo *MockConversationRepo,
	msgRepo *MockMessageRepo,
	processMsg *ProcessMessageUseCase,
	agentRepo *MockAgentRepo,
) *InboundWebhookUseCase {
	return NewInboundWebhookUseCase(channelRepo, convRepo, msgRepo, processMsg, agentRepo)
}

func TestSaveInbound(t *testing.T) {
	ctx := context.Background()

	baseChannel := &domain.Channel{
		AccountID: "acc-001",
		ID:        "chan-001",
		AgentID:   strPtr("agent-001"),
		Type:      "whatsapp",
		Name:      "Main WhatsApp",
		IsActive:  true,
	}

	baseDTO := InboundMessageDTO{
		ChannelToken:   "tok-abc",
		SenderPhone:    "+5511999999999",
		SenderName:     "John Doe",
		SenderUsername: "@johndoe",
		MessageType:    "text",
		Content:        "Hello, I need help",
		Metadata:       nil,
	}

	t.Run("happy path: new conversation created when FindByContact returns nil", func(t *testing.T) {
		channelRepo := new(MockChannelRepo)
		convRepo := new(MockConversationRepo)
		convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		msgRepo := new(MockMessageRepo)
		agentRepo := new(MockAgentRepo)

		uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)

		// SaveInbound no longer calls agentRepo.GetByID — it derives
		// accountID directly from channel.AccountID.

		convRepo.On("FindByContact", ctx, "chan-001", "+5511999999999").Return(nil, nil)

		convRepo.On("Create", ctx, mock.MatchedBy(func(c *domain.Conversation) bool {
			return c.AccountID == "acc-001" &&
				c.ChannelID == "chan-001" &&
				c.ContactID == "+5511999999999" &&
				c.Status == domain.ConversationOpen &&
				!c.IsBotPaused &&
				c.AgentID != nil && *c.AgentID == "agent-001" &&
				c.LastCustomerMessageAt != nil &&
				c.WaitingSince != nil &&
				len(c.ID) == 26
		})).Return(nil)

		msgRepo.On("Create", ctx, mock.MatchedBy(func(m *domain.Message) bool {
			return m.ChannelID == "chan-001" &&
				m.From == "+5511999999999" &&
				m.Type == "text" &&
				m.Content != nil && *m.Content == "Hello, I need help" &&
				m.Direction == "inbound" &&
				m.ConversationID != nil &&
				len(m.ID) == 26
		})).Return(nil)

		result, err := uc.SaveInbound(ctx, baseChannel, baseDTO)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "acc-001", result.AccountID)
		assert.Equal(t, "chan-001", result.ChannelID)
		assert.NotNil(t, result.AgentID)
		assert.Equal(t, "agent-001", *result.AgentID)
		assert.Equal(t, "+5511999999999", result.ContactID)
		assert.False(t, result.IsBotPaused)
		assert.NotNil(t, result.Message)
		assert.Equal(t, "inbound", result.Message.Direction)

		agentRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
		convRepo.AssertExpectations(t)
		msgRepo.AssertExpectations(t)
	})

	t.Run("happy path: existing conversation updates last_customer_message_at", func(t *testing.T) {
		channelRepo := new(MockChannelRepo)
		convRepo := new(MockConversationRepo)
		convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		msgRepo := new(MockMessageRepo)
		agentRepo := new(MockAgentRepo)

		uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)

		existingConv := &domain.Conversation{
			ID:          "conv-existing",
			AccountID:   "acc-001",
			ChannelID:   "chan-001",
			ContactID:   "+5511999999999",
			Status:      domain.ConversationOpen,
			IsBotPaused: false,
		}
		convRepo.On("FindByContact", ctx, "chan-001", "+5511999999999").Return(existingConv, nil)
		convRepo.On("UpdateLastCustomerMessageAt", ctx, "conv-existing", "acc-001").Return(nil)

		msgRepo.On("Create", ctx, mock.MatchedBy(func(m *domain.Message) bool {
			return m.ConversationID != nil && *m.ConversationID == "conv-existing" &&
				m.Direction == "inbound"
		})).Return(nil)

		result, err := uc.SaveInbound(ctx, baseChannel, baseDTO)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "conv-existing", result.ConversationID)

		convRepo.AssertExpectations(t)
		convRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		msgRepo.AssertExpectations(t)
	})

	t.Run("channel without agent creates conversation with nil AgentID", func(t *testing.T) {
		channelRepo := new(MockChannelRepo)
		convRepo := new(MockConversationRepo)
		convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		msgRepo := new(MockMessageRepo)
		agentRepo := new(MockAgentRepo)

		uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)

		noAgentChannel := &domain.Channel{
			AccountID: "acc-001",
			ID:        "chan-002",
			AgentID:   nil, // human-only channel
			Type:      "telegram",
			Name:      "Human Only",
			IsActive:  true,
		}

		convRepo.On("FindByContact", ctx, "chan-002", "+5511999999999").Return(nil, nil)
		convRepo.On("Create", ctx, mock.MatchedBy(func(c *domain.Conversation) bool {
			return c.AccountID == "acc-001" &&
				c.ChannelID == "chan-002" &&
				c.AgentID == nil // no agent
		})).Return(nil)
		msgRepo.On("Create", ctx, mock.AnythingOfType("*domain.Message")).Return(nil)

		result, err := uc.SaveInbound(ctx, noAgentChannel, baseDTO)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "acc-001", result.AccountID)
		assert.Nil(t, result.AgentID)
		agentRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	})

	t.Run("conversation creation fails returns error", func(t *testing.T) {
		channelRepo := new(MockChannelRepo)
		convRepo := new(MockConversationRepo)
		convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		msgRepo := new(MockMessageRepo)
		agentRepo := new(MockAgentRepo)

		uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)

		convRepo.On("FindByContact", ctx, "chan-001", "+5511999999999").Return(nil, nil)
		convRepo.On("Create", ctx, mock.Anything).Return(errors.New("db connection failed"))

		result, err := uc.SaveInbound(ctx, baseChannel, baseDTO)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to create conversation")

		convRepo.AssertExpectations(t)
	})

	t.Run("message creation fails returns error", func(t *testing.T) {
		channelRepo := new(MockChannelRepo)
		convRepo := new(MockConversationRepo)
		convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		msgRepo := new(MockMessageRepo)
		agentRepo := new(MockAgentRepo)

		uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)

		convRepo.On("FindByContact", ctx, "chan-001", "+5511999999999").Return(nil, nil)
		convRepo.On("Create", ctx, mock.Anything).Return(nil)

		msgRepo.On("Create", ctx, mock.Anything).Return(errors.New("disk full"))

		result, err := uc.SaveInbound(ctx, baseChannel, baseDTO)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to save inbound message")

		msgRepo.AssertExpectations(t)
	})

	t.Run("SenderName and SenderUsername populated on new conversation", func(t *testing.T) {
		channelRepo := new(MockChannelRepo)
		convRepo := new(MockConversationRepo)
		convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		msgRepo := new(MockMessageRepo)
		agentRepo := new(MockAgentRepo)

		uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)

		convRepo.On("FindByContact", ctx, "chan-001", "+5511999999999").Return(nil, nil)

		convRepo.On("Create", ctx, mock.MatchedBy(func(c *domain.Conversation) bool {
			return c.ContactName != nil && *c.ContactName == "John Doe" &&
				c.ContactUsername != nil && *c.ContactUsername == "@johndoe"
		})).Return(nil)

		msgRepo.On("Create", ctx, mock.MatchedBy(func(m *domain.Message) bool {
			return m.SenderName != nil && *m.SenderName == "John Doe"
		})).Return(nil)

		result, err := uc.SaveInbound(ctx, baseChannel, baseDTO)

		assert.NoError(t, err)
		assert.NotNil(t, result)

		convRepo.AssertExpectations(t)
		msgRepo.AssertExpectations(t)
	})

	t.Run("empty SenderName and SenderUsername result in nil fields", func(t *testing.T) {
		channelRepo := new(MockChannelRepo)
		convRepo := new(MockConversationRepo)
		convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		msgRepo := new(MockMessageRepo)
		agentRepo := new(MockAgentRepo)

		uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)

		dtoNoName := InboundMessageDTO{
			ChannelToken:   "tok-abc",
			SenderPhone:    "+5511999999999",
			SenderName:     "",
			SenderUsername: "",
			MessageType:    "text",
			Content:        "Hi",
		}

		convRepo.On("FindByContact", ctx, "chan-001", "+5511999999999").Return(nil, nil)

		convRepo.On("Create", ctx, mock.MatchedBy(func(c *domain.Conversation) bool {
			return c.ContactName == nil && c.ContactUsername == nil
		})).Return(nil)

		msgRepo.On("Create", ctx, mock.MatchedBy(func(m *domain.Message) bool {
			return m.SenderName == nil
		})).Return(nil)

		result, err := uc.SaveInbound(ctx, baseChannel, dtoNoName)

		assert.NoError(t, err)
		assert.NotNil(t, result)

		convRepo.AssertExpectations(t)
		msgRepo.AssertExpectations(t)
	})
}

func TestProcessAI(t *testing.T) {
	ctx := context.Background()

	t.Run("bot is paused returns nil immediately", func(t *testing.T) {
		channelRepo := new(MockChannelRepo)
		convRepo := new(MockConversationRepo)
		convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		msgRepo := new(MockMessageRepo)
		agentRepo := new(MockAgentRepo)

		uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)

		partial := &InboundPartialResult{
			AccountID:      "acc-001",
			ConversationID: "conv-001",
			ChannelID:      "chan-001",
			AgentID:        strPtr("agent-001"),
			IsBotPaused:    true,
			ContactID:      "+5511999999999",
		}

		result := uc.ProcessAI(ctx, partial, "Hello")

		assert.Nil(t, result)
		agentRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	})

	t.Run("agent not found returns nil", func(t *testing.T) {
		channelRepo := new(MockChannelRepo)
		convRepo := new(MockConversationRepo)
		convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		msgRepo := new(MockMessageRepo)
		agentRepo := new(MockAgentRepo)

		uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)

		agentRepo.On("GetByID", mock.Anything, "agent-001").Return(nil, nil)

		partial := &InboundPartialResult{
			AccountID:      "acc-001",
			ConversationID: "conv-001",
			ChannelID:      "chan-001",
			AgentID:        strPtr("agent-001"),
			IsBotPaused:    false,
			ContactID:      "+5511999999999",
		}

		result := uc.ProcessAI(ctx, partial, "Hello")

		assert.Nil(t, result)
		agentRepo.AssertExpectations(t)
	})

	t.Run("AI processing succeeds saves and returns AI message", func(t *testing.T) {
		channelRepo := new(MockChannelRepo)
		convRepo := new(MockConversationRepo)
		convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		msgRepo := new(MockMessageRepo)
		agentRepo := new(MockAgentRepo)
		usageRepo := new(MockUsageRepo)
		planRepo := new(MockPlanRepo)
		accountRepo := new(MockAccountRepo)
		calendarRepo := new(MockCalendarRepo)
		calendarAdapter := new(MockCalendarAdapter)
		aiFactory := new(MockAIFactory)
		knowledgeRepo := new(MockKnowledgeRepo)

		// Build a real AgentRunner with mocked dependencies
		agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)

		// Conversation history for multi-turn (empty for test)
		msgRepo.On("ListByConversationID", mock.Anything, mock.AnythingOfType("string"), int32(20), int32(0)).Return([]*domain.Message{}, nil)

		// Build a real ProcessMessageUseCase with mocked dependencies
		processMsg := NewProcessMessageUseCase(
			usageRepo, planRepo, agentRepo, accountRepo, convRepo, msgRepo,
			calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter,
			new(MockEmailService), "http://localhost:8080", passGuard(),
		)

		uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, processMsg, agentRepo)

		// ProcessAI calls agentRepo.GetByID first
		agentRepo.On("GetByID", mock.Anything, "agent-001").Return(&domain.Agent{
			ID:        "agent-001",
			AccountID: "acc-001",
			Name:      "Support Bot",
			Provider:  "gemini",
			Model:     "gemini-2.5-flash",
			Timezone:  "America/Sao_Paulo",
			IsActive:  true,
		}, nil)

		// ProcessMessageUseCase.Execute: check conversation bot-pause state
		convRepo.On("GetByID", mock.Anything, "conv-001", "acc-001").Return(&domain.Conversation{
			ID:          "conv-001",
			AccountID:   "acc-001",
			ChannelID:   "chan-001",
			Status:      domain.ConversationOpen,
			IsBotPaused: false,
		}, nil)
		convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-001", "acc-001").Return(nil)

		// ProcessMessageUseCase.Execute: fetch Account & Plan
		planID := "plan-free"
		accountRepo.On("GetByID", mock.Anything, "acc-001").Return(&domain.Account{
			ID:                 "acc-001",
			PlanID:             &planID,
			SubscriptionStatus: "trialing",
		}, nil)
		planRepo.On("GetByID", mock.Anything, "plan-free").Return(&domain.Plan{
			ID:                       "plan-free",
			AIProvider:               "gemini",
			AIModel:                  "gemini-2.5-flash",
			InputMultiplier:          1.0,
			OutputMultiplier:         1.0,
			MaxAITokensPerMonth:      1000000,
			MaxConversationsPerMonth: 100,
		}, nil)

		// Check token limits
		zeroTokens := int64(0)
		usageRepo.On("GetByAccountID", mock.Anything, "acc-001").Return(&domain.AccountUsage{
			ID:           "usage-001",
			AccountID:    "acc-001",
			AITokensUsed: &zeroTokens,
		}, nil)

		// AgentRunner.BuildSystemPrompt: knowledge sources (empty)
		knowledgeRepo.On("ListSources", mock.Anything, "agent-001").Return([]*domain.KnowledgeSource{}, nil)

		// AgentRunner.BuildSystemPrompt: calendars (empty, no tools)
		calendarRepo.On("ListByAgentID", mock.Anything, "agent-001").Return([]*domain.Calendar{}, nil)

		// AI provider
		aiProvider := new(MockAIProvider)
		aiFactory.On("GetProvider", mock.Anything, "gemini").Return(aiProvider, nil)

		// The AI stream response
		streamCh := make(chan string, 2)
		streamCh <- "I can help "
		streamCh <- "you with that!"
		close(streamCh)

		aiProvider.On("GenerateStream", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(domain.StreamResult{
				Chunks:   streamCh,
				GetUsage: func() (int64, int64) { return 10, 20 },
			}, nil)

		// Usage update after AI call (runs in background goroutine with context.Background())
		usageRepo.On("AddUsage", mock.Anything, "acc-001", mock.AnythingOfType("int64"), mock.AnythingOfType("int64"), mock.AnythingOfType("int64"), int32(0)).Return(nil)

		// SLA tracking: convRepo.Update called to set first_response_at
		convRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Conversation")).Return(nil)

		// The AI response message is saved by ProcessAI, stamped with the
		// provider-reported token counts (10 in / 20 out) and a positive
		// estimated cost (US-030).
		msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.Message) bool {
			return m.Direction == "outbound" &&
				m.From == "bot" &&
				m.SenderName != nil && *m.SenderName == "Support Bot" &&
				m.Content != nil && *m.Content == "I can help you with that!" &&
				m.ChannelID == "chan-001" &&
				m.ConversationID != nil && *m.ConversationID == "conv-001" &&
				m.InputTokens != nil && *m.InputTokens == 10 &&
				m.OutputTokens != nil && *m.OutputTokens == 20 &&
				m.ProviderCostUSD != nil && *m.ProviderCostUSD > 0 &&
				m.TokenEstimated != nil && !*m.TokenEstimated &&
				len(m.ID) == 26
		})).Return(nil)

		// US-030: the per-turn cost is folded into the conversation total.
		convRepo.On("IncrementCost", mock.Anything, "conv-001", "acc-001", mock.MatchedBy(func(c float64) bool {
			return c > 0
		})).Return(nil)

		partial := &InboundPartialResult{
			AccountID:      "acc-001",
			ConversationID: "conv-001",
			ChannelID:      "chan-001",
			AgentID:        strPtr("agent-001"),
			IsBotPaused:    false,
			ContactID:      "+5511999999999",
		}

		result := uc.ProcessAI(ctx, partial, "Hello, I need help")

		assert.NotNil(t, result)
		assert.NotNil(t, result.Message)
		assert.Equal(t, "outbound", result.Message.Direction)
		assert.Equal(t, "bot", result.Message.From)
		assert.NotNil(t, result.Message.Content)
		assert.Equal(t, "I can help you with that!", *result.Message.Content)
		assert.NotNil(t, result.Message.SenderName)
		assert.Equal(t, "Support Bot", *result.Message.SenderName)

		agentRepo.AssertExpectations(t)
		msgRepo.AssertExpectations(t)
		convRepo.AssertCalled(t, "IncrementCost", mock.Anything, "conv-001", "acc-001", mock.Anything)
	})

	t.Run("US-042 estimates tokens and flags token_estimated when provider omits usage", func(t *testing.T) {
		channelRepo := new(MockChannelRepo)
		convRepo := new(MockConversationRepo)
		convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		msgRepo := new(MockMessageRepo)
		agentRepo := new(MockAgentRepo)
		usageRepo := new(MockUsageRepo)
		planRepo := new(MockPlanRepo)
		accountRepo := new(MockAccountRepo)
		calendarRepo := new(MockCalendarRepo)
		calendarAdapter := new(MockCalendarAdapter)
		aiFactory := new(MockAIFactory)
		knowledgeRepo := new(MockKnowledgeRepo)

		agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)

		msgRepo.On("ListByConversationID", mock.Anything, mock.AnythingOfType("string"), int32(20), int32(0)).Return([]*domain.Message{}, nil)

		processMsg := NewProcessMessageUseCase(
			usageRepo, planRepo, agentRepo, accountRepo, convRepo, msgRepo,
			calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter,
			new(MockEmailService), "http://localhost:8080", passGuard(),
		)

		uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, processMsg, agentRepo)

		agentRepo.On("GetByID", mock.Anything, "agent-001").Return(&domain.Agent{
			ID:        "agent-001",
			AccountID: "acc-001",
			Name:      "Support Bot",
			Provider:  "gemini",
			Model:     "gemini-2.5-flash",
			Timezone:  "America/Sao_Paulo",
			IsActive:  true,
		}, nil)

		convRepo.On("GetByID", mock.Anything, "conv-001", "acc-001").Return(&domain.Conversation{
			ID:          "conv-001",
			AccountID:   "acc-001",
			ChannelID:   "chan-001",
			Status:      domain.ConversationOpen,
			IsBotPaused: false,
		}, nil)
		convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-001", "acc-001").Return(nil)

		planID := "plan-free"
		accountRepo.On("GetByID", mock.Anything, "acc-001").Return(&domain.Account{
			ID:                 "acc-001",
			PlanID:             &planID,
			SubscriptionStatus: "trialing",
		}, nil)
		planRepo.On("GetByID", mock.Anything, "plan-free").Return(&domain.Plan{
			ID:                       "plan-free",
			AIProvider:               "gemini",
			AIModel:                  "gemini-2.5-flash",
			InputMultiplier:          1.0,
			OutputMultiplier:         1.0,
			MaxAITokensPerMonth:      1000000,
			MaxConversationsPerMonth: 100,
		}, nil)

		zeroTokens := int64(0)
		usageRepo.On("GetByAccountID", mock.Anything, "acc-001").Return(&domain.AccountUsage{
			ID:           "usage-001",
			AccountID:    "acc-001",
			AITokensUsed: &zeroTokens,
		}, nil)

		knowledgeRepo.On("ListSources", mock.Anything, "agent-001").Return([]*domain.KnowledgeSource{}, nil)
		calendarRepo.On("ListByAgentID", mock.Anything, "agent-001").Return([]*domain.Calendar{}, nil)

		aiProvider := new(MockAIProvider)
		aiFactory.On("GetProvider", mock.Anything, "gemini").Return(aiProvider, nil)

		streamCh := make(chan string, 1)
		streamCh <- "Hi there!"
		close(streamCh)

		// Provider omits usage (truncated/streamed response) -> GetUsage
		// returns 0,0 so the pipeline must fall back to len(text)/4.
		aiProvider.On("GenerateStream", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(domain.StreamResult{
				Chunks:   streamCh,
				GetUsage: func() (int64, int64) { return 0, 0 },
			}, nil)

		usageRepo.On("AddUsage", mock.Anything, "acc-001", mock.AnythingOfType("int64"), mock.AnythingOfType("int64"), mock.AnythingOfType("int64"), int32(0)).Return(nil)
		convRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Conversation")).Return(nil)

		// Estimated counts: input = len("Hello, I need help")/4 = 4,
		// output = len("Hi there!")/4 = 2. token_estimated must be true.
		msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.Message) bool {
			return m.Direction == "outbound" &&
				m.From == "bot" &&
				m.Content != nil && *m.Content == "Hi there!" &&
				m.InputTokens != nil && *m.InputTokens > 0 &&
				m.OutputTokens != nil && *m.OutputTokens > 0 &&
				m.TokenEstimated != nil && *m.TokenEstimated
		})).Return(nil)

		convRepo.On("IncrementCost", mock.Anything, "conv-001", "acc-001", mock.Anything).Return(nil).Maybe()

		partial := &InboundPartialResult{
			AccountID:      "acc-001",
			ConversationID: "conv-001",
			ChannelID:      "chan-001",
			AgentID:        strPtr("agent-001"),
			IsBotPaused:    false,
			ContactID:      "+5511999999999",
		}

		result := uc.ProcessAI(ctx, partial, "Hello, I need help")

		assert.NotNil(t, result)
		assert.NotNil(t, result.Message)
		msgRepo.AssertExpectations(t)
	})

	t.Run("AI processing fails returns nil without error", func(t *testing.T) {
		channelRepo := new(MockChannelRepo)
		convRepo := new(MockConversationRepo)
		convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		msgRepo := new(MockMessageRepo)
		agentRepo := new(MockAgentRepo)
		usageRepo := new(MockUsageRepo)
		planRepo := new(MockPlanRepo)
		accountRepo := new(MockAccountRepo)
		calendarRepo := new(MockCalendarRepo)
		calendarAdapter := new(MockCalendarAdapter)
		aiFactory := new(MockAIFactory)
		knowledgeRepo := new(MockKnowledgeRepo)

		agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)

		// Conversation history for multi-turn
		msgRepo.On("ListByConversationID", mock.Anything, mock.AnythingOfType("string"), int32(20), int32(0)).Return([]*domain.Message{}, nil)

		processMsg := NewProcessMessageUseCase(
			usageRepo, planRepo, agentRepo, accountRepo, convRepo, msgRepo,
			calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter,
			new(MockEmailService), "http://localhost:8080", passGuard(),
		)

		uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, processMsg, agentRepo)

		agentRepo.On("GetByID", mock.Anything, "agent-001").Return(&domain.Agent{
			ID:        "agent-001",
			AccountID: "acc-001",
			Name:      "Support Bot",
			Provider:  "gemini",
			Model:     "gemini-2.5-flash",
			Timezone:  "America/Sao_Paulo",
			IsActive:  true,
		}, nil)

		// ProcessMessageUseCase.Execute: conversation check
		convRepo.On("GetByID", mock.Anything, "conv-001", "acc-001").Return(&domain.Conversation{
			ID:          "conv-001",
			AccountID:   "acc-001",
			ChannelID:   "chan-001",
			Status:      domain.ConversationOpen,
			IsBotPaused: false,
		}, nil)
		convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-001", "acc-001").Return(nil)

		// Account fetch fails, causing ProcessMessageUseCase.Execute to return error
		accountRepo.On("GetByID", mock.Anything, "acc-001").Return(nil, errors.New("db error"))

		partial := &InboundPartialResult{
			AccountID:      "acc-001",
			ConversationID: "conv-001",
			ChannelID:      "chan-001",
			AgentID:        strPtr("agent-001"),
			IsBotPaused:    false,
			ContactID:      "+5511999999999",
		}

		// ProcessAI logs the error and returns nil (does not propagate error)
		result := uc.ProcessAI(ctx, partial, "Hello")

		assert.Nil(t, result)
		agentRepo.AssertExpectations(t)
		msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	t.Run("US-045 sends fallback and raises human takeover on AI provider failure", func(t *testing.T) {
		channelRepo := new(MockChannelRepo)
		convRepo := new(MockConversationRepo)
		convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		msgRepo := new(MockMessageRepo)
		agentRepo := new(MockAgentRepo)
		usageRepo := new(MockUsageRepo)
		planRepo := new(MockPlanRepo)
		accountRepo := new(MockAccountRepo)
		calendarRepo := new(MockCalendarRepo)
		calendarAdapter := new(MockCalendarAdapter)
		aiFactory := new(MockAIFactory)
		knowledgeRepo := new(MockKnowledgeRepo)

		agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
		msgRepo.On("ListByConversationID", mock.Anything, mock.AnythingOfType("string"), int32(20), int32(0)).Return([]*domain.Message{}, nil)

		processMsg := NewProcessMessageUseCase(
			usageRepo, planRepo, agentRepo, accountRepo, convRepo, msgRepo,
			calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter,
			new(MockEmailService), "http://localhost:8080", passGuard(),
		)

		uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, processMsg, agentRepo)

		agentRepo.On("GetByID", mock.Anything, "agent-001").Return(&domain.Agent{
			ID:        "agent-001",
			AccountID: "acc-001",
			Name:      "Support Bot",
			Provider:  "gemini",
			Model:     "gemini-2.5-flash",
			Timezone:  "America/Sao_Paulo",
			IsActive:  true,
		}, nil)
		convRepo.On("GetByID", mock.Anything, "conv-001", "acc-001").Return(&domain.Conversation{
			ID: "conv-001", AccountID: "acc-001", ChannelID: "chan-001",
			Status: domain.ConversationOpen, IsBotPaused: false,
		}, nil)
		convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-001", "acc-001").Return(nil)
		planID := "plan-free"
		accountRepo.On("GetByID", mock.Anything, "acc-001").Return(&domain.Account{
			ID: "acc-001", PlanID: &planID, SubscriptionStatus: "trialing",
		}, nil)
		planRepo.On("GetByID", mock.Anything, "plan-free").Return(&domain.Plan{
			ID: "plan-free", AIProvider: "gemini", AIModel: "gemini-2.5-flash",
			InputMultiplier: 1.0, OutputMultiplier: 1.0,
			MaxAITokensPerMonth: 1000000, MaxConversationsPerMonth: 100,
		}, nil)
		zeroTokens := int64(0)
		usageRepo.On("GetByAccountID", mock.Anything, "acc-001").Return(&domain.AccountUsage{
			ID: "usage-001", AccountID: "acc-001", AITokensUsed: &zeroTokens,
		}, nil)
		knowledgeRepo.On("ListSources", mock.Anything, "agent-001").Return([]*domain.KnowledgeSource{}, nil)
		calendarRepo.On("ListByAgentID", mock.Anything, "agent-001").Return([]*domain.Calendar{}, nil)

		// The AI provider's streaming call fails outright — the ghosting bug.
		aiProvider := new(MockAIProvider)
		aiFactory.On("GetProvider", mock.Anything, "gemini").Return(aiProvider, nil)
		aiProvider.On("GenerateStream", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(domain.StreamResult{}, errors.New("provider 503"))

		// US-045: human-takeover flag is raised (bot paused).
		convRepo.On("SetBotPaused", mock.Anything, "conv-001", "acc-001", true).Return(nil)
		// US-045: the configurable fallback is saved + sent to the channel.
		msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.Message) bool {
			return m.Direction == "outbound" && m.From == "bot" &&
				m.ConversationID != nil && *m.ConversationID == "conv-001" &&
				m.Content != nil && *m.Content == services.AIFailureFallbackMessage
		})).Return(nil)

		partial := &InboundPartialResult{
			AccountID: "acc-001", ConversationID: "conv-001", ChannelID: "chan-001",
			AgentID: strPtr("agent-001"), IsBotPaused: false, ContactID: "+5511999999999",
		}

		result := uc.ProcessAI(ctx, partial, "Hello")

		assert.NotNil(t, result)
		assert.NotNil(t, result.Message)
		assert.False(t, result.LimitReached, "fallback must be sent to the channel, not held back")
		assert.Equal(t, services.AIFailureFallbackMessage, *result.Message.Content)
		convRepo.AssertCalled(t, "SetBotPaused", mock.Anything, "conv-001", "acc-001", true)
		msgRepo.AssertExpectations(t)
	})
}

func TestSaveInbound_MarksFirstInboundWhenAccountRepoSet(t *testing.T) {
	ctx := context.Background()

	channel := &domain.Channel{
		AccountID: "acc-onb",
		ID:        "chan-onb",
		AgentID:   strPtr("agent-onb"),
		Type:      "whatsapp",
		IsActive:  true,
	}
	dto := InboundMessageDTO{
		ChannelToken: "tok",
		SenderPhone:  "+5511",
		MessageType:  "text",
		Content:      "first ever",
	}

	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)
	accountRepo := new(MockAccountRepo)

	uc := NewInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)
	uc.SetAccountRepo(accountRepo)

	convRepo.On("FindByContact", ctx, "chan-onb", "+5511").Return(nil, nil)
	convRepo.On("Create", ctx, mock.Anything).Return(nil)
	msgRepo.On("Create", ctx, mock.Anything).Return(nil)
	// Idempotent at the SQL level — UC always calls; repo is the no-op
	// gate. We just assert the call happens.
	accountRepo.On("MarkFirstInboundReceived", ctx, "acc-onb", mock.MatchedBy(func(at time.Time) bool {
		return !at.IsZero() && time.Since(at) < time.Second
	})).Return(nil)

	_, err := uc.SaveInbound(ctx, channel, dto)
	assert.NoError(t, err)
	accountRepo.AssertExpectations(t)
}

func TestSaveInbound_DoesNotCallAccountRepoWhenUnset(t *testing.T) {
	ctx := context.Background()

	channel := &domain.Channel{
		AccountID: "acc-x",
		ID:        "chan-x",
		AgentID:   strPtr("agent-x"),
		Type:      "whatsapp",
		IsActive:  true,
	}
	dto := InboundMessageDTO{ChannelToken: "tok", SenderPhone: "+5511", MessageType: "text", Content: "hi"}

	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)
	accountRepo := new(MockAccountRepo)

	uc := NewInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)
	// no SetAccountRepo — onboarding hook stays dormant.

	convRepo.On("FindByContact", ctx, "chan-x", "+5511").Return(nil, nil)
	convRepo.On("Create", ctx, mock.Anything).Return(nil)
	msgRepo.On("Create", ctx, mock.Anything).Return(nil)

	_, err := uc.SaveInbound(ctx, channel, dto)
	assert.NoError(t, err)
	accountRepo.AssertNotCalled(t, "MarkFirstInboundReceived", mock.Anything, mock.Anything, mock.Anything)
}

// stubOptOutSender records the opt-out confirmation send for assertions.
type stubOptOutSender struct {
	called    bool
	to        string
	channelID string
	body      string
}

func (s *stubOptOutSender) SendMessage(_ context.Context, channel *domain.Channel, to string, msg *domain.Message) error {
	s.called = true
	s.to = to
	s.channelID = channel.ID
	if msg.Content != nil {
		s.body = *msg.Content
	}
	return nil
}

// TestSaveInbound_OptOutKeyword locks in US-025: an inbound opt-out keyword
// flags the contact, sends the short confirmation reply, pauses the bot, and
// moves the contact's lead to status=lost (so it drops from batch-approach
// lists) — without advancing it to responded.
func TestSaveInbound_OptOutKeyword(t *testing.T) {
	ctx := context.Background()

	channel := &domain.Channel{
		AccountID: "acc-oo",
		ID:        "chan-oo",
		AgentID:   strPtr("agent-oo"),
		Type:      "whatsapp",
		IsActive:  true,
	}
	dto := InboundMessageDTO{ChannelToken: "tok", SenderPhone: "+5511", MessageType: "text", Content: "PARAR"}

	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("FindByContact", ctx, "chan-oo", "+5511").Return(nil, nil)
	convRepo.On("Create", ctx, mock.Anything).Return(nil)
	convRepo.On("SetBotPaused", ctx, mock.Anything, "acc-oo", true).Return(nil)
	msgRepo := new(MockMessageRepo)
	msgRepo.On("Create", ctx, mock.Anything).Return(nil)
	agentRepo := new(MockAgentRepo)

	contactRepo := new(MockContactRepo)
	contactRepo.On("SetOptedOut", ctx, "acc-oo", "+5511").Return(true, nil)

	funnelRepo := new(MockSalesFunnelRepo)
	funnelRepo.On("GetByChannelID", ctx, "chan-oo").Return(&domain.SalesFunnel{ID: "fun-1"}, nil)
	leadRepo := new(mockLeadRepo)
	leadRepo.On("GetByFunnelAndContact", ctx, "fun-1", "+5511").
		Return(&domain.Lead{ID: "lead-1", FunnelID: "fun-1", ContactID: "+5511", Status: domain.LeadStatusContacted}, nil)
	leadRepo.On("UpdateStatus", ctx, "lead-1", domain.LeadStatusLost).Return(nil)

	sender := &stubOptOutSender{}

	uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)
	uc.SetContactRepo(contactRepo)
	uc.SetSalesLead(funnelRepo, leadRepo)
	uc.SetOptOutSender(sender)

	result, err := uc.SaveInbound(ctx, channel, dto)
	assert.NoError(t, err)
	assert.True(t, result.IsBotPaused, "bot should be paused after opt-out")

	contactRepo.AssertCalled(t, "SetOptedOut", ctx, "acc-oo", "+5511")
	convRepo.AssertCalled(t, "SetBotPaused", ctx, mock.Anything, "acc-oo", true)
	leadRepo.AssertCalled(t, "UpdateStatus", ctx, "lead-1", domain.LeadStatusLost)
	// Must NOT advance an opt-out lead to responded.
	leadRepo.AssertNotCalled(t, "UpdateStatus", ctx, "lead-1", domain.LeadStatusResponded)

	assert.True(t, sender.called, "opt-out confirmation should be sent")
	assert.Equal(t, "+5511", sender.to)
	assert.Equal(t, "chan-oo", sender.channelID)
	assert.NotEmpty(t, sender.body)
}

// TestSaveInbound_DuplicateExternalIDRejected locks in US-016: a WhatsApp
// webhook redelivery (Meta retries the same wamid when our handler is
// slow or returns non-2xx) must NOT create a second conversation message.
// SaveInbound short-circuits with ErrDuplicateInbound the moment it sees
// the (channel_id, external_message_id) already persisted — and the
// messages unique index + ON CONFLICT DO NOTHING is the race-safe backstop
// behind it. Asserts the conversation/message writes never fire.
func TestSaveInbound_DuplicateExternalIDRejected(t *testing.T) {
	ctx := context.Background()

	channel := &domain.Channel{
		AccountID: "acc-1",
		ID:        "chan-1",
		AgentID:   strPtr("agent-1"),
		Type:      "whatsapp",
		IsActive:  true,
	}
	dto := InboundMessageDTO{
		ChannelToken:      "tok",
		SenderPhone:       "+5511999999999",
		MessageType:       "text",
		Content:           "hello (retry)",
		ExternalMessageID: "wamid.DUPLICATE123",
	}

	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	// The same wamid was already persisted by an earlier delivery.
	msgRepo.On("GetByExternalID", ctx, "chan-1", "wamid.DUPLICATE123").
		Return(&domain.Message{ID: "msg-existing"}, nil)

	uc := NewInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)

	result, err := uc.SaveInbound(ctx, channel, dto)

	assert.Nil(t, result)
	assert.Equal(t, ErrDuplicateInbound, err)
	// No second conversation, no second message, no double counter bump.
	convRepo.AssertNotCalled(t, "FindByContact", mock.Anything, mock.Anything, mock.Anything)
	convRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// TestSaveInbound_DuplicateInsertBackstop locks in US-047: the (channel_id,
// external_message_id) unique key on insert is the race-safe backstop when
// two Meta retries land within milliseconds and BOTH miss the read-then-check
// dedupe (GetByExternalID returns nil for the loser). The loser's
// CreateMessage hits ON CONFLICT DO NOTHING and the repo reports inserted=false
// — SaveInbound must drop it with ErrDuplicateInbound so only ONE message is
// persisted and no downstream side effects (AI, billing, websocket) fire twice.
func TestSaveInbound_DuplicateInsertBackstop(t *testing.T) {
	ctx := context.Background()

	channel := &domain.Channel{
		AccountID: "acc-1",
		ID:        "chan-1",
		AgentID:   strPtr("agent-1"),
		Type:      "whatsapp",
		IsActive:  true,
	}
	dto := InboundMessageDTO{
		ChannelToken:      "tok",
		SenderPhone:       "+5511999999999",
		MessageType:       "text",
		Content:           "hello (concurrent retry)",
		ExternalMessageID: "wamid.RACE456",
	}

	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)

	// Read-then-check dedupe MISSES — the winning retry hasn't committed yet
	// when the loser does its GetByExternalID lookup.
	msgRepo.On("GetByExternalID", ctx, "chan-1", "wamid.RACE456").Return(nil, nil)

	existingConv := &domain.Conversation{
		ID:        "conv-1",
		AccountID: "acc-1",
		ChannelID: "chan-1",
		ContactID: "+5511999999999",
		Status:    domain.ConversationOpen,
	}
	convRepo.On("FindByContact", ctx, "chan-1", "+5511999999999").Return(existingConv, nil)
	convRepo.On("UpdateLastCustomerMessageAt", ctx, "conv-1", "acc-1").Return(nil)

	// The unique index rejects the loser's INSERT → ON CONFLICT DO NOTHING →
	// 0 rows affected → repo returns inserted=false.
	msgRepo.On("Create", ctx, mock.AnythingOfType("*domain.Message")).Return(false, nil)

	uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)

	result, err := uc.SaveInbound(ctx, channel, dto)

	assert.Nil(t, result)
	assert.Equal(t, ErrDuplicateInbound, err)
	msgRepo.AssertExpectations(t)
}

// TestSaveInbound_ConversationUsageMeteredForAllChannelTypes locks in
// US-014: the conversations_count bump on new-conversation creation is
// channel-type-agnostic — an inbound via instagram or sms meters exactly
// like whatsapp/telegram, and an existing conversation never re-bumps.
func TestSaveInbound_ConversationUsageMeteredForAllChannelTypes(t *testing.T) {
	ctx := context.Background()

	dto := InboundMessageDTO{
		ChannelToken: "tok",
		SenderPhone:  "+15550001111",
		MessageType:  "text",
		Content:      "hi there",
	}

	for _, chType := range []string{"instagram", "sms", "whatsapp", "telegram"} {
		t.Run(chType+": new conversation bumps conversations_count", func(t *testing.T) {
			channelRepo := new(MockChannelRepo)
			convRepo := new(MockConversationRepo)
			convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
			convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
			msgRepo := new(MockMessageRepo)
			usageRepo := new(MockUsageRepo)

			channel := &domain.Channel{
				AccountID: "acc-1",
				ID:        "chan-" + chType,
				Type:      chType,
				IsActive:  true,
			}

			convRepo.On("FindByContact", ctx, channel.ID, "+15550001111").Return(nil, nil)
			convRepo.On("Create", ctx, mock.AnythingOfType("*domain.Conversation")).Return(nil)
			msgRepo.On("Create", ctx, mock.AnythingOfType("*domain.Message")).Return(true, nil)
			usageRepo.On("AddUsage", ctx, "acc-1", int64(0), int64(0), int64(0), int32(1)).Return(nil).Once()

			uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, new(MockAgentRepo))
			uc.SetUsageRepo(usageRepo)

			result, err := uc.SaveInbound(ctx, channel, dto)

			assert.NoError(t, err)
			assert.NotNil(t, result)
			usageRepo.AssertExpectations(t)
		})

		t.Run(chType+": existing conversation does not re-bump", func(t *testing.T) {
			channelRepo := new(MockChannelRepo)
			convRepo := new(MockConversationRepo)
			convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
			convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
			msgRepo := new(MockMessageRepo)
			usageRepo := new(MockUsageRepo)

			channel := &domain.Channel{
				AccountID: "acc-1",
				ID:        "chan-" + chType,
				Type:      chType,
				IsActive:  true,
			}
			existingConv := &domain.Conversation{
				ID:        "conv-1",
				AccountID: "acc-1",
				ChannelID: channel.ID,
				ContactID: "+15550001111",
				Status:    domain.ConversationOpen,
			}

			convRepo.On("FindByContact", ctx, channel.ID, "+15550001111").Return(existingConv, nil)
			convRepo.On("UpdateLastCustomerMessageAt", ctx, "conv-1", "acc-1").Return(nil)
			msgRepo.On("Create", ctx, mock.AnythingOfType("*domain.Message")).Return(true, nil)

			uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, new(MockAgentRepo))
			uc.SetUsageRepo(usageRepo)

			_, err := uc.SaveInbound(ctx, channel, dto)

			assert.NoError(t, err)
			usageRepo.AssertNotCalled(t, "AddUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// TestProcessAI_TakeoverCancelsInflightReply locks in US-044: when a human
// assumes a conversation while the bot reply is already streaming, the
// shared InflightAIRegistry cancel must abort the generation so NO bot
// message is persisted or dispatched.
//
// The stream channel is intentionally left open (never closed) so the only
// way ProcessAI can return is via context cancellation — proving the cancel
// path, not a natural end-of-stream, is what stops the reply.
func TestProcessAI_TakeoverCancelsInflightReply(t *testing.T) {
	ctx := context.Background()

	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	agentRepo := new(MockAgentRepo)
	usageRepo := new(MockUsageRepo)
	planRepo := new(MockPlanRepo)
	accountRepo := new(MockAccountRepo)
	calendarRepo := new(MockCalendarRepo)
	calendarAdapter := new(MockCalendarAdapter)
	aiFactory := new(MockAIFactory)
	knowledgeRepo := new(MockKnowledgeRepo)

	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	msgRepo.On("ListByConversationID", mock.Anything, mock.AnythingOfType("string"), int32(20), int32(0)).Return([]*domain.Message{}, nil)

	processMsg := NewProcessMessageUseCase(
		usageRepo, planRepo, agentRepo, accountRepo, convRepo, msgRepo,
		calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter,
		new(MockEmailService), "http://localhost:8080", passGuard(),
	)

	uc := newInboundWebhookUseCase(channelRepo, convRepo, msgRepo, processMsg, agentRepo)

	// Shared registry — the same instance the takeover use case would hold.
	registry := services.NewInflightAIRegistry()
	uc.SetInflightRegistry(registry)

	agentRepo.On("GetByID", mock.Anything, "agent-001").Return(&domain.Agent{
		ID: "agent-001", AccountID: "acc-001", Name: "Support Bot",
		Provider: "gemini", Model: "gemini-2.5-flash",
		Timezone: "America/Sao_Paulo", IsActive: true,
	}, nil)
	convRepo.On("GetByID", mock.Anything, "conv-001", "acc-001").Return(&domain.Conversation{
		ID: "conv-001", AccountID: "acc-001", ChannelID: "chan-001",
		Status: domain.ConversationOpen, IsBotPaused: false,
	}, nil)
	convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-001", "acc-001").Return(nil)

	planID := "plan-free"
	accountRepo.On("GetByID", mock.Anything, "acc-001").Return(&domain.Account{
		ID: "acc-001", PlanID: &planID, SubscriptionStatus: "trialing",
	}, nil)
	planRepo.On("GetByID", mock.Anything, "plan-free").Return(&domain.Plan{
		ID: "plan-free", AIProvider: "gemini", AIModel: "gemini-2.5-flash",
		InputMultiplier: 1.0, OutputMultiplier: 1.0,
		MaxAITokensPerMonth: 1000000, MaxConversationsPerMonth: 100,
	}, nil)
	zeroTokens := int64(0)
	usageRepo.On("GetByAccountID", mock.Anything, "acc-001").Return(&domain.AccountUsage{
		ID: "usage-001", AccountID: "acc-001", AITokensUsed: &zeroTokens,
	}, nil)
	usageRepo.On("AddUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	knowledgeRepo.On("ListSources", mock.Anything, "agent-001").Return([]*domain.KnowledgeSource{}, nil)
	calendarRepo.On("ListByAgentID", mock.Anything, "agent-001").Return([]*domain.Calendar{}, nil)

	aiProvider := new(MockAIProvider)
	aiFactory.On("GetProvider", mock.Anything, "gemini").Return(aiProvider, nil)

	// Unbuffered stream, never closed: the goroutine sends one chunk, then —
	// the moment that send is consumed by the pipeline (proving we're
	// mid-stream) — fires the takeover cancel. Without the cancel, ProcessAI
	// would block here forever.
	streamCh := make(chan string)
	go func() {
		streamCh <- "Olá, posso "
		registry.Cancel("conv-001")
	}()
	aiProvider.On("GenerateStream", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(domain.StreamResult{
			Chunks:   streamCh,
			GetUsage: func() (int64, int64) { return 0, 0 },
		}, nil)

	partial := &InboundPartialResult{
		AccountID: "acc-001", ConversationID: "conv-001", ChannelID: "chan-001",
		AgentID: strPtr("agent-001"), IsBotPaused: false, ContactID: "+5511999999999",
	}

	done := make(chan *ProcessAIResult, 1)
	go func() { done <- uc.ProcessAI(ctx, partial, "Quero ajuda") }()

	select {
	case result := <-done:
		// Canceled mid-stream → no reply object returned.
		assert.Nil(t, result, "canceled generation must not return a reply")
	case <-time.After(5 * time.Second):
		t.Fatal("ProcessAI did not return after takeover cancel — reply was not aborted")
	}

	// The crux of US-044: no bot message persisted, so nothing gets sent to
	// the customer on a conversation a human just took over.
	msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	// And the cancel is consumed — a second Cancel is a no-op.
	assert.False(t, registry.Cancel("conv-001"), "cancel should be idempotent / entry removed")
}

// The identity index was empty for the product's main channel.
//
// Meta writes a WhatsApp sender as bare digits. `resolveContact` handed that
// value straight to the identity write, where it failed the E.164 check — and
// because identity registration is best-effort, the failure was a log line and
// nothing else. So "this email and this WhatsApp are the same person", the one
// question the index exists to answer, had no WhatsApp rows to answer it with.
//
// The same value has to be used for the lookup and the write, or the lookup
// misses the row it just wrote.
func TestSaveInbound_RegistersTheWhatsAppSenderAsAnIdentity(t *testing.T) {
	ctx := context.Background()

	channel := &domain.Channel{
		AccountID: "acc-id", ID: "chan-id", Type: "whatsapp", IsActive: true,
	}
	dto := InboundMessageDTO{
		ChannelToken: "tok",
		// Exactly what Meta sends: digits, no plus.
		SenderPhone: "5511999998888",
		SenderName:  "Marina",
		MessageType: "text",
		Content:     "oi",
	}

	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("FindByContact", ctx, "chan-id", "5511999998888").Return(nil, nil)
	convRepo.On("Create", ctx, mock.Anything).Return(nil)
	msgRepo := new(MockMessageRepo)
	msgRepo.On("Create", ctx, mock.Anything).Return(nil)
	agentRepo := new(MockAgentRepo)

	contacts := newFakeContactStore()
	identities := newFakeIdentityStore()

	uc := NewInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)
	uc.SetContactRepo(contacts)
	uc.SetIdentityRepo(identities)

	_, err := uc.SaveInbound(ctx, channel, dto)
	require.NoError(t, err)

	require.Len(t, identities.written, 1, "the WhatsApp sender was never indexed")
	assert.Equal(t, domain.IdentityKindPhone, identities.written[0].Kind)
	assert.Equal(t, "+5511999998888", identities.written[0].Value)
	assert.NotNil(t, identities.written[0].VerifiedAt,
		"an inbound message IS proof the number reaches them")

	// The contact keeps the channel's key, because that is what the next
	// message and every conversation lookup arrive on...
	require.Len(t, contacts.upserts, 1)
	assert.Equal(t, "5511999998888", contacts.upserts[0].ExternalID)
	// ...and now also carries the phone, which is the column the dashboard and
	// every outbound confirmation read. It used to be nil for every WhatsApp
	// contact, because NormalizeE164 refuses bare digits.
	require.NotNil(t, contacts.upserts[0].Phone)
	assert.Equal(t, "+5511999998888", *contacts.upserts[0].Phone)
}

// And the second message resolves to the contact the first one created, rather
// than upserting a stranger.
func TestSaveInbound_TheSecondMessageResolvesToTheSameContact(t *testing.T) {
	ctx := context.Background()

	channel := &domain.Channel{
		AccountID: "acc-id2", ID: "chan-id2", Type: "whatsapp", IsActive: true,
	}
	dto := InboundMessageDTO{
		ChannelToken: "tok", SenderPhone: "5511999997777",
		MessageType: "text", Content: "oi",
	}

	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("FindByContact", ctx, "chan-id2", "5511999997777").Return(nil, nil)
	convRepo.On("Create", ctx, mock.Anything).Return(nil)
	msgRepo := new(MockMessageRepo)
	msgRepo.On("Create", ctx, mock.Anything).Return(nil)
	agentRepo := new(MockAgentRepo)

	contacts := newFakeContactStore()
	identities := newFakeIdentityStore()

	uc := NewInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, agentRepo)
	uc.SetContactRepo(contacts)
	uc.SetIdentityRepo(identities)

	_, err := uc.SaveInbound(ctx, channel, dto)
	require.NoError(t, err)
	_, err = uc.SaveInbound(ctx, channel, dto)
	require.NoError(t, err)

	assert.Len(t, contacts.upserts, 1,
		"the second message created a second contact — the lookup did not find the first")
}

// ── Which funnel a reply belongs to ──────────────────────────────────────

// In production every default funnel has channel_id NULL, so keyed on the
// channel alone the lookup came back nil for every account and no reply ever
// marked a lead as responded — silently. The default funnel is where imports
// and finds land when nobody chose one, so it is where the reply goes too.
func TestMarkLeadResponded_FallsBackToTheDefaultFunnel(t *testing.T) {
	ctx := context.Background()
	channel := &domain.Channel{ID: "chan-mail", AccountID: "acc-mail", Type: "email"}

	funnelRepo := new(MockSalesFunnelRepo)
	funnelRepo.On("GetByChannelID", ctx, "chan-mail").Return(nil, nil)
	funnelRepo.On("EnsureDefault", ctx, "acc-mail").Return(&domain.SalesFunnel{ID: "fun-default"}, nil)
	leadRepo := new(mockLeadRepo)
	leadRepo.On("GetByFunnelAndContact", ctx, "fun-default", "contact-1").
		Return(&domain.Lead{ID: "lead-1", FunnelID: "fun-default", ContactID: "contact-1", Status: domain.LeadStatusContacted}, nil)
	leadRepo.On("UpdateStatus", ctx, "lead-1", domain.LeadStatusResponded).Return(nil)

	uc := newInboundWebhookUseCase(new(MockChannelRepo), new(MockConversationRepo), new(MockMessageRepo), nil, new(MockAgentRepo))
	uc.SetSalesLead(funnelRepo, leadRepo)

	uc.markLeadResponded(ctx, channel, &domain.Contact{ID: "contact-1"}, "dono@clinica.com.br", InboundMessageDTO{})

	leadRepo.AssertCalled(t, "UpdateStatus", ctx, "lead-1", domain.LeadStatusResponded)
	funnelRepo.AssertCalled(t, "EnsureDefault", ctx, "acc-mail")
}

// A channel-bound funnel still wins: the fallback is for the missing case,
// not a second opinion.
func TestMarkLeadResponded_AChannelBoundFunnelWinsOverTheDefault(t *testing.T) {
	ctx := context.Background()
	channel := &domain.Channel{ID: "chan-wa", AccountID: "acc-wa", Type: "whatsapp"}

	funnelRepo := new(MockSalesFunnelRepo)
	funnelRepo.On("GetByChannelID", ctx, "chan-wa").Return(&domain.SalesFunnel{ID: "fun-bound"}, nil)
	leadRepo := new(mockLeadRepo)
	leadRepo.On("GetByFunnelAndContact", ctx, "fun-bound", "contact-2").
		Return(&domain.Lead{ID: "lead-2", FunnelID: "fun-bound", ContactID: "contact-2", Status: domain.LeadStatusContacted}, nil)
	leadRepo.On("UpdateStatus", ctx, "lead-2", domain.LeadStatusResponded).Return(nil)

	uc := newInboundWebhookUseCase(new(MockChannelRepo), new(MockConversationRepo), new(MockMessageRepo), nil, new(MockAgentRepo))
	uc.SetSalesLead(funnelRepo, leadRepo)

	uc.markLeadResponded(ctx, channel, &domain.Contact{ID: "contact-2"}, "+5511", InboundMessageDTO{})

	leadRepo.AssertCalled(t, "UpdateStatus", ctx, "lead-2", domain.LeadStatusResponded)
	funnelRepo.AssertNotCalled(t, "EnsureDefault", mock.Anything, mock.Anything)
}

// ── One customer, one key ────────────────────────────────────────────────

// "Sair" by e-mail from a person the base knows by phone: the flag goes on
// THAT contact. Keyed on the raw sender it updated zero rows.
func TestSaveInbound_OptOutFlagsTheResolvedContactNotTheRawSender(t *testing.T) {
	ctx := context.Background()
	channel := &domain.Channel{AccountID: "acc-e", ID: "chan-e", Type: "email", IsActive: true}
	dto := InboundMessageDTO{ChannelToken: "tok", SenderPhone: "maria@clinica.br", MessageType: "text", Content: "SAIR"}

	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("FindByContact", ctx, "chan-e", "maria@clinica.br").Return(nil, nil)
	convRepo.On("Create", ctx, mock.Anything).Return(nil)
	convRepo.On("SetBotPaused", ctx, mock.Anything, "acc-e", true).Return(nil)
	msgRepo := new(MockMessageRepo)
	msgRepo.On("Create", ctx, mock.Anything).Return(nil)

	contacts, identities := newFakeContactStore(), newFakeIdentityStore()
	phone := "+5565999990000"
	_, _ = contacts.Upsert(ctx, &domain.Contact{ID: "ct-1", AccountID: "acc-e", ExternalID: "5565999990000", Phone: &phone})
	_, _ = identities.Upsert(ctx, &domain.ContactIdentity{
		AccountID: "acc-e", ContactID: "ct-1", Kind: domain.IdentityKindEmail, Value: "maria@clinica.br"})

	uc := NewInboundWebhookUseCase(new(MockChannelRepo), convRepo, msgRepo, nil, new(MockAgentRepo))
	uc.SetContactRepo(contacts)
	uc.SetIdentityRepo(identities)

	result, err := uc.SaveInbound(ctx, channel, dto)
	require.NoError(t, err)
	assert.True(t, result.IsBotPaused)
	assert.Equal(t, []string{"5565999990000"}, contacts.optedOut,
		"the flag must land on the resolved contact's own key")
	assert.Len(t, contacts.upserts, 1, "no stranger minted for the e-mail")
}

// We wrote to contato@hospital; the purchasing manager answered from his own
// address. The threading headers name the message he answered, so the reply
// belongs to that thread and that contact — not to a new conversation with a
// new stranger while the lead we worked stays "contacted" forever.
func TestSaveInbound_AReplyFromAnotherAddressLandsInItsThread(t *testing.T) {
	ctx := context.Background()
	channel := &domain.Channel{AccountID: "acc-e", ID: "chan-e", Type: "email", IsActive: true}
	dto := InboundMessageDTO{
		ChannelToken: "tok", SenderPhone: "joao@hospital.br", SenderName: "João",
		MessageType: "text", Content: "Podemos conversar amanhã?",
		Metadata: []byte(`{"subject":"Re: proposta","in_reply_to":"<sent-1@replider>","references":"<older@replider> <sent-1@replider>"}`),
	}

	convRepo := new(MockConversationRepo)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, "conv-1", "acc-e").Return(nil).Maybe()
	convRepo.On("GetByID", ctx, "conv-1", "acc-e").Return(&domain.Conversation{
		ID: "conv-1", AccountID: "acc-e", ChannelID: "chan-e",
		ContactID: "contato@hospital.br", ContactRefID: strPtr("ct-1"),
		Status: domain.ConversationOpen,
	}, nil)
	msgRepo := new(MockMessageRepo)
	msgRepo.On("GetByExternalID", ctx, "chan-e", "<sent-1@replider>").
		Return(&domain.Message{ID: "m-1", ChannelID: "chan-e", ConversationID: strPtr("conv-1"), Direction: "outbound"}, nil)
	msgRepo.On("Create", ctx, mock.Anything).Return(nil)

	contacts, identities := newFakeContactStore(), newFakeIdentityStore()
	_, _ = contacts.Upsert(ctx, &domain.Contact{ID: "ct-1", AccountID: "acc-e", ExternalID: "contato@hospital.br"})

	funnelRepo := new(MockSalesFunnelRepo)
	funnelRepo.On("GetByChannelID", mock.Anything, "chan-e").Return(&domain.SalesFunnel{ID: "fun-1", AccountID: "acc-e"}, nil).Maybe()
	funnelRepo.On("ListByAccountID", mock.Anything, "acc-e").Return([]*domain.SalesFunnel{{ID: "fun-1", AccountID: "acc-e", IsDefault: true}}, nil).Maybe()
	leadRepo := new(mockLeadRepo)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fun-1", "ct-1").
		Return(&domain.Lead{ID: "lead-1", FunnelID: "fun-1", ContactID: "ct-1", Status: domain.LeadStatusContacted}, nil)
	leadRepo.On("UpdateStatus", mock.Anything, "lead-1", domain.LeadStatusResponded).Return(nil)

	uc := NewInboundWebhookUseCase(new(MockChannelRepo), convRepo, msgRepo, nil, new(MockAgentRepo))
	uc.SetContactRepo(contacts)
	uc.SetIdentityRepo(identities)
	uc.SetSalesLead(funnelRepo, leadRepo)

	result, err := uc.SaveInbound(ctx, channel, dto)
	require.NoError(t, err)

	assert.Equal(t, "conv-1", result.ConversationID, "the reply did not land in the thread it answers")
	assert.False(t, result.IsNewConversation)
	convRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	convRepo.AssertNotCalled(t, "FindByContact", mock.Anything, mock.Anything, mock.Anything)
	assert.Len(t, contacts.upserts, 1, "a second contact was minted for the colleague's address")
	// And the lead we actually worked is the one that responded.
	leadRepo.AssertCalled(t, "UpdateStatus", mock.Anything, "lead-1", domain.LeadStatusResponded)
}

func TestThreadMessageIDs_ParentFirstThenReferencesNewestFirst(t *testing.T) {
	got := threadMessageIDs("<c@x>", "<a@x> <b@x> <c@x>")
	assert.Equal(t, []string{"<c@x>", "<b@x>", "<a@x>"}, got)
	assert.Empty(t, threadMessageIDs("", ""))
}
