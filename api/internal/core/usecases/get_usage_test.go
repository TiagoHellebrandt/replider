package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestGetUsageUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should return usage with plan limits", func(t *testing.T) {
		usageRepo := new(MockUsageRepo)
		planRepo := new(MockPlanRepo)
		accountRepo := new(MockAccountRepo)

		planID := "plan1"
		account := &domain.Account{
			ID:     "acc1",
			PlanID: &planID,
		}
		tokensUsed := int64(5000)
		convCount := int32(10)
		usage := &domain.AccountUsage{
			ID:                 "usage1",
			AccountID:          "acc1",
			AITokensUsed:       &tokensUsed,
			ConversationsCount: &convCount,
		}
		plan := &domain.Plan{
			ID:                       "plan1",
			MaxAITokensPerMonth:      500000,
			MaxConversationsPerMonth: 5000,
		}

		accountRepo.On("GetByID", ctx, "acc1").Return(account, nil)
		usageRepo.On("GetSummaryByAccountID", ctx, "acc1").Return(usage, nil)
		planRepo.On("GetByID", ctx, "plan1").Return(plan, nil)

		uc := NewGetUsageUseCase(usageRepo, planRepo, accountRepo)
		resp, err := uc.Execute(ctx, "acc1")

		assert.NoError(t, err)
		assert.Equal(t, int64(5000), resp.AITokensUsed)
		assert.Equal(t, int32(10), resp.ConversationsCount)
		assert.Equal(t, int64(500000), resp.MaxAITokensPerMonth)
		assert.Equal(t, 5000, resp.MaxConversationsPerMonth)
		accountRepo.AssertExpectations(t)
		usageRepo.AssertExpectations(t)
		planRepo.AssertExpectations(t)
	})

	t.Run("should return ErrAccountNotFound when account does not exist", func(t *testing.T) {
		usageRepo := new(MockUsageRepo)
		planRepo := new(MockPlanRepo)
		accountRepo := new(MockAccountRepo)

		accountRepo.On("GetByID", ctx, "bad-acc").Return(nil, errors.New("not found"))

		uc := NewGetUsageUseCase(usageRepo, planRepo, accountRepo)
		resp, err := uc.Execute(ctx, "bad-acc")

		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrAccountNotFound, err)
	})

	t.Run("should return default limits when usage record is nil", func(t *testing.T) {
		usageRepo := new(MockUsageRepo)
		planRepo := new(MockPlanRepo)
		accountRepo := new(MockAccountRepo)

		account := &domain.Account{ID: "acc1"}
		accountRepo.On("GetByID", ctx, "acc1").Return(account, nil)
		usageRepo.On("GetSummaryByAccountID", ctx, "acc1").Return(nil, nil)

		uc := NewGetUsageUseCase(usageRepo, planRepo, accountRepo)
		resp, err := uc.Execute(ctx, "acc1")

		assert.NoError(t, err)
		assert.Equal(t, int64(0), resp.AITokensUsed)
		assert.Equal(t, int32(0), resp.ConversationsCount)
		assert.Equal(t, int64(10000), resp.MaxAITokensPerMonth)
		assert.Equal(t, 50, resp.MaxConversationsPerMonth)
	})

	t.Run("should use default limits when account has no plan", func(t *testing.T) {
		usageRepo := new(MockUsageRepo)
		planRepo := new(MockPlanRepo)
		accountRepo := new(MockAccountRepo)

		account := &domain.Account{ID: "acc1", PlanID: nil}
		tokensUsed := int64(100)
		convCount := int32(2)
		usage := &domain.AccountUsage{
			ID:                 "usage1",
			AccountID:          "acc1",
			AITokensUsed:       &tokensUsed,
			ConversationsCount: &convCount,
		}

		accountRepo.On("GetByID", ctx, "acc1").Return(account, nil)
		usageRepo.On("GetSummaryByAccountID", ctx, "acc1").Return(usage, nil)

		uc := NewGetUsageUseCase(usageRepo, planRepo, accountRepo)
		resp, err := uc.Execute(ctx, "acc1")

		assert.NoError(t, err)
		assert.Equal(t, int64(100), resp.AITokensUsed)
		assert.Equal(t, int64(10000), resp.MaxAITokensPerMonth)
		assert.Equal(t, 50, resp.MaxConversationsPerMonth)
	})
}

// The month's consumption in the units the customer buys.
//
// The Plan & Usage screen answered "how much of my package have I used" in TOKENS
// — which is our cost, not something anybody buys. Leads, research and approaches
// were already counted in the sales event log (that is where the enrichment cap
// blocks from), and never left it.

// countingSalesEvents returns one number per kind, and can fail on demand.
type countingSalesEvents struct {
	byKind map[domain.SalesEventKind]int64
	err    error
}

func (c *countingSalesEvents) Record(context.Context, *domain.SalesEvent) error {
	return nil
}

func (c *countingSalesEvents) Metrics(context.Context, string, time.Time, time.Time) (*domain.CaseMetrics, error) {
	return &domain.CaseMetrics{}, nil
}

func (c *countingSalesEvents) CountSince(_ context.Context, _ string,
	kind domain.SalesEventKind, _ time.Time) (int64, error) {
	if c.err != nil {
		return 0, c.err
	}
	return c.byKind[kind], nil
}

func (c *countingSalesEvents) LastApproachAt(context.Context, string) (time.Time, error) {
	return time.Time{}, nil
}

func usageFixture(t *testing.T) (*MockUsageRepo, *MockPlanRepo, *MockAccountRepo) {
	t.Helper()
	ctx := context.Background()
	planID := "plan1"
	usageRepo := new(MockUsageRepo)
	planRepo := new(MockPlanRepo)
	accountRepo := new(MockAccountRepo)

	tokens := int64(10)
	convs := int32(1)
	accountRepo.On("GetByID", ctx, "acc1").
		Return(&domain.Account{ID: "acc1", PlanID: &planID}, nil)
	usageRepo.On("GetSummaryByAccountID", ctx, "acc1").Return(&domain.AccountUsage{
		ID: "u1", AccountID: "acc1", AITokensUsed: &tokens, ConversationsCount: &convs,
	}, nil)
	planRepo.On("GetByID", ctx, "plan1").Return(&domain.Plan{
		ID: "plan1", MaxAITokensPerMonth: 100, MaxConversationsPerMonth: 100,
	}, nil)
	return usageRepo, planRepo, accountRepo
}

func TestGetUsage_CountsTheMonthInSDRUnits(t *testing.T) {
	ctx := context.Background()
	usageRepo, planRepo, accountRepo := usageFixture(t)

	uc := NewGetUsageUseCase(usageRepo, planRepo, accountRepo)
	uc.SetSalesEventRepo(&countingSalesEvents{byKind: map[domain.SalesEventKind]int64{
		domain.SalesEventLeadCreated:   40,
		domain.SalesEventEnrichmentRun: 12,
		domain.SalesEventApproachSent:  7,
		domain.SalesEventMeetingBooked: 2,
	}})

	resp, err := uc.Execute(ctx, "acc1")

	assert.NoError(t, err)
	if assert.NotNil(t, resp.SDR) {
		assert.Equal(t, int64(40), resp.SDR.Leads)
		assert.Equal(t, int64(12), resp.SDR.Enrichments)
		assert.Equal(t, int64(7), resp.SDR.ApproachesSent)
		assert.Equal(t, int64(2), resp.SDR.Meetings)
	}
}

// A failed count is not a zero count. "0 researches this month" because the query
// broke is exactly the lie this block exists to prevent — it disappears entirely,
// and the screen promises nothing it does not know.
func TestGetUsage_AFailedCountIsAbsent_NotZero(t *testing.T) {
	ctx := context.Background()
	usageRepo, planRepo, accountRepo := usageFixture(t)

	uc := NewGetUsageUseCase(usageRepo, planRepo, accountRepo)
	uc.SetSalesEventRepo(&countingSalesEvents{err: errors.New("db caiu")})

	resp, err := uc.Execute(ctx, "acc1")

	assert.NoError(t, err, "o resto do uso continua valendo")
	assert.Nil(t, resp.SDR)
}

// With the log unwired the block does not come — and the rest of the screen does
// not break over it.
func TestGetUsage_WithoutTheEventLogTheBlockIsAbsent(t *testing.T) {
	ctx := context.Background()
	usageRepo, planRepo, accountRepo := usageFixture(t)

	resp, err := NewGetUsageUseCase(usageRepo, planRepo, accountRepo).Execute(ctx, "acc1")

	assert.NoError(t, err)
	assert.Nil(t, resp.SDR)
}
