package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetAnalytics_Success(t *testing.T) {
	ctx := context.Background()
	analyticsRepo := new(MockAnalyticsRepo)
	usageRepo := new(MockUsageRepo)
	planRepo := new(MockPlanRepo)
	accountRepo := new(MockAccountRepo)

	analyticsRepo.On("GetConversationStats", ctx, "acc-1").Return(&domain.ConversationStats{
		Total: 100, Open: 20, PendingHuman: 5, Closed: 75, Last24h: 10, Last7d: 40,
	}, nil)
	analyticsRepo.On("GetMessageStats", ctx, "acc-1").Return(&domain.MessageStats{
		Total: 500, Bot: 200, Human: 100, Contact: 200, Last24h: 50,
	}, nil)
	analyticsRepo.On("GetDailyConversationCounts", ctx, "acc-1").Return([]domain.DailyCount{
		{Date: time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC), Count: 15},
		{Date: time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC), Count: 10},
	}, nil)
	analyticsRepo.On("GetSLAMetrics", ctx, "acc-1").Return(&domain.SLAMetrics{
		AvgFirstResponseSeconds: 120, AvgResolutionSeconds: 3600,
		PctRespondedWithin5Min: 85, TotalResolved: 50, TotalResponded: 60,
	}, nil)
	analyticsRepo.On("GetAgentPerformance", ctx, "acc-1").Return([]domain.AgentPerformance{
		{AgentID: "user-1", ConversationsHandled: 30, MessagesSent: 90},
	}, nil)
	analyticsRepo.On("GetAIPerformance", ctx, "acc-1").Return(&domain.AIPerformance{TotalConversations: 50, AutonomousConversations: 35}, nil)

	tokensUsed := int64(3000)
	addonTokens := int64(1000)
	usageRepo.On("GetByAccountID", ctx, "acc-1").Return(&domain.AccountUsage{
		ID:           "usage-1",
		AccountID:    "acc-1",
		AITokensUsed: &tokensUsed,
		AddonTokens:  &addonTokens,
	}, nil)

	planID := "plan-1"
	accountRepo.On("GetByID", ctx, "acc-1").Return(&domain.Account{
		ID:     "acc-1",
		PlanID: &planID,
	}, nil)
	planRepo.On("GetByID", ctx, "plan-1").Return(&domain.Plan{
		ID:                  "plan-1",
		Name:                "Pro",
		MaxAITokensPerMonth: 50000,
	}, nil)

	uc := NewGetAnalyticsUseCase(analyticsRepo, usageRepo, planRepo, accountRepo)
	resp, err := uc.Execute(ctx, "acc-1")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(100), resp.Conversations.Total)
	assert.Equal(t, int64(20), resp.Conversations.Open)
	assert.Equal(t, int64(500), resp.Messages.Total)
	assert.Equal(t, int64(200), resp.Messages.Bot)
	assert.Len(t, resp.DailyActivity, 2)
	assert.Equal(t, int64(3000), resp.TokensUsed)
	assert.Equal(t, int64(51000), resp.TokensLimit) // 50000 plan + 1000 addon
	assert.NotNil(t, resp.SLA)
	assert.Equal(t, 120, resp.SLA.AvgFirstResponseSeconds)
	assert.Len(t, resp.AgentPerformance, 1)
	assert.Equal(t, "user-1", resp.AgentPerformance[0].AgentID)
	analyticsRepo.AssertExpectations(t)
	usageRepo.AssertExpectations(t)
	planRepo.AssertExpectations(t)
	accountRepo.AssertExpectations(t)
}

func TestGetAnalytics_NoUsageRecord(t *testing.T) {
	ctx := context.Background()
	analyticsRepo := new(MockAnalyticsRepo)
	usageRepo := new(MockUsageRepo)
	planRepo := new(MockPlanRepo)
	accountRepo := new(MockAccountRepo)

	analyticsRepo.On("GetConversationStats", ctx, "acc-1").Return(&domain.ConversationStats{}, nil)
	analyticsRepo.On("GetMessageStats", ctx, "acc-1").Return(&domain.MessageStats{}, nil)
	analyticsRepo.On("GetDailyConversationCounts", ctx, "acc-1").Return([]domain.DailyCount{}, nil)
	analyticsRepo.On("GetSLAMetrics", ctx, "acc-1").Return(&domain.SLAMetrics{}, nil)
	analyticsRepo.On("GetAgentPerformance", ctx, "acc-1").Return([]domain.AgentPerformance{}, nil)
	analyticsRepo.On("GetAIPerformance", ctx, "acc-1").Return(&domain.AIPerformance{}, nil)

	// No usage record found
	usageRepo.On("GetByAccountID", ctx, "acc-1").Return(nil, errors.New("not found"))

	uc := NewGetAnalyticsUseCase(analyticsRepo, usageRepo, planRepo, accountRepo)
	resp, err := uc.Execute(ctx, "acc-1")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(0), resp.TokensUsed)
	assert.Equal(t, int64(0), resp.TokensLimit)
	analyticsRepo.AssertExpectations(t)
}

func TestGetAnalytics_AnalyticsRepoError(t *testing.T) {
	ctx := context.Background()
	analyticsRepo := new(MockAnalyticsRepo)
	usageRepo := new(MockUsageRepo)
	planRepo := new(MockPlanRepo)
	accountRepo := new(MockAccountRepo)

	analyticsRepo.On("GetConversationStats", ctx, "acc-1").Return((*domain.ConversationStats)(nil), errors.New("db error"))

	uc := NewGetAnalyticsUseCase(analyticsRepo, usageRepo, planRepo, accountRepo)
	resp, err := uc.Execute(ctx, "acc-1")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, "db error", err.Error())
	analyticsRepo.AssertExpectations(t)
}

func TestGetAnalytics_NoPlan_TrialDefaults(t *testing.T) {
	ctx := context.Background()
	analyticsRepo := new(MockAnalyticsRepo)
	usageRepo := new(MockUsageRepo)
	planRepo := new(MockPlanRepo)
	accountRepo := new(MockAccountRepo)

	analyticsRepo.On("GetConversationStats", ctx, "acc-1").Return(&domain.ConversationStats{Total: 5}, nil)
	analyticsRepo.On("GetMessageStats", ctx, "acc-1").Return(&domain.MessageStats{Total: 20}, nil)
	analyticsRepo.On("GetDailyConversationCounts", ctx, "acc-1").Return([]domain.DailyCount{}, nil)
	analyticsRepo.On("GetSLAMetrics", ctx, "acc-1").Return(&domain.SLAMetrics{}, nil)
	analyticsRepo.On("GetAgentPerformance", ctx, "acc-1").Return([]domain.AgentPerformance{}, nil)
	analyticsRepo.On("GetAIPerformance", ctx, "acc-1").Return(&domain.AIPerformance{}, nil)

	tokensUsed := int64(500)
	usageRepo.On("GetByAccountID", ctx, "acc-1").Return(&domain.AccountUsage{
		ID:           "usage-1",
		AccountID:    "acc-1",
		AITokensUsed: &tokensUsed,
	}, nil)

	// Account has no plan (trial)
	accountRepo.On("GetByID", mock.Anything, "acc-1").Return(&domain.Account{
		ID:     "acc-1",
		PlanID: nil,
	}, nil)

	uc := NewGetAnalyticsUseCase(analyticsRepo, usageRepo, planRepo, accountRepo)
	resp, err := uc.Execute(ctx, "acc-1")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(500), resp.TokensUsed)
	assert.Equal(t, int64(0), resp.TokensLimit) // no plan = 0 limit
	analyticsRepo.AssertExpectations(t)
	usageRepo.AssertExpectations(t)
	accountRepo.AssertExpectations(t)
}
