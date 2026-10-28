package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateAgentUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should successfully create an agent when account has none", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewCreateAgentUseCase(repo, nil, nil)

		repo.On("ListByAccountID", ctx, "acc_123").Return([]*domain.Agent{}, nil)
		repo.On("Create", ctx, mock.MatchedBy(func(a *domain.Agent) bool {
			return a.Name == "Test Agent" && a.AccountID == "acc_123" && len(a.ID) == 26
		})).Return(nil)

		agent, err := uc.Execute(ctx, CreateAgentDTO{
			AccountID: "acc_123",
			Name:      "Test Agent",
		})

		assert.NoError(t, err)
		assert.NotNil(t, agent)
		assert.Equal(t, "Test Agent", agent.Name)
		repo.AssertExpectations(t)
	})

	t.Run("should return PLAN_LIMIT_EXCEEDED when account hit plan.MaxAgents", func(t *testing.T) {
		repo := new(MockAgentRepo)
		// accountRepo/planRepo nil → use case falls back to default limit of 1
		// (trial-safe behaviour when plan isn't resolvable).
		uc := NewCreateAgentUseCase(repo, nil, nil)

		repo.On("ListByAccountID", ctx, "acc_123").Return([]*domain.Agent{
			{ID: "existing_agent", AccountID: "acc_123", Name: "Existing"},
		}, nil)

		agent, err := uc.Execute(ctx, CreateAgentDTO{
			AccountID: "acc_123",
			Name:      "Second Agent",
		})

		assert.Nil(t, agent)
		appErr, ok := err.(*domain.AppError)
		assert.True(t, ok, "expected *domain.AppError")
		assert.Equal(t, "PLAN_LIMIT_EXCEEDED", appErr.Code)
		assert.Equal(t, "agents", appErr.Metadata["resource"])
		assert.Equal(t, 1, appErr.Metadata["current"])
		assert.Equal(t, 1, appErr.Metadata["limit"])
		repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		repo.AssertExpectations(t)
	})

	t.Run("should honour plan.MaxAgents when repos are wired", func(t *testing.T) {
		role := "admin"
		_ = role
		repo := new(MockAgentRepo)
		accountRepo := new(MockAccountRepo)
		planRepo := new(MockPlanRepo)

		planID := "plan_pro"
		accountRepo.On("GetByID", ctx, "acc_123").Return(&domain.Account{
			ID: "acc_123", PlanID: &planID,
		}, nil)
		planRepo.On("GetByID", ctx, planID).Return(&domain.Plan{
			ID: planID, MaxAgents: 3,
		}, nil)
		repo.On("ListByAccountID", ctx, "acc_123").Return([]*domain.Agent{
			{ID: "a1"}, {ID: "a2"},
		}, nil)
		repo.On("Create", ctx, mock.Anything).Return(nil)

		uc := NewCreateAgentUseCase(repo, accountRepo, planRepo)
		agent, err := uc.Execute(ctx, CreateAgentDTO{AccountID: "acc_123", Name: "Third"})

		assert.NoError(t, err)
		assert.NotNil(t, agent)
	})

	t.Run("Mission 7: template_id fills name + applies placeholders to system_prompt", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewCreateAgentUseCase(repo, nil, nil)

		repo.On("ListByAccountID", ctx, "acc_t").Return([]*domain.Agent{}, nil)
		var captured *domain.Agent
		repo.On("Create", ctx, mock.MatchedBy(func(a *domain.Agent) bool {
			captured = a
			return true
		})).Return(nil)

		_, err := uc.Execute(ctx, CreateAgentDTO{
			AccountID:  "acc_t",
			Name:       "", // empty → template name kicks in
			TemplateID: "clinic",
			Placeholders: map[string]string{
				"NOME_DA_EMPRESA":     "Clínica Luz",
				"HORARIO_ATENDIMENTO": "seg-sex 9-18h",
			},
		})
		assert.NoError(t, err)
		// Name auto-filled from template.
		assert.Equal(t, "Clara, assistente da clínica", captured.Name)
		// System prompt populated and placeholders substituted.
		if assert.NotNil(t, captured.SystemPrompt) {
			assert.Contains(t, *captured.SystemPrompt, "Clínica Luz")
			assert.Contains(t, *captured.SystemPrompt, "seg-sex 9-18h")
			assert.NotContains(t, *captured.SystemPrompt, "{{NOME_DA_EMPRESA}}")
		}
	})

	t.Run("Mission 7: caller-supplied name + system_prompt beat the template", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewCreateAgentUseCase(repo, nil, nil)

		repo.On("ListByAccountID", ctx, "acc_t2").Return([]*domain.Agent{}, nil)
		var captured *domain.Agent
		repo.On("Create", ctx, mock.MatchedBy(func(a *domain.Agent) bool {
			captured = a
			return true
		})).Return(nil)

		custom := "prompt customizado pelo usuário no modo avançado"
		_, err := uc.Execute(ctx, CreateAgentDTO{
			AccountID:    "acc_t2",
			Name:         "Meu Atendente",
			SystemPrompt: &custom,
			TemplateID:   "clinic",
			Placeholders: map[string]string{"NOME_DA_EMPRESA": "X"},
		})
		assert.NoError(t, err)
		assert.Equal(t, "Meu Atendente", captured.Name)
		assert.Equal(t, custom, *captured.SystemPrompt)
	})

	t.Run("Mission 7: unknown template_id returns TEMPLATE_NOT_FOUND", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewCreateAgentUseCase(repo, nil, nil)

		repo.On("ListByAccountID", ctx, "acc_t3").Return([]*domain.Agent{}, nil)

		_, err := uc.Execute(ctx, CreateAgentDTO{
			AccountID:  "acc_t3",
			Name:       "n",
			TemplateID: "nonexistent",
		})
		appErr, ok := err.(*domain.AppError)
		assert.True(t, ok)
		assert.Equal(t, "TEMPLATE_NOT_FOUND", appErr.Code)
		repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})
}
