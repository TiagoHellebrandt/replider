package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPreviewChatUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should successfully preview chat when tokens are available", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		agentRepo := new(MockAgentRepo)
		usageRepo := new(MockUsageRepo)
		planRepo := new(MockPlanRepo)
		auditLogRepo := new(MockAuditLogRepo)
		aiFactory := new(MockAIFactory)
		knowledgeRepo := new(MockKnowledgeRepo)
		calendarRepo := new(MockCalendarRepo)

		agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
		uc := NewPreviewChatUseCase(accountRepo, agentRepo, usageRepo, planRepo, auditLogRepo, aiFactory, agentRunner)

		planID := "plan_premium"
		account := &domain.Account{ID: "acc_123", PlanID: &planID}
		plan := &domain.Plan{
			ID:                  planID,
			MaxAITokensPerMonth: 1000,
			AIProvider:          "gemini",
			InputMultiplier:     1.0,
			OutputMultiplier:    1.0,
		}
		tokensUsed := int64(500)
		usage := &domain.AccountUsage{ID: "usg_123", AITokensUsed: &tokensUsed, AccountID: "acc_123"}
		agent := &domain.Agent{ID: "agt_123", AccountID: "acc_123"}

		agentRepo.On("GetByID", ctx, "agt_123").Return(agent, nil)
		accountRepo.On("GetByID", ctx, "acc_123").Return(account, nil)
		planRepo.On("GetByID", ctx, planID).Return(plan, nil)
		usageRepo.On("GetByAccountID", ctx, "acc_123").Return(usage, nil)
		knowledgeRepo.On("ListSources", ctx, "agt_123").Return([]*domain.KnowledgeSource{}, nil)
		calendarRepo.On("ListByAgentID", ctx, "agt_123").Return([]*domain.Calendar{}, nil)

		// Mock AI Provider
		mockPrv := new(MockAIProvider)
		aiFactory.On("GetProvider", ctx, "gemini").Return(mockPrv, nil)

		aiResponse := &domain.AICallbackResponse{
			Content: "Real AI Response",
			Usage: domain.AIUsage{
				PromptTokens:     60,
				CompletionTokens: 40,
			},
		}
		mockPrv.On("GenerateResponse", ctx, mock.Anything, mock.Anything, mock.Anything).Return(aiResponse, nil)

		// Expect usage to be updated with +100 tokens (60+40)
		usageRepo.On("Update", ctx, mock.MatchedBy(func(u *domain.AccountUsage) bool {
			return *u.AITokensUsed == 600
		})).Return(nil)

		// Expect audit log to be created
		auditLogRepo.On("Create", ctx, mock.MatchedBy(func(log *domain.AuditLog) bool {
			return log.Action == "agent.preview_chat" && log.AccountID == "acc_123" && log.EntityID == "agt_123"
		})).Return(nil)

		// Onboarding milestone — best-effort, idempotent at the SQL layer.
		accountRepo.On("MarkPreviewChatTested", ctx, "acc_123").Return(nil)

		resp, err := uc.Execute(ctx, PreviewChatDTO{
			AccountID: "acc_123",
			AgentID:   "agt_123",
			Message:   "Hello AI",
		})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "Real AI Response", resp.Response)
		assert.Equal(t, int64(100), resp.TokensUsed)
		assert.Equal(t, int64(400), resp.TokensRemain) // 1000 - 600

		agentRepo.AssertExpectations(t)
		accountRepo.AssertExpectations(t)
		planRepo.AssertExpectations(t)
		usageRepo.AssertExpectations(t)
		auditLogRepo.AssertExpectations(t)
	})

	t.Run("should fail if tokens are exceeded", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		agentRepo := new(MockAgentRepo)
		usageRepo := new(MockUsageRepo)
		planRepo := new(MockPlanRepo)
		auditLogRepo := new(MockAuditLogRepo)
		aiFactory := new(MockAIFactory)
		knowledgeRepo := new(MockKnowledgeRepo)
		calendarRepo := new(MockCalendarRepo)
		guard := new(MockUsageChecker)

		agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
		uc := NewPreviewChatUseCase(accountRepo, agentRepo, usageRepo, planRepo, auditLogRepo, aiFactory, agentRunner)
		uc.SetUsageGuard(guard)

		planID := "plan_basic"
		account := &domain.Account{ID: "acc_123", PlanID: &planID}
		plan := &domain.Plan{ID: planID, MaxAITokensPerMonth: 100}
		tokensUsed := int64(100) // Limit reached
		usage := &domain.AccountUsage{ID: "usg_123", AITokensUsed: &tokensUsed, AccountID: "acc_123"}
		agent := &domain.Agent{ID: "agt_123", AccountID: "acc_123"}

		agentRepo.On("GetByID", ctx, "agt_123").Return(agent, nil)
		accountRepo.On("GetByID", ctx, "acc_123").Return(account, nil)
		planRepo.On("GetByID", ctx, planID).Return(plan, nil)
		usageRepo.On("GetByAccountID", ctx, "acc_123").Return(usage, nil)
		guard.On("Check", plan, usage, "tokens").Return(domain.ErrUsageLimitExceeded)

		resp, err := uc.Execute(ctx, PreviewChatDTO{
			AccountID: "acc_123",
			AgentID:   "agt_123",
			Message:   "Hello AI",
		})

		assert.Error(t, err)
		assert.Equal(t, domain.ErrUsageLimitExceeded, err)
		assert.Nil(t, resp)

		// Verify structured error code
		assert.Equal(t, "USAGE_LIMIT_EXCEEDED", err.(*domain.AppError).Code)

		// Ensure Update and Create are NOT called when it fails
		usageRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
		auditLogRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})
}

func TestPreviewChatUseCase_WrapsAIProviderFailureAsAppError(t *testing.T) {
	// When the AI factory (or the provider itself) fails, the use case
	// must return a typed AppError(AI_PROVIDER_UNAVAILABLE) so the HTTP
	// layer maps it to a 502 with a readable code. Without this the
	// handler falls back to a generic 500 and the Flutter app shows
	// "An unexpected error occurred" — which is what onboarding users
	// were hitting when the hardcoded model name was invalid.
	ctx := context.Background()

	accountRepo := new(MockAccountRepo)
	agentRepo := new(MockAgentRepo)
	usageRepo := new(MockUsageRepo)
	planRepo := new(MockPlanRepo)
	auditLogRepo := new(MockAuditLogRepo)
	aiFactory := new(MockAIFactory)
	knowledgeRepo := new(MockKnowledgeRepo)
	calendarRepo := new(MockCalendarRepo)

	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	uc := NewPreviewChatUseCase(accountRepo, agentRepo, usageRepo, planRepo,
		auditLogRepo, aiFactory, agentRunner)

	planID := "plan_basic"
	account := &domain.Account{ID: "acc_123", PlanID: &planID}
	plan := &domain.Plan{ID: planID, AIProvider: "gemini", MaxAITokensPerMonth: 1000}
	tokensUsed := int64(10)
	usage := &domain.AccountUsage{
		ID: "usg_123", AITokensUsed: &tokensUsed, AccountID: "acc_123",
	}
	agent := &domain.Agent{ID: "agt_123", AccountID: "acc_123"}

	agentRepo.On("GetByID", ctx, "agt_123").Return(agent, nil)
	accountRepo.On("GetByID", ctx, "acc_123").Return(account, nil)
	planRepo.On("GetByID", ctx, planID).Return(plan, nil)
	usageRepo.On("GetByAccountID", ctx, "acc_123").Return(usage, nil)
	knowledgeRepo.On("ListSources", ctx, "agt_123").Return([]*domain.KnowledgeSource{}, nil)
	calendarRepo.On("ListByAgentID", ctx, "agt_123").Return([]*domain.Calendar{}, nil)

	// Simulate the bug we hit in prod: model name doesn't exist, so
	// GetProvider returns a raw error from the SDK.
	aiFactory.On("GetProvider", ctx, "gemini").
		Return((domain.AIProvider)(nil), errors.New("model_not_found"))

	resp, err := uc.Execute(ctx, PreviewChatDTO{
		AccountID: "acc_123",
		AgentID:   "agt_123",
		Message:   "Olá",
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	if !ok {
		t.Fatalf("expected *domain.AppError, got %T: %v", err, err)
	}
	assert.Equal(t, "AI_PROVIDER_UNAVAILABLE", appErr.Code)
	assert.Contains(t, appErr.Message, "model_not_found")
}

func TestParseSuggestions_HappyPath(t *testing.T) {
	raw := `Somos uma clínica odontológica.

[SUGESTOES]Qual o valor da consulta?|Atendem por convênio?|Como marcar um horário?[/SUGESTOES]`

	cleaned, suggestions := parseSuggestions(raw)

	assert.Equal(t, "Somos uma clínica odontológica.", cleaned)
	assert.Equal(t, []string{
		"Qual o valor da consulta?",
		"Atendem por convênio?",
		"Como marcar um horário?",
	}, suggestions)
}

func TestParseSuggestions_NoBlock(t *testing.T) {
	raw := "Resposta normal sem bloco de sugestões."
	cleaned, suggestions := parseSuggestions(raw)
	assert.Equal(t, raw, cleaned)
	assert.Nil(t, suggestions)
}

func TestParseSuggestions_CapsToThree(t *testing.T) {
	raw := "Ok!\n[SUGESTOES]A?|B?|C?|D?|E?[/SUGESTOES]"
	_, suggestions := parseSuggestions(raw)
	assert.Len(t, suggestions, 3)
}

// O agente pede transferência emitindo um token de controle. A conversa real
// consome esse token (inbound_webhook.go) e pausa o bot; o preview não tinha
// esse passo, então o token chegava à tela literalmente — quem estava avaliando
// o produto digitava uma pergunta e recebia "[HANDOFF]" de volta, na única tela
// feita para mostrar o agente funcionando.
func TestPreviewChatUseCase_HandoffTokenNeverReachesTheScreen(t *testing.T) {
	ctx := context.Background()

	newUC := func(content string) (*PreviewChatUseCase, PreviewChatDTO) {
		accountRepo := new(MockAccountRepo)
		agentRepo := new(MockAgentRepo)
		usageRepo := new(MockUsageRepo)
		planRepo := new(MockPlanRepo)
		auditLogRepo := new(MockAuditLogRepo)
		aiFactory := new(MockAIFactory)
		knowledgeRepo := new(MockKnowledgeRepo)
		calendarRepo := new(MockCalendarRepo)

		runner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
		uc := NewPreviewChatUseCase(accountRepo, agentRepo, usageRepo, planRepo,
			auditLogRepo, aiFactory, runner)

		planID := "plan_x"
		tokensUsed := int64(0)
		agentRepo.On("GetByID", ctx, "agt_1").
			Return(&domain.Agent{ID: "agt_1", AccountID: "acc_1"}, nil)
		accountRepo.On("GetByID", ctx, "acc_1").
			Return(&domain.Account{ID: "acc_1", PlanID: &planID}, nil)
		planRepo.On("GetByID", ctx, planID).Return(&domain.Plan{
			ID: planID, MaxAITokensPerMonth: 10000, AIProvider: "gemini",
			InputMultiplier: 1, OutputMultiplier: 1,
		}, nil)
		usageRepo.On("GetByAccountID", ctx, "acc_1").Return(
			&domain.AccountUsage{ID: "u1", AccountID: "acc_1", AITokensUsed: &tokensUsed}, nil)
		usageRepo.On("Update", ctx, mock.Anything).Return(nil)
		knowledgeRepo.On("ListSources", ctx, "agt_1").Return([]*domain.KnowledgeSource{}, nil)
		calendarRepo.On("ListByAgentID", ctx, "agt_1").Return([]*domain.Calendar{}, nil)
		auditLogRepo.On("Create", ctx, mock.Anything).Return(nil)
		accountRepo.On("MarkPreviewChatTested", ctx, "acc_1").Return(nil)

		prv := new(MockAIProvider)
		aiFactory.On("GetProvider", ctx, "gemini").Return(prv, nil)
		prv.On("GenerateResponse", ctx, mock.Anything, mock.Anything, mock.Anything).
			Return(&domain.AICallbackResponse{
				Content: content,
				Usage:   domain.AIUsage{PromptTokens: 10, CompletionTokens: 10},
			}, nil)

		return uc, PreviewChatDTO{AccountID: "acc_1", AgentID: "agt_1", Message: "oi"}
	}

	t.Run("o token sai do texto e vira um sinal", func(t *testing.T) {
		uc, dto := newUC(services.HandoffSentinel)

		resp, err := uc.Execute(ctx, dto)

		assert.NoError(t, err)
		assert.True(t, resp.Handoff, "o cliente precisa saber que houve transferência")
		assert.NotContains(t, resp.Response, services.HandoffSentinel,
			"o token de controle não pode chegar à tela")
	})

	t.Run("texto junto do token é preservado", func(t *testing.T) {
		// O modelo às vezes escreve uma frase antes de pedir a transferência.
		// Jogar a frase fora perderia a única parte que o cliente entende.
		uc, dto := newUC("Vou chamar alguém do time. " + services.HandoffSentinel)

		resp, err := uc.Execute(ctx, dto)

		assert.NoError(t, err)
		assert.True(t, resp.Handoff)
		assert.Equal(t, "Vou chamar alguém do time.", resp.Response)
	})

	t.Run("uma resposta normal não é marcada como transferência", func(t *testing.T) {
		uc, dto := newUC("Atendemos clínicas sim!")

		resp, err := uc.Execute(ctx, dto)

		assert.NoError(t, err)
		assert.False(t, resp.Handoff)
		assert.Equal(t, "Atendemos clínicas sim!", resp.Response)
	})
}
