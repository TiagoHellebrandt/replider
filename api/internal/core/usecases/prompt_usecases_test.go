package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockPromptRepo struct{ mock.Mock }

func (m *MockPromptRepo) Create(ctx context.Context, p *domain.Prompt) error {
	return m.Called(ctx, p).Error(0)
}
func (m *MockPromptRepo) GetByID(ctx context.Context, id string) (*domain.Prompt, error) {
	a := m.Called(ctx, id)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*domain.Prompt), a.Error(1)
}
func (m *MockPromptRepo) ListByAccountID(ctx context.Context, accountID string, limit, offset int) ([]*domain.Prompt, error) {
	a := m.Called(ctx, accountID, limit, offset)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).([]*domain.Prompt), a.Error(1)
}
func (m *MockPromptRepo) CountByAccountID(ctx context.Context, accountID string) (int64, error) {
	a := m.Called(ctx, accountID)
	return a.Get(0).(int64), a.Error(1)
}
func (m *MockPromptRepo) Update(ctx context.Context, p *domain.Prompt) error {
	return m.Called(ctx, p).Error(0)
}
func (m *MockPromptRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

type MockPromptVersionRepo struct{ mock.Mock }

func (m *MockPromptVersionRepo) Create(ctx context.Context, v *domain.PromptVersion) error {
	return m.Called(ctx, v).Error(0)
}
func (m *MockPromptVersionRepo) GetByID(ctx context.Context, id string) (*domain.PromptVersion, error) {
	a := m.Called(ctx, id)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*domain.PromptVersion), a.Error(1)
}
func (m *MockPromptVersionRepo) ListByAgentID(ctx context.Context, agentID string, limit, offset int) ([]*domain.PromptVersion, error) {
	a := m.Called(ctx, agentID, limit, offset)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).([]*domain.PromptVersion), a.Error(1)
}

// Inline transaction manager that runs the closure synchronously without
// touching the DB — good enough for unit-testing the apply/rollback
// orchestration.
type inlineTxManager struct{}

func (inlineTxManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func TestPromptUseCase_Create(t *testing.T) {
	ctx := context.Background()
	repo := new(MockPromptRepo)
	uc := NewPromptUseCase(repo)

	repo.On("Create", ctx, mock.MatchedBy(func(p *domain.Prompt) bool {
		return p.AccountID == "acc-1" && p.Name == "Clinic v1" && p.Source == domain.PromptSourceWizard && len(p.ID) == 26
	})).Return(nil)

	p, err := uc.Create(ctx, CreatePromptDTO{
		AccountID: "acc-1",
		UserID:    "u-1",
		Name:      "Clinic v1",
		Body:      "You are an assistant.",
		Source:    domain.PromptSourceWizard,
	})

	assert.NoError(t, err)
	assert.Equal(t, "Clinic v1", p.Name)
	repo.AssertExpectations(t)
}

func TestPromptUseCase_Create_RejectsBlankName(t *testing.T) {
	uc := NewPromptUseCase(new(MockPromptRepo))
	_, err := uc.Create(context.Background(), CreatePromptDTO{
		AccountID: "acc-1",
		Name:      "   ",
		Body:      "x",
	})
	assert.Error(t, err)
}

func TestPromptUseCase_Create_RejectsOversizedBody(t *testing.T) {
	uc := NewPromptUseCase(new(MockPromptRepo))
	huge := make([]byte, domain.MaxPromptBodyBytes+1)
	for i := range huge {
		huge[i] = 'x'
	}
	_, err := uc.Create(context.Background(), CreatePromptDTO{
		AccountID: "acc-1",
		Name:      "Big",
		Body:      string(huge),
	})
	assert.ErrorIs(t, err, domain.ErrPromptBodyTooLarge)
}

func TestPromptUseCase_Get_RejectsCrossAccount(t *testing.T) {
	ctx := context.Background()
	repo := new(MockPromptRepo)
	uc := NewPromptUseCase(repo)

	repo.On("GetByID", ctx, "p-1").Return(&domain.Prompt{ID: "p-1", AccountID: "acc-2"}, nil)

	_, err := uc.Get(ctx, "acc-1", "p-1")
	assert.ErrorIs(t, err, domain.ErrPromptNotFound)
}

func TestPromptUseCase_Duplicate_PrefixesName(t *testing.T) {
	ctx := context.Background()
	repo := new(MockPromptRepo)
	uc := NewPromptUseCase(repo)

	src := &domain.Prompt{ID: "p-1", AccountID: "acc-1", Name: "Clinic v1", Body: "B", Source: domain.PromptSourceWizard}
	repo.On("GetByID", ctx, "p-1").Return(src, nil)
	repo.On("Create", ctx, mock.MatchedBy(func(p *domain.Prompt) bool {
		return p.Name == "Copy of Clinic v1" && p.Source == domain.PromptSourceDuplicated
	})).Return(nil)

	out, err := uc.Duplicate(ctx, "acc-1", "u-1", "p-1")
	assert.NoError(t, err)
	assert.Equal(t, "Copy of Clinic v1", out.Name)
	repo.AssertExpectations(t)
}

func TestApplyPromptToAgent_AppliesByPromptID_AndSnapshotsPrevious(t *testing.T) {
	ctx := context.Background()
	prevBody := "old body"
	agent := &domain.Agent{ID: "a-1", AccountID: "acc-1", SystemPrompt: &prevBody}
	prompt := &domain.Prompt{ID: "p-1", AccountID: "acc-1", Body: "new body", Source: domain.PromptSourceWizard}

	agentRepo := new(MockAgentRepo)
	promptRepo := new(MockPromptRepo)
	versionRepo := new(MockPromptVersionRepo)
	auditRepo := new(MockAuditLogRepo)

	agentRepo.On("GetByID", ctx, "a-1").Return(agent, nil)
	promptRepo.On("GetByID", ctx, "p-1").Return(prompt, nil)
	// Snapshot row carries the *previous* body, not the new one.
	versionRepo.On("Create", ctx, mock.MatchedBy(func(v *domain.PromptVersion) bool {
		return v.AgentID == "a-1" && v.Body == prevBody && v.PromptID != nil && *v.PromptID == "p-1"
	})).Return(nil)
	agentRepo.On("Update", ctx, mock.MatchedBy(func(a *domain.Agent) bool {
		return a.SystemPrompt != nil && *a.SystemPrompt == "new body"
	})).Return(nil)
	auditRepo.On("Create", ctx, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	uc := NewApplyPromptToAgentUseCase(inlineTxManager{}, agentRepo, promptRepo, versionRepo, auditRepo)
	pid := "p-1"
	v, err := uc.Execute(ctx, ApplyPromptToAgentDTO{
		AccountID: "acc-1",
		UserID:    "u-1",
		AgentID:   "a-1",
		PromptID:  &pid,
	})
	assert.NoError(t, err)
	assert.Equal(t, prevBody, v.Body)
	versionRepo.AssertExpectations(t)
	agentRepo.AssertExpectations(t)
}

func TestApplyPromptToAgent_RejectsCrossAccountAgent(t *testing.T) {
	ctx := context.Background()
	agentRepo := new(MockAgentRepo)
	promptRepo := new(MockPromptRepo)
	versionRepo := new(MockPromptVersionRepo)
	auditRepo := new(MockAuditLogRepo)

	agentRepo.On("GetByID", ctx, "a-1").Return(&domain.Agent{ID: "a-1", AccountID: "acc-OTHER"}, nil)

	uc := NewApplyPromptToAgentUseCase(inlineTxManager{}, agentRepo, promptRepo, versionRepo, auditRepo)
	body := "x"
	_, err := uc.Execute(ctx, ApplyPromptToAgentDTO{
		AccountID: "acc-1",
		AgentID:   "a-1",
		Body:      &body,
	})
	assert.ErrorIs(t, err, domain.ErrAgentNotFound)
}

func TestApplyPromptToAgent_Rollback_RestoresSnapshot(t *testing.T) {
	ctx := context.Background()
	current := "current body"
	agent := &domain.Agent{ID: "a-1", AccountID: "acc-1", SystemPrompt: &current}
	target := &domain.PromptVersion{ID: "v-1", AgentID: "a-1", Body: "older body"}

	agentRepo := new(MockAgentRepo)
	promptRepo := new(MockPromptRepo)
	versionRepo := new(MockPromptVersionRepo)
	auditRepo := new(MockAuditLogRepo)

	agentRepo.On("GetByID", ctx, "a-1").Return(agent, nil)
	versionRepo.On("GetByID", ctx, "v-1").Return(target, nil)
	// Rollback writes a fresh audit row capturing the body that was
	// active *before* the rollback (so we can roll forward later).
	versionRepo.On("Create", ctx, mock.MatchedBy(func(v *domain.PromptVersion) bool {
		return v.AgentID == "a-1" && v.Body == current
	})).Return(nil)
	agentRepo.On("Update", ctx, mock.MatchedBy(func(a *domain.Agent) bool {
		return a.SystemPrompt != nil && *a.SystemPrompt == "older body"
	})).Return(nil)
	auditRepo.On("Create", ctx, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	uc := NewApplyPromptToAgentUseCase(inlineTxManager{}, agentRepo, promptRepo, versionRepo, auditRepo)
	_, err := uc.Rollback(ctx, "acc-1", "u-1", "a-1", "v-1")
	assert.NoError(t, err)
	versionRepo.AssertExpectations(t)
	agentRepo.AssertExpectations(t)
}
