package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockExperimentRepo struct{ mock.Mock }

func (m *MockExperimentRepo) Create(ctx context.Context, e *domain.PromptExperiment) error {
	return m.Called(ctx, e).Error(0)
}
func (m *MockExperimentRepo) GetByID(ctx context.Context, id string) (*domain.PromptExperiment, error) {
	a := m.Called(ctx, id)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*domain.PromptExperiment), a.Error(1)
}
func (m *MockExperimentRepo) GetRunningByAgentID(ctx context.Context, agentID string) (*domain.PromptExperiment, error) {
	a := m.Called(ctx, agentID)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*domain.PromptExperiment), a.Error(1)
}
func (m *MockExperimentRepo) ListByAccountID(ctx context.Context, accountID string, limit, offset int) ([]*domain.PromptExperiment, error) {
	a := m.Called(ctx, accountID, limit, offset)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).([]*domain.PromptExperiment), a.Error(1)
}
func (m *MockExperimentRepo) ListByAgentID(ctx context.Context, agentID string, limit, offset int) ([]*domain.PromptExperiment, error) {
	a := m.Called(ctx, agentID, limit, offset)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).([]*domain.PromptExperiment), a.Error(1)
}
func (m *MockExperimentRepo) UpdateStatus(ctx context.Context, id, status string, startedAt, stoppedAt *time.Time) error {
	return m.Called(ctx, id, status, startedAt, stoppedAt).Error(0)
}
func (m *MockExperimentRepo) SetWinner(ctx context.Context, id string, winner *string) error {
	return m.Called(ctx, id, winner).Error(0)
}
func (m *MockExperimentRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockExperimentRepo) MetricsForVariant(ctx context.Context, experimentID, variant string) (*domain.VariantMetrics, error) {
	a := m.Called(ctx, experimentID, variant)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*domain.VariantMetrics), a.Error(1)
}
func (m *MockExperimentRepo) SampleConversations(ctx context.Context, experimentID, variant string, bestLimit, worstLimit int) ([]domain.ExperimentSample, []domain.ExperimentSample, error) {
	a := m.Called(ctx, experimentID, variant, bestLimit, worstLimit)
	var best, worst []domain.ExperimentSample
	if a.Get(0) != nil {
		best = a.Get(0).([]domain.ExperimentSample)
	}
	if a.Get(1) != nil {
		worst = a.Get(1).([]domain.ExperimentSample)
	}
	return best, worst, a.Error(2)
}

func newTestPlan(name string) *domain.Plan {
	return &domain.Plan{ID: "p-" + name, Name: name}
}

func makeExperimentUC(t *testing.T, account *domain.Account, plan *domain.Plan, agent *domain.Agent) (*ExperimentUseCase, *MockAccountRepo, *MockPlanRepo, *MockAgentRepo, *MockExperimentRepo, *MockAuditLogRepo) {
	t.Helper()
	accRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	agentRepo := new(MockAgentRepo)
	expRepo := new(MockExperimentRepo)
	auditRepo := new(MockAuditLogRepo)

	if account != nil {
		accRepo.On("GetByID", mock.Anything, account.ID).Return(account, nil).Maybe()
	}
	if plan != nil && account != nil && account.PlanID != nil {
		planRepo.On("GetByID", mock.Anything, *account.PlanID).Return(plan, nil).Maybe()
	}
	if agent != nil {
		agentRepo.On("GetByID", mock.Anything, agent.ID).Return(agent, nil).Maybe()
	}
	auditRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Maybe()

	uc := NewExperimentUseCase(accRepo, planRepo, agentRepo, expRepo, auditRepo)
	return uc, accRepo, planRepo, agentRepo, expRepo, auditRepo
}

func TestExperiment_Create_RequiresPaidPlan(t *testing.T) {
	planID := "plan-solo"
	account := &domain.Account{ID: "acc-1", PlanID: &planID}
	plan := newTestPlan("Solo")
	agent := &domain.Agent{ID: "a-1", AccountID: "acc-1"}
	uc, _, _, _, _, _ := makeExperimentUC(t, account, plan, agent)

	_, err := uc.Create(context.Background(), CreateExperimentDTO{
		AccountID:    "acc-1",
		AgentID:      "a-1",
		Name:         "test",
		VariantABody: "A",
		VariantBBody: "B",
	})
	assert.ErrorIs(t, err, domain.ErrPlanFeatureLocked)
}

func TestExperiment_Create_AllowsProPlan(t *testing.T) {
	planID := "plan-pro"
	account := &domain.Account{ID: "acc-1", PlanID: &planID}
	plan := newTestPlan("Pro")
	agent := &domain.Agent{ID: "a-1", AccountID: "acc-1"}
	uc, _, _, _, expRepo, _ := makeExperimentUC(t, account, plan, agent)

	expRepo.On("Create", mock.Anything, mock.MatchedBy(func(e *domain.PromptExperiment) bool {
		return e.AgentID == "a-1" && e.Status == domain.ExperimentStatusDraft &&
			e.SplitPctB == 50 && e.VariantALabel == "A" && e.VariantBLabel == "B"
	})).Return(nil)

	e, err := uc.Create(context.Background(), CreateExperimentDTO{
		AccountID:    "acc-1",
		AgentID:      "a-1",
		Name:         "Friendlier opener",
		VariantABody: "Hello!",
		VariantBBody: "Hey!",
	})
	assert.NoError(t, err)
	assert.Equal(t, domain.ExperimentStatusDraft, e.Status)
	expRepo.AssertExpectations(t)
}

func TestExperiment_Create_RejectsIdenticalBodies(t *testing.T) {
	planID := "plan-pro"
	account := &domain.Account{ID: "acc-1", PlanID: &planID}
	plan := newTestPlan("Pro")
	agent := &domain.Agent{ID: "a-1", AccountID: "acc-1"}
	uc, _, _, _, _, _ := makeExperimentUC(t, account, plan, agent)

	_, err := uc.Create(context.Background(), CreateExperimentDTO{
		AccountID:    "acc-1",
		AgentID:      "a-1",
		Name:         "x",
		VariantABody: "same",
		VariantBBody: "same",
	})
	assert.Error(t, err)
}

func TestExperiment_Start_DraftToRunning(t *testing.T) {
	planID := "plan-pro"
	account := &domain.Account{ID: "acc-1", PlanID: &planID}
	plan := newTestPlan("Pro")
	agent := &domain.Agent{ID: "a-1", AccountID: "acc-1"}
	uc, _, _, _, expRepo, _ := makeExperimentUC(t, account, plan, agent)

	exp := &domain.PromptExperiment{ID: "e-1", AccountID: "acc-1", AgentID: "a-1", Status: domain.ExperimentStatusDraft}
	expRepo.On("GetByID", mock.Anything, "e-1").Return(exp, nil)
	expRepo.On("UpdateStatus", mock.Anything, "e-1", domain.ExperimentStatusRunning,
		mock.AnythingOfType("*time.Time"), (*time.Time)(nil)).Return(nil)

	out, err := uc.Start(context.Background(), "acc-1", "e-1")
	assert.NoError(t, err)
	assert.Equal(t, domain.ExperimentStatusRunning, out.Status)
}

type fakeCacheInv struct{ called []string }

func (f *fakeCacheInv) InvalidateExperimentCache(agentID string) {
	f.called = append(f.called, agentID)
}

func TestExperiment_Start_InvalidatesCache(t *testing.T) {
	planID := "plan-pro"
	account := &domain.Account{ID: "acc-1", PlanID: &planID}
	plan := newTestPlan("Pro")
	agent := &domain.Agent{ID: "a-1", AccountID: "acc-1"}
	uc, _, _, _, expRepo, _ := makeExperimentUC(t, account, plan, agent)
	inv := &fakeCacheInv{}
	uc.SetCacheInvalidator(inv)

	exp := &domain.PromptExperiment{ID: "e-1", AccountID: "acc-1", AgentID: "a-1", Status: domain.ExperimentStatusDraft}
	expRepo.On("GetByID", mock.Anything, "e-1").Return(exp, nil)
	expRepo.On("UpdateStatus", mock.Anything, "e-1", domain.ExperimentStatusRunning,
		mock.AnythingOfType("*time.Time"), (*time.Time)(nil)).Return(nil)

	_, err := uc.Start(context.Background(), "acc-1", "e-1")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a-1"}, inv.called)
}

func TestExperiment_Stop_InvalidatesCache(t *testing.T) {
	planID := "plan-pro"
	account := &domain.Account{ID: "acc-1", PlanID: &planID}
	plan := newTestPlan("Pro")
	agent := &domain.Agent{ID: "a-1", AccountID: "acc-1"}
	uc, _, _, _, expRepo, _ := makeExperimentUC(t, account, plan, agent)
	inv := &fakeCacheInv{}
	uc.SetCacheInvalidator(inv)

	exp := &domain.PromptExperiment{ID: "e-1", AccountID: "acc-1", AgentID: "a-1", Status: domain.ExperimentStatusRunning}
	expRepo.On("GetByID", mock.Anything, "e-1").Return(exp, nil)
	expRepo.On("UpdateStatus", mock.Anything, "e-1", domain.ExperimentStatusStopped,
		(*time.Time)(nil), mock.AnythingOfType("*time.Time")).Return(nil)

	_, err := uc.Stop(context.Background(), "acc-1", "e-1")
	assert.NoError(t, err)
	assert.Equal(t, []string{"a-1"}, inv.called)
}

func TestExperiment_Stop_RejectsNonRunning(t *testing.T) {
	planID := "plan-pro"
	account := &domain.Account{ID: "acc-1", PlanID: &planID}
	plan := newTestPlan("Pro")
	uc, _, _, _, expRepo, _ := makeExperimentUC(t, account, plan, nil)

	exp := &domain.PromptExperiment{ID: "e-1", AccountID: "acc-1", Status: domain.ExperimentStatusDraft}
	expRepo.On("GetByID", mock.Anything, "e-1").Return(exp, nil)

	_, err := uc.Stop(context.Background(), "acc-1", "e-1")
	assert.ErrorIs(t, err, domain.ErrExperimentNotRunning)
}

func TestComputeLikelyWinner_BelowSampleSize(t *testing.T) {
	a := domain.VariantMetrics{ConversationCount: 10}
	b := domain.VariantMetrics{ConversationCount: 50}
	sig, w := computeLikelyWinner(a, b)
	assert.False(t, sig)
	assert.Nil(t, w)
}

func TestComputeLikelyWinner_AIResolutionGap(t *testing.T) {
	a := domain.VariantMetrics{ConversationCount: 100, AIResolvedCount: 80} // 80%
	b := domain.VariantMetrics{ConversationCount: 100, AIResolvedCount: 60} // 60%
	sig, w := computeLikelyWinner(a, b)
	assert.True(t, sig)
	if assert.NotNil(t, w) {
		assert.Equal(t, domain.VariantA, *w)
	}
}

func TestComputeLikelyWinner_CSATGapForB(t *testing.T) {
	csatA := 3.5
	csatB := 4.5
	a := domain.VariantMetrics{ConversationCount: 60, CSATAverage: &csatA, AIResolvedCount: 40}
	b := domain.VariantMetrics{ConversationCount: 60, CSATAverage: &csatB, AIResolvedCount: 40}
	sig, w := computeLikelyWinner(a, b)
	assert.True(t, sig)
	if assert.NotNil(t, w) {
		assert.Equal(t, domain.VariantB, *w)
	}
}
