package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/config"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// passGuard returns a permissive usage guard for tests that don't exercise
// quota limits — every check passes. US-021 made the guard a required
// constructor arg (nil panics), so tests must always supply one.
func passGuard() *MockUsageChecker {
	g := &MockUsageChecker{}
	g.On("CheckAccountActive", mock.Anything).Return(nil).Maybe()
	g.On("Check", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	g.On("CheckAllowance", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	return g
}

// TestNewProcessMessageUseCase_NilGuardPanics locks in US-021: a nil
// usageGuard must fail fast at construction (startup) rather than silently
// disabling all quota enforcement until a request hits it at runtime.
func TestNewProcessMessageUseCase_NilGuardPanics(t *testing.T) {
	assert.Panics(t, func() {
		NewProcessMessageUseCase(
			new(MockUsageRepo), new(MockPlanRepo), new(MockAgentRepo), new(MockAccountRepo),
			new(MockConversationRepo), new(MockMessageRepo), new(MockCalendarRepo), new(MockUserRepo),
			nil, nil, &MockCalendarAdapter{}, new(MockEmailService), "http://localhost:8080", nil,
		)
	})
}

// --- Test Suite ---

func TestProcessMessageUseCase_Execute_Success(t *testing.T) {
	agentRepo := &MockAgentRepo{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}
	usageRepo := &MockUsageRepo{}
	convRepo := &MockConversationRepo{}
	knowledgeRepo := &MockKnowledgeRepo{}
	calendarAdapter := &MockCalendarAdapter{}

	cfg := &config.Config{GeminiAPIKey: "fake", AIProvider: "gemini"}
	aiFactory := services.NewAIFactory(cfg)

	planID := "plan-123"
	tokensUsed := int64(500)

	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Name: "Bot"}
	account := &domain.Account{ID: "acc-1", PlanID: &planID}
	plan := &domain.Plan{
		ID:                  planID,
		AIProvider:          "gemini",
		AIModel:             "gemini-1.5-flash",
		InputMultiplier:     1.0,
		OutputMultiplier:    1.0,
		MaxAITokensPerMonth: 100000,
	}
	usage := &domain.AccountUsage{AccountID: "acc-1", AITokensUsed: &tokensUsed}

	agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(account, nil)
	planRepo.On("GetByID", mock.Anything, planID).Return(plan, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc-1").Return(usage, nil)
	usageRepo.On("AddUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	calendarRepo := &MockCalendarRepo{}
	knowledgeRepo.On("ListSources", mock.Anything, mock.Anything).Return([]*domain.KnowledgeSource{}, nil)

	calendarRepo.On("ListByAgentID", mock.Anything, mock.Anything).Return([]*domain.Calendar{}, nil)

	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	uc := NewProcessMessageUseCase(usageRepo, planRepo, agentRepo, accountRepo, convRepo, new(MockMessageRepo), calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter, new(MockEmailService), "http://localhost:8080", passGuard())

	dto := ProcessMessageDTO{AccountID: "acc-1", AgentID: "agent-1", Message: "Hello!"}
	result, err := uc.Execute(context.Background(), dto)

	_ = result
	assert.NotEqual(t, domain.ErrPlanNotFound, err)
	assert.NotEqual(t, domain.ErrNoCredits, err)

	agentRepo.AssertExpectations(t)
	accountRepo.AssertExpectations(t)
	planRepo.AssertExpectations(t)
	usageRepo.AssertExpectations(t)
}

func TestProcessMessageUseCase_Execute_TokenLimitExceeded(t *testing.T) {
	agentRepo := &MockAgentRepo{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}
	usageRepo := &MockUsageRepo{}
	convRepo := &MockConversationRepo{}
	calendarAdapter := &MockCalendarAdapter{}
	guard := &MockUsageChecker{}

	cfg := &config.Config{GeminiAPIKey: "fake", AIProvider: "gemini"}
	aiFactory := services.NewAIFactory(cfg)

	planID := "plan-123"
	tokensUsed := int64(110001)
	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Name: "Bot"}
	account := &domain.Account{ID: "acc-1", PlanID: &planID}
	plan := &domain.Plan{
		ID: planID, AIProvider: "gemini", AIModel: "gemini-1.5-flash",
		InputMultiplier: 1.0, OutputMultiplier: 1.0,
		MaxAITokensPerMonth: 100000, MaxConversationsPerMonth: 1000,
	}
	usage := &domain.AccountUsage{AccountID: "acc-1", AITokensUsed: &tokensUsed}

	agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(account, nil)
	planRepo.On("GetByID", mock.Anything, planID).Return(plan, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc-1").Return(usage, nil)
	guard.On("CheckAccountActive", account).Return(nil)
	guard.On("Check", plan, usage, "conversations").Return(nil)
	guard.On("Check", plan, usage, "tokens").Return(domain.ErrUsageLimitExceeded)

	knowledgeRepo := &MockKnowledgeRepo{}
	calendarRepo := &MockCalendarRepo{}
	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	uc := NewProcessMessageUseCase(usageRepo, planRepo, agentRepo, accountRepo, convRepo, new(MockMessageRepo), calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter, new(MockEmailService), "http://localhost:8080", guard)

	result, err := uc.Execute(context.Background(), ProcessMessageDTO{AccountID: "acc-1", AgentID: "agent-1", Message: "Hello!"})
	assert.Nil(t, result)
	assert.Equal(t, domain.ErrUsageLimitExceeded, err)
}

func TestProcessMessageUseCase_Execute_ConversationLimitExceeded(t *testing.T) {
	agentRepo := &MockAgentRepo{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}
	usageRepo := &MockUsageRepo{}
	convRepo := &MockConversationRepo{}
	calendarAdapter := &MockCalendarAdapter{}
	guard := &MockUsageChecker{}

	cfg := &config.Config{GeminiAPIKey: "fake", AIProvider: "gemini"}
	aiFactory := services.NewAIFactory(cfg)

	planID := "plan-123"
	tokensUsed := int64(500)
	convCount := int32(1000)
	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Name: "Bot"}
	account := &domain.Account{ID: "acc-1", PlanID: &planID}
	plan := &domain.Plan{
		ID: planID, AIProvider: "gemini", AIModel: "gemini-1.5-flash",
		InputMultiplier: 1.0, OutputMultiplier: 1.0,
		MaxAITokensPerMonth: 100000, MaxConversationsPerMonth: 1000,
	}
	usage := &domain.AccountUsage{AccountID: "acc-1", AITokensUsed: &tokensUsed, ConversationsCount: &convCount}

	agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(account, nil)
	planRepo.On("GetByID", mock.Anything, planID).Return(plan, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc-1").Return(usage, nil)
	guard.On("CheckAccountActive", account).Return(nil)
	guard.On("Check", plan, usage, "conversations").Return(domain.ErrUsageLimitExceeded)

	knowledgeRepo := &MockKnowledgeRepo{}
	calendarRepo := &MockCalendarRepo{}
	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	uc := NewProcessMessageUseCase(usageRepo, planRepo, agentRepo, accountRepo, convRepo, new(MockMessageRepo), calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter, new(MockEmailService), "http://localhost:8080", guard)

	result, err := uc.Execute(context.Background(), ProcessMessageDTO{AccountID: "acc-1", AgentID: "agent-1", Message: "Hello!"})
	assert.Nil(t, result)
	assert.Equal(t, domain.ErrUsageLimitExceeded, err)
}

func TestProcessMessageUseCase_Execute_AddonTokensCoverOverage(t *testing.T) {
	agentRepo := &MockAgentRepo{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}
	usageRepo := &MockUsageRepo{}
	convRepo := &MockConversationRepo{}
	calendarAdapter := &MockCalendarAdapter{}
	guard := &MockUsageChecker{}

	cfg := &config.Config{GeminiAPIKey: "fake", AIProvider: "gemini"}
	aiFactory := services.NewAIFactory(cfg)

	planID := "plan-123"
	tokensUsed := int64(105000)
	addonTokens := int64(50000)
	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Name: "Bot"}
	account := &domain.Account{ID: "acc-1", PlanID: &planID}
	plan := &domain.Plan{
		ID: planID, AIProvider: "gemini", AIModel: "gemini-1.5-flash",
		InputMultiplier: 1.0, OutputMultiplier: 1.0,
		MaxAITokensPerMonth: 100000, MaxConversationsPerMonth: 1000,
	}
	usage := &domain.AccountUsage{AccountID: "acc-1", AITokensUsed: &tokensUsed, AddonTokens: &addonTokens}

	agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(account, nil)
	planRepo.On("GetByID", mock.Anything, planID).Return(plan, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc-1").Return(usage, nil)
	usageRepo.On("AddUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	guard.On("CheckAccountActive", account).Return(nil)
	guard.On("Check", plan, usage, "conversations").Return(nil)
	guard.On("Check", plan, usage, "tokens").Return(nil)

	knowledgeRepo := &MockKnowledgeRepo{}
	calendarRepo := &MockCalendarRepo{}
	knowledgeRepo.On("ListSources", mock.Anything, mock.Anything).Return([]*domain.KnowledgeSource{}, nil)
	calendarRepo.On("ListByAgentID", mock.Anything, mock.Anything).Return([]*domain.Calendar{}, nil)
	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	uc := NewProcessMessageUseCase(usageRepo, planRepo, agentRepo, accountRepo, convRepo, new(MockMessageRepo), calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter, new(MockEmailService), "http://localhost:8080", guard)

	result, err := uc.Execute(context.Background(), ProcessMessageDTO{AccountID: "acc-1", AgentID: "agent-1", Message: "Hello!"})
	// Should proceed past limits — AI provider may fail but not a limit error
	assert.NotEqual(t, domain.ErrUsageLimitExceeded, err)
	_ = result
}

func TestProcessMessageUseCase_Execute_TrialLimitsExceeded(t *testing.T) {
	agentRepo := &MockAgentRepo{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}
	usageRepo := &MockUsageRepo{}
	convRepo := &MockConversationRepo{}
	calendarAdapter := &MockCalendarAdapter{}
	guard := &MockUsageChecker{}

	cfg := &config.Config{GeminiAPIKey: "fake", AIProvider: "gemini"}
	aiFactory := services.NewAIFactory(cfg)

	tokensUsed := int64(11001)
	convCount := int32(50)
	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Name: "Bot"}
	account := &domain.Account{ID: "acc-1", PlanID: nil}
	trialPlan := domain.TrialPlan()
	usage := &domain.AccountUsage{AccountID: "acc-1", AITokensUsed: &tokensUsed, ConversationsCount: &convCount}

	agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(account, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc-1").Return(usage, nil)
	guard.On("CheckAccountActive", account).Return(nil)
	guard.On("Check", trialPlan, usage, "conversations").Return(nil)
	guard.On("Check", trialPlan, usage, "tokens").Return(domain.ErrUsageLimitExceeded)

	knowledgeRepo := &MockKnowledgeRepo{}
	calendarRepo := &MockCalendarRepo{}
	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	uc := NewProcessMessageUseCase(usageRepo, planRepo, agentRepo, accountRepo, convRepo, new(MockMessageRepo), calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter, new(MockEmailService), "http://localhost:8080", guard)

	result, err := uc.Execute(context.Background(), ProcessMessageDTO{AccountID: "acc-1", AgentID: "agent-1", Message: "Hello!"})
	assert.Nil(t, result)
	assert.Equal(t, domain.ErrUsageLimitExceeded, err)
}

// TestProcessMessageUseCase_Execute_TrialPlan_NilPlanID verifies that accounts
// without a paid plan fall back to the Trial plan instead of returning PLAN_NOT_FOUND.
func TestProcessMessageUseCase_Execute_TrialPlan_NilPlanID(t *testing.T) {
	agentRepo := &MockAgentRepo{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}
	usageRepo := &MockUsageRepo{}
	convRepo := &MockConversationRepo{}
	knowledgeRepo := &MockKnowledgeRepo{}
	calendarAdapter := &MockCalendarAdapter{}

	cfg := &config.Config{GeminiAPIKey: "fake", AIProvider: "gemini"}
	aiFactory := services.NewAIFactory(cfg)

	tokensUsed := int64(0)
	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Name: "Bot"}
	account := &domain.Account{ID: "acc-1", PlanID: nil} // no paid plan
	usage := &domain.AccountUsage{AccountID: "acc-1", AITokensUsed: &tokensUsed}

	agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(account, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc-1").Return(usage, nil)
	usageRepo.On("AddUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	calendarRepo := &MockCalendarRepo{}
	knowledgeRepo.On("ListSources", mock.Anything, mock.Anything).Return([]*domain.KnowledgeSource{}, nil)

	calendarRepo.On("ListByAgentID", mock.Anything, mock.Anything).Return([]*domain.Calendar{}, nil)

	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	uc := NewProcessMessageUseCase(usageRepo, planRepo, agentRepo, accountRepo, convRepo, new(MockMessageRepo), calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter, new(MockEmailService), "http://localhost:8080", passGuard())

	dto := ProcessMessageDTO{AccountID: "acc-1", AgentID: "agent-1", Message: "Hello!"}
	result, err := uc.Execute(context.Background(), dto)

	// Should NOT return ErrPlanNotFound — use trial plan and proceed
	assert.NotEqual(t, domain.ErrPlanNotFound, err)
	_ = result
}

// TestProcessMessageUseCase_Execute_PlanMissing locks in US-043: when an
// account references a PlanID but the plan row can't be fetched, we must NOT
// silently downgrade to the trial plan (a paying account would get throttled
// and the token-cap logic would deref a nil plan). Instead the pipeline fails
// loud with BILLING_PLAN_MISSING (mapped to 500).
func TestProcessMessageUseCase_Execute_PlanMissing(t *testing.T) {
	agentRepo := &MockAgentRepo{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}
	usageRepo := &MockUsageRepo{}
	convRepo := &MockConversationRepo{}
	knowledgeRepo := &MockKnowledgeRepo{}
	calendarAdapter := &MockCalendarAdapter{}

	cfg := &config.Config{GeminiAPIKey: "fake", AIProvider: "gemini"}
	aiFactory := services.NewAIFactory(cfg)

	planID := "plan-missing"
	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Name: "Bot"}
	account := &domain.Account{ID: "acc-1", PlanID: &planID}

	agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(account, nil)
	planRepo.On("GetByID", mock.Anything, planID).Return(nil, errors.New("not found"))

	calendarRepo := &MockCalendarRepo{}
	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	uc := NewProcessMessageUseCase(usageRepo, planRepo, agentRepo, accountRepo, convRepo, new(MockMessageRepo), calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter, new(MockEmailService), "http://localhost:8080", passGuard())

	dto := ProcessMessageDTO{AccountID: "acc-1", AgentID: "agent-1", Message: "Hello!"}
	result, err := uc.Execute(context.Background(), dto)

	assert.Nil(t, result)
	assert.Equal(t, domain.ErrPlanMissing, err)
}

// TestProcessMessageUseCase_Execute_PlanNilNoError covers the other nil branch:
// GetByID returns (nil, nil) — a dangling FK with no DB error. Still must fail
// loud rather than panic on a nil plan deref.
func TestProcessMessageUseCase_Execute_PlanNilNoError(t *testing.T) {
	agentRepo := &MockAgentRepo{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}
	usageRepo := &MockUsageRepo{}
	convRepo := &MockConversationRepo{}
	knowledgeRepo := &MockKnowledgeRepo{}
	calendarAdapter := &MockCalendarAdapter{}

	cfg := &config.Config{GeminiAPIKey: "fake", AIProvider: "gemini"}
	aiFactory := services.NewAIFactory(cfg)

	planID := "plan-dangling"
	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Name: "Bot"}
	account := &domain.Account{ID: "acc-1", PlanID: &planID}

	agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(account, nil)
	planRepo.On("GetByID", mock.Anything, planID).Return(nil, nil)

	calendarRepo := &MockCalendarRepo{}
	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	uc := NewProcessMessageUseCase(usageRepo, planRepo, agentRepo, accountRepo, convRepo, new(MockMessageRepo), calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter, new(MockEmailService), "http://localhost:8080", passGuard())

	dto := ProcessMessageDTO{AccountID: "acc-1", AgentID: "agent-1", Message: "Hello!"}
	result, err := uc.Execute(context.Background(), dto)

	assert.Nil(t, result)
	assert.Equal(t, domain.ErrPlanMissing, err)
}

func TestProcessMessageUseCase_Execute_AccountSuspended(t *testing.T) {
	agentRepo := &MockAgentRepo{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}
	usageRepo := &MockUsageRepo{}
	convRepo := &MockConversationRepo{}
	calendarAdapter := &MockCalendarAdapter{}
	guard := &MockUsageChecker{}

	cfg := &config.Config{GeminiAPIKey: "fake", AIProvider: "gemini"}
	aiFactory := services.NewAIFactory(cfg)

	planID := "plan-123"
	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Name: "Bot"}
	account := &domain.Account{ID: "acc-1", PlanID: &planID, SubscriptionStatus: domain.SubscriptionStatusPastDue}

	agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(account, nil)
	guard.On("CheckAccountActive", account).Return(domain.ErrAccountSuspended)

	knowledgeRepo := &MockKnowledgeRepo{}
	calendarRepo := &MockCalendarRepo{}
	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	uc := NewProcessMessageUseCase(usageRepo, planRepo, agentRepo, accountRepo, convRepo, new(MockMessageRepo), calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter, new(MockEmailService), "http://localhost:8080", guard)

	result, err := uc.Execute(context.Background(), ProcessMessageDTO{AccountID: "acc-1", AgentID: "agent-1", Message: "Hello!"})
	assert.Nil(t, result)
	assert.Equal(t, domain.ErrAccountSuspended, err)
}

// TrialBypass exempts the account from the past_due grace gate so the
// owner + Meta App Reviewer accounts keep getting AI replies without a
// live Stripe subscription. Regression: previously the inbound AI
// pipeline never read the flag and silently saved a "Conta suspensa"
// fallback that never reached the channel — customer saw nothing.
func TestProcessMessageUseCase_Execute_TrialBypassSkipsSuspendedCheck(t *testing.T) {
	agentRepo := &MockAgentRepo{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}
	usageRepo := &MockUsageRepo{}
	convRepo := &MockConversationRepo{}
	knowledgeRepo := &MockKnowledgeRepo{}
	calendarAdapter := &MockCalendarAdapter{}
	guard := &MockUsageChecker{}

	cfg := &config.Config{GeminiAPIKey: "fake", AIProvider: "gemini"}
	aiFactory := services.NewAIFactory(cfg)

	planID := "plan-123"
	tokensUsed := int64(0)
	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Name: "Bot"}
	// Past_due account that, without TrialBypass, would trip
	// CheckAccountActive and return ErrAccountSuspended.
	account := &domain.Account{
		ID:                 "acc-1",
		PlanID:             &planID,
		SubscriptionStatus: domain.SubscriptionStatusPastDue,
		TrialBypass:        true,
	}
	plan := &domain.Plan{
		ID:                  planID,
		AIProvider:          "gemini",
		AIModel:             "gemini-1.5-flash",
		InputMultiplier:     1.0,
		OutputMultiplier:    1.0,
		MaxAITokensPerMonth: 100000,
	}
	usage := &domain.AccountUsage{AccountID: "acc-1", AITokensUsed: &tokensUsed}

	agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(account, nil)
	planRepo.On("GetByID", mock.Anything, planID).Return(plan, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc-1").Return(usage, nil)
	usageRepo.On("AddUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	// Plan-limit checks still apply per the documented bypass scope.
	guard.On("Check", plan, usage, "conversations").Return(nil)
	guard.On("Check", plan, usage, "tokens").Return(nil)

	calendarRepo := &MockCalendarRepo{}
	knowledgeRepo.On("ListSources", mock.Anything, mock.Anything).Return([]*domain.KnowledgeSource{}, nil)
	calendarRepo.On("ListByAgentID", mock.Anything, mock.Anything).Return([]*domain.Calendar{}, nil)

	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	uc := NewProcessMessageUseCase(usageRepo, planRepo, agentRepo, accountRepo, convRepo, new(MockMessageRepo), calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter, new(MockEmailService), "http://localhost:8080", guard)

	result, err := uc.Execute(context.Background(), ProcessMessageDTO{AccountID: "acc-1", AgentID: "agent-1", Message: "Hello!"})

	// CheckAccountActive must NOT have been consulted — TrialBypass
	// short-circuits it before the call.
	guard.AssertNotCalled(t, "CheckAccountActive", mock.Anything)
	assert.NotEqual(t, domain.ErrAccountSuspended, err)
	_ = result
}

func TestProcessMessageUseCase_Execute_ErrAgentNotFound(t *testing.T) {
	agentRepo := &MockAgentRepo{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}
	usageRepo := &MockUsageRepo{}
	convRepo := &MockConversationRepo{}
	knowledgeRepo := &MockKnowledgeRepo{}
	calendarAdapter := &MockCalendarAdapter{}

	cfg := &config.Config{GeminiAPIKey: "fake", AIProvider: "gemini"}
	aiFactory := services.NewAIFactory(cfg)

	agentRepo.On("GetByID", mock.Anything, "agent-bad").Return(nil, errors.New("not found"))

	calendarRepo := &MockCalendarRepo{}
	knowledgeRepo.On("ListSources", mock.Anything, mock.Anything).Return([]*domain.KnowledgeSource{}, nil)

	calendarRepo.On("ListByAgentID", mock.Anything, mock.Anything).Return([]*domain.Calendar{}, nil)

	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	uc := NewProcessMessageUseCase(usageRepo, planRepo, agentRepo, accountRepo, convRepo, new(MockMessageRepo), calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter, new(MockEmailService), "http://localhost:8080", passGuard())

	dto := ProcessMessageDTO{AccountID: "acc-1", AgentID: "agent-bad", Message: "Hello!"}
	result, err := uc.Execute(context.Background(), dto)

	assert.Nil(t, result)
	assert.Equal(t, domain.ErrAgentNotFound, err)
}

func TestProcessMessageUseCase_Execute_ErrBotPaused(t *testing.T) {
	agentRepo := &MockAgentRepo{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}
	usageRepo := &MockUsageRepo{}
	convRepo := &MockConversationRepo{}
	knowledgeRepo := &MockKnowledgeRepo{}
	calendarAdapter := &MockCalendarAdapter{}

	cfg := &config.Config{GeminiAPIKey: "fake", AIProvider: "gemini"}
	aiFactory := services.NewAIFactory(cfg)

	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Name: "Bot"}
	conv := &domain.Conversation{ID: "conv-1", AccountID: "acc-1", IsBotPaused: true}

	agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
	convRepo.On("GetByID", mock.Anything, "conv-1", "acc-1").Return(conv, nil)

	calendarRepo := &MockCalendarRepo{}
	knowledgeRepo.On("ListSources", mock.Anything, mock.Anything).Return([]*domain.KnowledgeSource{}, nil)

	calendarRepo.On("ListByAgentID", mock.Anything, mock.Anything).Return([]*domain.Calendar{}, nil)

	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	uc := NewProcessMessageUseCase(usageRepo, planRepo, agentRepo, accountRepo, convRepo, new(MockMessageRepo), calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter, new(MockEmailService), "http://localhost:8080", passGuard())

	dto := ProcessMessageDTO{AccountID: "acc-1", AgentID: "agent-1", ConversationID: "conv-1", Message: "Hello!"}
	result, err := uc.Execute(context.Background(), dto)

	assert.Nil(t, result)
	assert.Equal(t, domain.ErrBotPaused, err)
}

// TestProcessMessageUseCase_Execute_CancellationPropagates verifies US-032:
// Execute derives its 30s deadline FROM the caller's context (not from
// context.Background), so a parent cancellation flows down to the very
// first downstream call. If a future refactor swapped the wrapper to
// context.Background, the matcher below would see a live ctx and fail —
// catching the regression that lets a client-disconnect orphan an
// in-flight AI call.
func TestProcessMessageUseCase_Execute_CancellationPropagates(t *testing.T) {
	agentRepo := &MockAgentRepo{}

	cfg := &config.Config{GeminiAPIKey: "fake", AIProvider: "gemini"}
	aiFactory := services.NewAIFactory(cfg)
	knowledgeRepo := &MockKnowledgeRepo{}
	calendarRepo := &MockCalendarRepo{}
	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)

	// The ctx the repo receives must already be cancelled — proof that
	// the derived (timeout) ctx carries the parent's cancellation.
	agentRepo.On("GetByID", mock.MatchedBy(func(c context.Context) bool {
		return c.Err() != nil
	}), "agent-1").Return(nil, context.Canceled)

	uc := NewProcessMessageUseCase(new(MockUsageRepo), new(MockPlanRepo), agentRepo, new(MockAccountRepo), new(MockConversationRepo), new(MockMessageRepo), calendarRepo, new(MockUserRepo), aiFactory, agentRunner, &MockCalendarAdapter{}, new(MockEmailService), "http://localhost:8080", passGuard())

	parentCtx, cancel := context.WithCancel(context.Background())
	cancel() // client disconnected before we even started

	dto := ProcessMessageDTO{AccountID: "acc-1", AgentID: "agent-1", Message: "Hello!"}
	result, err := uc.Execute(parentCtx, dto)

	assert.Nil(t, result)
	assert.Error(t, err)
	// The MatchedBy assertion only passes if the cancelled ctx reached
	// the repo — i.e. propagation works.
	agentRepo.AssertExpectations(t)
}

// TestProcessMessage_DailyTokenCapEnforced locks in US-025: once an account
// has consumed its daily AI token cap (today), further AI replies are
// skipped with ErrDailyTokenLimitExceeded — bounding a runaway day.
func TestProcessMessage_DailyTokenCapEnforced(t *testing.T) {
	agentRepo := &MockAgentRepo{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}
	usageRepo := &MockUsageRepo{}
	convRepo := &MockConversationRepo{}
	knowledgeRepo := &MockKnowledgeRepo{}
	calendarRepo := &MockCalendarRepo{}

	cfg := &config.Config{GeminiAPIKey: "fake", AIProvider: "gemini"}
	aiFactory := services.NewAIFactory(cfg)

	planID := "plan-1"
	tokensUsed := int64(500)
	now := time.Now()
	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Name: "Bot"}
	account := &domain.Account{ID: "acc-1", PlanID: &planID}
	// Generous monthly limits so only the daily cap can trip.
	plan := &domain.Plan{
		ID: planID, AIProvider: "gemini", AIModel: "gemini-1.5-flash",
		InputMultiplier: 1.0, OutputMultiplier: 1.0,
		MaxAITokensPerMonth: 10000000, MaxConversationsPerMonth: 1000000,
	}
	// Daily counter at the default cap, stamped today.
	usage := &domain.AccountUsage{
		AccountID: "acc-1", AITokensUsed: &tokensUsed,
		DailyTokens: domain.DefaultMaxDailyAITokens, DailyTokensDate: &now,
	}

	agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(account, nil)
	planRepo.On("GetByID", mock.Anything, planID).Return(plan, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc-1").Return(usage, nil)

	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	uc := NewProcessMessageUseCase(usageRepo, planRepo, agentRepo, accountRepo, convRepo, new(MockMessageRepo), calendarRepo, new(MockUserRepo), aiFactory, agentRunner, &MockCalendarAdapter{}, new(MockEmailService), "http://localhost:8080", passGuard())

	result, err := uc.Execute(context.Background(), ProcessMessageDTO{AccountID: "acc-1", AgentID: "agent-1", Message: "Hi"})
	assert.Nil(t, result)
	assert.Equal(t, domain.ErrDailyTokenLimitExceeded, err)
}

// TestProcessMessage_DailyCapResetsOnNewDay confirms a stale counter from a
// previous day does NOT block today's replies.
func TestProcessMessage_DailyCapResetsOnNewDay(t *testing.T) {
	usage := &domain.AccountUsage{}
	yesterday := time.Now().AddDate(0, 0, -1)
	usage.DailyTokens = domain.DefaultMaxDailyAITokens * 10
	usage.DailyTokensDate = &yesterday
	// Yesterday's huge counter reads as 0 today.
	assert.Equal(t, int64(0), usage.EffectiveDailyTokens(time.Now()))
}

// TestProcessMessage_DailyCapScalesWithPlan locks in review-fixes US-016:
// a plan carrying its own max_daily_ai_tokens (USD Pro: 21M/day, sized so
// 30 × daily never throttles below the 105M marketed monthly allowance)
// lets an account burn >100K tokens in one day — the old fixed 100K
// default no longer applies when the plan sets a cap.
func TestProcessMessage_DailyCapScalesWithPlan(t *testing.T) {
	agentRepo := &MockAgentRepo{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}
	usageRepo := &MockUsageRepo{}
	convRepo := &MockConversationRepo{}
	knowledgeRepo := &MockKnowledgeRepo{}
	calendarRepo := &MockCalendarRepo{}
	aiFactory := &MockAIFactory{}
	provider := &MockAIProvider{}

	planID := "01PLAN_PRO_USD_00000000"
	tokensUsed := int64(150_000)
	now := time.Now()
	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Name: "Bot"}
	account := &domain.Account{ID: "acc-1", PlanID: &planID}
	// USD Pro shape: the plan's own daily cap, far above the platform
	// default the old code pinned every plan to.
	plan := &domain.Plan{
		ID: planID, Name: "Pro", Currency: "usd",
		AIProvider: "gemini", AIModel: "gemini-1.5-flash",
		InputMultiplier: 1.0, OutputMultiplier: 1.0,
		MaxAITokensPerMonth: 105_000_000, MaxConversationsPerMonth: 3000,
		MaxDailyAITokens: 21_000_000,
	}
	// 150K tokens consumed TODAY — above the 100K default, well inside
	// both the plan's daily cap and monthly franchise.
	usage := &domain.AccountUsage{
		AccountID: "acc-1", AITokensUsed: &tokensUsed,
		DailyTokens: 150_000, DailyTokensDate: &now,
	}

	agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(account, nil)
	planRepo.On("GetByID", mock.Anything, planID).Return(plan, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc-1").Return(usage, nil)
	usageRepo.On("AddUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	knowledgeRepo.On("ListSources", mock.Anything, mock.Anything).Return([]*domain.KnowledgeSource{}, nil)
	calendarRepo.On("ListByAgentID", mock.Anything, "agent-1").Return([]*domain.Calendar{}, nil)
	aiFactory.On("GetProvider", mock.Anything, "gemini").Return(provider, nil)

	closed := make(chan string)
	close(closed)
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&domain.AICallbackResponse{Content: "reply"}, nil)
	provider.On("GenerateStream", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(domain.StreamResult{Chunks: closed, GetUsage: func() (int64, int64) { return 0, 0 }}, nil)

	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	uc := NewProcessMessageUseCase(usageRepo, planRepo, agentRepo, accountRepo, convRepo, new(MockMessageRepo), calendarRepo, new(MockUserRepo), aiFactory, agentRunner, &MockCalendarAdapter{}, new(MockEmailService), "http://localhost:8080", passGuard())

	result, err := uc.Execute(context.Background(), ProcessMessageDTO{AccountID: "acc-1", AgentID: "agent-1", Message: "Hi"})
	assert.NoError(t, err, "a >100K-token day within the plan's own cap must not be blocked")
	if result != nil {
		for range result.Chunks { // drain so the stream goroutine finishes
		}
	}
}

// TestProcessMessage_ToolCallLoopCapped locks in US-023: a single AI turn
// emitting more tool calls than the cap must process at most
// MaxToolCallIterations of them — bounding the fan-out of slow/paid
// Google Calendar lookups a buggy or adversarial model could otherwise
// trigger.
func TestProcessMessage_ToolCallLoopCapped(t *testing.T) {
	agentRepo := &MockAgentRepo{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}
	usageRepo := &MockUsageRepo{}
	convRepo := &MockConversationRepo{}
	knowledgeRepo := &MockKnowledgeRepo{}
	calendarRepo := &MockCalendarRepo{}
	calendarAdapter := &MockCalendarAdapter{}
	aiFactory := &MockAIFactory{}
	provider := &MockAIProvider{}

	planID := "plan-1"
	tokensUsed := int64(0)
	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Name: "Bot"}
	account := &domain.Account{ID: "acc-1", PlanID: &planID}
	plan := &domain.Plan{
		ID: planID, AIProvider: "gemini", AIModel: "gemini-1.5-flash",
		InputMultiplier: 1.0, OutputMultiplier: 1.0,
		MaxAITokensPerMonth: 1000000, MaxConversationsPerMonth: 100000,
	}
	usage := &domain.AccountUsage{AccountID: "acc-1", AITokensUsed: &tokensUsed}
	gcalID := "gcal-1"
	cal := &domain.Calendar{ID: "cal-1", AgentID: "agent-1", Name: "Clinic", IsActive: true, GoogleCalendarID: &gcalID, SlotDurationMinutes: 30}

	agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(account, nil)
	planRepo.On("GetByID", mock.Anything, planID).Return(plan, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc-1").Return(usage, nil)
	usageRepo.On("AddUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	knowledgeRepo.On("ListSources", mock.Anything, mock.Anything).Return([]*domain.KnowledgeSource{}, nil)
	calendarRepo.On("ListByAgentID", mock.Anything, "agent-1").Return([]*domain.Calendar{cal}, nil)
	calendarAdapter.On("ListFreeSlots", mock.Anything, "acc-1", gcalID, mock.Anything).Return([]time.Time{}, nil)

	// GetProviderForAccount delegates to GetProvider in the mock.
	aiFactory.On("GetProvider", mock.Anything, "gemini").Return(provider, nil)

	// Model emits 9 tool calls in one turn — far above the cap of 5.
	var toolCalls []domain.ToolCall
	for i := 0; i < 9; i++ {
		toolCalls = append(toolCalls, domain.ToolCall{
			FunctionName: "check_availability",
			Arguments:    `{"date":"2026-06-10"}`,
		})
	}
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&domain.AICallbackResponse{ToolCalls: toolCalls}, nil)

	closed := make(chan string)
	close(closed)
	provider.On("GenerateStream", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(domain.StreamResult{Chunks: closed, GetUsage: func() (int64, int64) { return 0, 0 }}, nil)

	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	uc := NewProcessMessageUseCase(usageRepo, planRepo, agentRepo, accountRepo, convRepo, new(MockMessageRepo), calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter, new(MockEmailService), "http://localhost:8080", passGuard())

	result, err := uc.Execute(context.Background(), ProcessMessageDTO{AccountID: "acc-1", AgentID: "agent-1", Message: "Quero marcar"})
	assert.NoError(t, err)
	if result != nil {
		for range result.Chunks { // drain so the stream goroutine finishes
		}
	}

	// The decisive assertion: despite 9 requested tool calls, the calendar
	// lookup ran at most the cap (5) times.
	calendarAdapter.AssertNumberOfCalls(t, "ListFreeSlots", services.MaxToolCallIterations)
}

// TestProcessMessage_AvailabilityRespectsAvailableDays locks in that a
// calendar restricted to weekdays (available_days=[1..5]) never offers
// slots for a Saturday, even when the Google agenda is wide open — the
// Google lookup must not even run.
func TestProcessMessage_AvailabilityRespectsAvailableDays(t *testing.T) {
	agentRepo := &MockAgentRepo{}
	accountRepo := &MockAccountRepo{}
	planRepo := &MockPlanRepo{}
	usageRepo := &MockUsageRepo{}
	convRepo := &MockConversationRepo{}
	knowledgeRepo := &MockKnowledgeRepo{}
	calendarRepo := &MockCalendarRepo{}
	calendarAdapter := &MockCalendarAdapter{}
	aiFactory := &MockAIFactory{}
	provider := &MockAIProvider{}

	planID := "plan-1"
	tokensUsed := int64(0)
	agent := &domain.Agent{ID: "agent-1", AccountID: "acc-1", Name: "Bot"}
	account := &domain.Account{ID: "acc-1", PlanID: &planID}
	plan := &domain.Plan{
		ID: planID, AIProvider: "gemini", AIModel: "gemini-1.5-flash",
		InputMultiplier: 1.0, OutputMultiplier: 1.0,
		MaxAITokensPerMonth: 1000000, MaxConversationsPerMonth: 100000,
	}
	usage := &domain.AccountUsage{AccountID: "acc-1", AITokensUsed: &tokensUsed}
	gcalID := "gcal-1"
	cal := &domain.Calendar{
		ID: "cal-1", AgentID: "agent-1", Name: "Demos", IsActive: true,
		GoogleCalendarID: &gcalID, SlotDurationMinutes: 30,
		AvailableDays: []int32{1, 2, 3, 4, 5}, // weekdays only
	}

	agentRepo.On("GetByID", mock.Anything, "agent-1").Return(agent, nil)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(account, nil)
	planRepo.On("GetByID", mock.Anything, planID).Return(plan, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc-1").Return(usage, nil)
	usageRepo.On("AddUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	knowledgeRepo.On("ListSources", mock.Anything, mock.Anything).Return([]*domain.KnowledgeSource{}, nil)
	calendarRepo.On("ListByAgentID", mock.Anything, "agent-1").Return([]*domain.Calendar{cal}, nil)

	aiFactory.On("GetProvider", mock.Anything, "gemini").Return(provider, nil)

	// 2026-06-13 is a Saturday.
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&domain.AICallbackResponse{ToolCalls: []domain.ToolCall{{
			FunctionName: "check_availability",
			Arguments:    `{"date":"2026-06-13"}`,
		}}}, nil)

	closed := make(chan string)
	close(closed)
	provider.On("GenerateStream", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(domain.StreamResult{Chunks: closed, GetUsage: func() (int64, int64) { return 0, 0 }}, nil)

	agentRunner := services.NewAgentRunner(aiFactory, knowledgeRepo, calendarRepo)
	uc := NewProcessMessageUseCase(usageRepo, planRepo, agentRepo, accountRepo, convRepo, new(MockMessageRepo), calendarRepo, new(MockUserRepo), aiFactory, agentRunner, calendarAdapter, new(MockEmailService), "http://localhost:8080", passGuard())

	result, err := uc.Execute(context.Background(), ProcessMessageDTO{AccountID: "acc-1", AgentID: "agent-1", Message: "Quero marcar sábado"})
	assert.NoError(t, err)
	if result != nil {
		for range result.Chunks { // drain so the stream goroutine finishes
		}
	}

	// Saturday is outside available_days: the Google lookup never runs.
	calendarAdapter.AssertNotCalled(t, "ListFreeSlots", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
