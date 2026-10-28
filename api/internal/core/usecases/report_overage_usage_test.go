package usecases

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// overagePrices is the standard test config — all three meters wired.
var overagePrices = OverageMeterPrices{
	ExtraConversation: "price_conv",
	ExtraToken:        "price_tok",
	ExtraAgent:        "price_agent",
}

func activeAccount() *domain.Account {
	return &domain.Account{
		ID:                   "acc_1",
		PlanID:               strPtr("plan_1"),
		StripeSubscriptionID: strPtr("sub_123"),
		SubscriptionStatus:   domain.SubscriptionStatusActive,
	}
}

func soloPlan() *domain.Plan {
	return &domain.Plan{
		ID:                       "plan_1",
		Name:                     "Solo",
		MaxConversationsPerMonth: 100,
		MaxAITokensPerMonth:      10_000,
		MaxAgents:                2,
	}
}

func TestReportOverageUsage_ReportsEachMeterOverage(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	usageRepo := new(MockUsageRepo)
	agentRepo := new(MockAgentRepo)
	payment := new(MockPaymentAdapter)

	cycleStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	convCount := int32(130)     // 30 over the 100 franchise
	tokensUsed := int64(15_000) // 5,000 over the 10,000 franchise
	addon := int64(0)

	accountRepo.On("GetByID", ctx, "acc_1").Return(activeAccount(), nil)
	planRepo.On("GetByID", ctx, "plan_1").Return(soloPlan(), nil)
	usageRepo.On("GetByAccountID", ctx, "acc_1").Return(&domain.AccountUsage{
		AccountID:          "acc_1",
		BillingCycleStart:  cycleStart,
		ConversationsCount: &convCount,
		AITokensUsed:       &tokensUsed,
		AddonTokens:        &addon,
	}, nil)
	// 3 agents against a cap of 2 → 1 over.
	agentRepo.On("ListByAccountID", ctx, "acc_1").Return([]*domain.Agent{{}, {}, {}}, nil)

	idemCycle := cycleStart.Unix()
	payment.On("ReportMeteredUsage", "sub_123", "price_conv", int64(30), mock.MatchedBy(func(k string) bool {
		return k == "overage_extra_conversation_acc_1_"+strconv.FormatInt(idemCycle, 10)
	})).Return(nil)
	payment.On("ReportMeteredUsage", "sub_123", "price_tok", int64(5_000), mock.Anything).Return(nil)
	payment.On("ReportMeteredUsage", "sub_123", "price_agent", int64(1), mock.Anything).Return(nil)

	uc := NewReportOverageUsageUseCase(accountRepo, planRepo, usageRepo, agentRepo, payment, overagePrices)
	n, err := uc.Execute(ctx, "acc_1")

	assert.NoError(t, err)
	assert.Equal(t, 3, n)
	payment.AssertExpectations(t)
}

func TestReportOverageUsage_NoOveragesReportsZero(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	usageRepo := new(MockUsageRepo)
	agentRepo := new(MockAgentRepo)
	payment := new(MockPaymentAdapter)

	convCount := int32(10)
	tokensUsed := int64(500)

	accountRepo.On("GetByID", ctx, "acc_1").Return(activeAccount(), nil)
	planRepo.On("GetByID", ctx, "plan_1").Return(soloPlan(), nil)
	usageRepo.On("GetByAccountID", ctx, "acc_1").Return(&domain.AccountUsage{
		AccountID:          "acc_1",
		ConversationsCount: &convCount,
		AITokensUsed:       &tokensUsed,
	}, nil)
	agentRepo.On("ListByAccountID", ctx, "acc_1").Return([]*domain.Agent{{}}, nil)

	// Under every cap → each meter is still reported, but with quantity 0
	// (action=set keeps Stripe in sync once usage drops back under).
	payment.On("ReportMeteredUsage", "sub_123", mock.Anything, int64(0), mock.Anything).Return(nil)

	uc := NewReportOverageUsageUseCase(accountRepo, planRepo, usageRepo, agentRepo, payment, overagePrices)
	n, err := uc.Execute(ctx, "acc_1")

	assert.NoError(t, err)
	assert.Equal(t, 3, n)
	payment.AssertNumberOfCalls(t, "ReportMeteredUsage", 3)
}

func TestReportOverageUsage_SkipsNonActiveSubscription(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	payment := new(MockPaymentAdapter)

	acc := activeAccount()
	acc.SubscriptionStatus = domain.SubscriptionStatusTrialing
	accountRepo.On("GetByID", ctx, "acc_1").Return(acc, nil)

	uc := NewReportOverageUsageUseCase(accountRepo, new(MockPlanRepo), new(MockUsageRepo), new(MockAgentRepo), payment, overagePrices)
	n, err := uc.Execute(ctx, "acc_1")

	assert.NoError(t, err)
	assert.Equal(t, 0, n)
	payment.AssertNotCalled(t, "ReportMeteredUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestReportOverageUsage_NoPricesConfigured_NoOp(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	payment := new(MockPaymentAdapter)

	uc := NewReportOverageUsageUseCase(accountRepo, new(MockPlanRepo), new(MockUsageRepo), new(MockAgentRepo), payment, OverageMeterPrices{})
	n, err := uc.Execute(ctx, "acc_1")

	assert.NoError(t, err)
	assert.Equal(t, 0, n)
	// The account is never even fetched when the subsystem is dormant.
	accountRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestReportOverageUsage_OnlyConfiguredMetersReported(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	usageRepo := new(MockUsageRepo)
	agentRepo := new(MockAgentRepo)
	payment := new(MockPaymentAdapter)

	convCount := int32(200)

	accountRepo.On("GetByID", ctx, "acc_1").Return(activeAccount(), nil)
	planRepo.On("GetByID", ctx, "plan_1").Return(soloPlan(), nil)
	usageRepo.On("GetByAccountID", ctx, "acc_1").Return(&domain.AccountUsage{
		AccountID:          "acc_1",
		ConversationsCount: &convCount,
	}, nil)
	agentRepo.On("ListByAccountID", ctx, "acc_1").Return([]*domain.Agent{}, nil)

	// Only the conversation meter is wired.
	payment.On("ReportMeteredUsage", "sub_123", "price_conv", int64(100), mock.Anything).Return(nil)

	uc := NewReportOverageUsageUseCase(accountRepo, planRepo, usageRepo, agentRepo, payment,
		OverageMeterPrices{ExtraConversation: "price_conv"})
	n, err := uc.Execute(ctx, "acc_1")

	assert.NoError(t, err)
	assert.Equal(t, 1, n)
	payment.AssertNumberOfCalls(t, "ReportMeteredUsage", 1)
}

func TestReportOverageUsage_PropagatesPaymentError(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	usageRepo := new(MockUsageRepo)
	agentRepo := new(MockAgentRepo)
	payment := new(MockPaymentAdapter)

	convCount := int32(200)

	accountRepo.On("GetByID", ctx, "acc_1").Return(activeAccount(), nil)
	planRepo.On("GetByID", ctx, "plan_1").Return(soloPlan(), nil)
	usageRepo.On("GetByAccountID", ctx, "acc_1").Return(&domain.AccountUsage{
		AccountID:          "acc_1",
		ConversationsCount: &convCount,
	}, nil)
	agentRepo.On("ListByAccountID", ctx, "acc_1").Return([]*domain.Agent{}, nil)
	payment.On("ReportMeteredUsage", "sub_123", "price_conv", int64(100), mock.Anything).
		Return(errors.New("stripe boom"))

	uc := NewReportOverageUsageUseCase(accountRepo, planRepo, usageRepo, agentRepo, payment,
		OverageMeterPrices{ExtraConversation: "price_conv"})
	_, err := uc.Execute(ctx, "acc_1")

	assert.Error(t, err)
}
