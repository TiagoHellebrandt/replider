package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

// buildBootstrapUC wires the BFF aggregator over the shared mocks with the
// minimum stubs every section needs for a happy path.
func buildBootstrapUC(userRepo *MockUserRepo, accountRepo *MockAccountRepo, planRepo *MockPlanRepo, usageRepo *MockUsageRepo, agentRepo *MockAgentRepo, channelRepo *MockChannelRepo, convRepo *MockConversationRepo) *GetDashboardBootstrapUseCase {
	getMeUC := NewGetMeUseCase(userRepo, accountRepo, planRepo, nil)
	getUsageUC := NewGetUsageUseCase(usageRepo, planRepo, accountRepo)
	listAgentsUC := NewListAgentsUseCase(agentRepo)
	listChannelsUC := NewListChannelsUseCase(channelRepo)
	return NewGetDashboardBootstrapUseCase(getMeUC, getUsageUC, listAgentsUC, listChannelsUC, convRepo)
}

func TestGetDashboardBootstrapUseCase_Execute(t *testing.T) {
	ctx := context.Background()
	role := "admin"
	planID := "plan1"

	newUser := func() *domain.User {
		return &domain.User{ID: "user1", AccountID: "acc1", Name: "John", Email: "john@example.com", Role: &role}
	}
	newAccount := func() *domain.Account {
		return &domain.Account{ID: "acc1", Name: "Acme", PlanID: &planID, SubscriptionStatus: "active"}
	}
	newPlan := func() *domain.Plan {
		return &domain.Plan{ID: "plan1", Name: "Bronze", MaxAITokensPerMonth: 500000, MaxConversationsPerMonth: 5000}
	}

	t.Run("aggregates all sections in one call", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		planRepo := new(MockPlanRepo)
		usageRepo := new(MockUsageRepo)
		agentRepo := new(MockAgentRepo)
		channelRepo := new(MockChannelRepo)
		convRepo := new(MockConversationRepo)

		userRepo.On("GetByID", ctx, "user1").Return(newUser(), nil)
		// GetByID is hit by /me, plan-limits and usage — allow any count.
		accountRepo.On("GetByID", ctx, "acc1").Return(newAccount(), nil)
		planRepo.On("GetByID", ctx, "plan1").Return(newPlan(), nil)
		usageRepo.On("GetSummaryByAccountID", ctx, "acc1").Return(&domain.AccountUsage{ID: "u1", AccountID: "acc1"}, nil)
		agentRepo.On("ListByAccountID", ctx, "acc1").Return([]*domain.Agent{{ID: "a1", AccountID: "acc1"}}, nil)
		channelRepo.On("ListByAccountID", ctx, "acc1").Return([]*domain.Channel{{ID: "c1", AccountID: "acc1"}}, nil)
		convRepo.On("ListEnriched", ctx, "acc1", "open", int32(recentConversationsLimit), int32(0), "", "", "").
			Return([]*domain.ConversationSummary{{ID: "conv1"}}, nil)

		uc := buildBootstrapUC(userRepo, accountRepo, planRepo, usageRepo, agentRepo, channelRepo, convRepo)
		resp, err := uc.Execute(ctx, "user1", "acc1")

		assert.NoError(t, err)
		assert.NotNil(t, resp.Me)
		assert.Equal(t, "user1", resp.Me.User.ID)
		assert.NotNil(t, resp.PlanLimits)
		assert.Equal(t, "Bronze", resp.PlanLimits.PlanName)
		assert.NotNil(t, resp.Usage)
		assert.Len(t, resp.Agents, 1)
		assert.Len(t, resp.Channels, 1)
		assert.Len(t, resp.RecentConversations, 1)
		assert.Empty(t, resp.Warnings)
	})

	t.Run("aborts when identity fails", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		planRepo := new(MockPlanRepo)
		usageRepo := new(MockUsageRepo)
		agentRepo := new(MockAgentRepo)
		channelRepo := new(MockChannelRepo)
		convRepo := new(MockConversationRepo)

		// Account resolution is the first step of identity; failing it aborts
		// the whole bootstrap before any secondary section runs.
		accountRepo.On("GetByID", ctx, "acc1").Return(nil, errors.New("db down"))

		uc := buildBootstrapUC(userRepo, accountRepo, planRepo, usageRepo, agentRepo, channelRepo, convRepo)
		resp, err := uc.Execute(ctx, "user1", "acc1")

		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("degrades a failing secondary section to a warning", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		planRepo := new(MockPlanRepo)
		usageRepo := new(MockUsageRepo)
		agentRepo := new(MockAgentRepo)
		channelRepo := new(MockChannelRepo)
		convRepo := new(MockConversationRepo)

		userRepo.On("GetByID", ctx, "user1").Return(newUser(), nil)
		accountRepo.On("GetByID", ctx, "acc1").Return(newAccount(), nil)
		planRepo.On("GetByID", ctx, "plan1").Return(newPlan(), nil)
		usageRepo.On("GetSummaryByAccountID", ctx, "acc1").Return(&domain.AccountUsage{ID: "u1", AccountID: "acc1"}, nil)
		agentRepo.On("ListByAccountID", ctx, "acc1").Return([]*domain.Agent{}, nil)
		channelRepo.On("ListByAccountID", ctx, "acc1").Return([]*domain.Channel{}, nil)
		// Recent conversations subsystem is down.
		convRepo.On("ListEnriched", ctx, "acc1", "open", int32(recentConversationsLimit), int32(0), "", "", "").
			Return([]*domain.ConversationSummary(nil), errors.New("timeout"))

		uc := buildBootstrapUC(userRepo, accountRepo, planRepo, usageRepo, agentRepo, channelRepo, convRepo)
		resp, err := uc.Execute(ctx, "user1", "acc1")

		assert.NoError(t, err)
		assert.NotNil(t, resp.Me)
		assert.Empty(t, resp.RecentConversations)
		assert.Contains(t, resp.Warnings, "recent_conversations")
	})
}
