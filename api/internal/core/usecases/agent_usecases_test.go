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

// --- ListAgentsUseCase ---

func TestListAgentsUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should return agents for account", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewListAgentsUseCase(repo)

		expected := []*domain.Agent{
			{ID: "agent_1", AccountID: "acc_123", Name: "Agent One"},
			{ID: "agent_2", AccountID: "acc_123", Name: "Agent Two"},
		}
		repo.On("ListByAccountID", ctx, "acc_123").Return(expected, nil)

		agents, err := uc.Execute(ctx, "acc_123")

		assert.NoError(t, err)
		assert.Len(t, agents, 2)
		assert.Equal(t, "Agent One", agents[0].Name)
		assert.Equal(t, "Agent Two", agents[1].Name)
		repo.AssertExpectations(t)
	})

	t.Run("should return empty list when account has no agents", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewListAgentsUseCase(repo)

		repo.On("ListByAccountID", ctx, "acc_456").Return([]*domain.Agent{}, nil)

		agents, err := uc.Execute(ctx, "acc_456")

		assert.NoError(t, err)
		assert.Empty(t, agents)
		repo.AssertExpectations(t)
	})

	t.Run("should propagate repository error", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewListAgentsUseCase(repo)

		repo.On("ListByAccountID", ctx, "acc_123").Return([]*domain.Agent(nil), errors.New("db error"))

		agents, err := uc.Execute(ctx, "acc_123")

		assert.Error(t, err)
		assert.Nil(t, agents)
		repo.AssertExpectations(t)
	})
}

// --- UpdateAgentUseCase ---

func TestUpdateAgentUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should successfully update an agent", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewUpdateAgentUseCase(repo)

		prompt := "You are helpful"
		existing := &domain.Agent{
			ID:        "agent_1",
			AccountID: "acc_123",
			Name:      "Old Name",
			Provider:  "openai",
			Model:     "gpt-4",
			Timezone:  "UTC",
			IsActive:  true,
			CreatedAt: time.Now().Add(-24 * time.Hour),
			UpdatedAt: time.Now().Add(-1 * time.Hour),
		}

		repo.On("GetByID", ctx, "agent_1").Return(existing, nil)
		repo.On("Update", ctx, mock.MatchedBy(func(a *domain.Agent) bool {
			return a.Name == "New Name" &&
				a.Provider == "google" &&
				a.Model == "gemini-pro" &&
				a.Timezone == "America/Sao_Paulo" &&
				a.SystemPrompt != nil && *a.SystemPrompt == "You are helpful"
		})).Return(nil)

		newPrompt := "You are helpful"
		agent, err := uc.Execute(ctx, UpdateAgentDTO{
			ID:           "agent_1",
			AccountID:    "acc_123",
			Name:         "New Name",
			SystemPrompt: &newPrompt,
			Provider:     "google",
			Model:        "gemini-pro",
			Timezone:     "America/Sao_Paulo",
		})

		assert.NoError(t, err)
		assert.NotNil(t, agent)
		assert.Equal(t, "New Name", agent.Name)
		assert.Equal(t, "google", agent.Provider)
		assert.Equal(t, "gemini-pro", agent.Model)
		assert.Equal(t, "America/Sao_Paulo", agent.Timezone)
		assert.Equal(t, &prompt, agent.SystemPrompt)
		repo.AssertExpectations(t)
	})

	t.Run("should return ErrAgentNotFound when agent does not exist", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewUpdateAgentUseCase(repo)

		repo.On("GetByID", ctx, "nonexistent").Return(nil, nil)

		agent, err := uc.Execute(ctx, UpdateAgentDTO{
			ID:        "nonexistent",
			AccountID: "acc_123",
			Name:      "Name",
		})

		assert.Nil(t, agent)
		assert.ErrorIs(t, err, domain.ErrAgentNotFound)
		repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
		repo.AssertExpectations(t)
	})

	t.Run("should return ErrAgentNotFound when agent belongs to different account", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewUpdateAgentUseCase(repo)

		existing := &domain.Agent{
			ID:        "agent_1",
			AccountID: "acc_other",
			Name:      "Agent",
		}
		repo.On("GetByID", ctx, "agent_1").Return(existing, nil)

		agent, err := uc.Execute(ctx, UpdateAgentDTO{
			ID:        "agent_1",
			AccountID: "acc_123",
			Name:      "Hacked",
		})

		assert.Nil(t, agent)
		assert.ErrorIs(t, err, domain.ErrAgentNotFound)
		repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
		repo.AssertExpectations(t)
	})

	t.Run("should keep existing provider, model and timezone when dto fields are empty", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewUpdateAgentUseCase(repo)

		existing := &domain.Agent{
			ID:        "agent_1",
			AccountID: "acc_123",
			Name:      "Old Name",
			Provider:  "openai",
			Model:     "gpt-4",
			Timezone:  "UTC",
			IsActive:  true,
		}

		repo.On("GetByID", ctx, "agent_1").Return(existing, nil)
		repo.On("Update", ctx, mock.MatchedBy(func(a *domain.Agent) bool {
			return a.Name == "Updated Name" &&
				a.Provider == "openai" &&
				a.Model == "gpt-4" &&
				a.Timezone == "UTC"
		})).Return(nil)

		agent, err := uc.Execute(ctx, UpdateAgentDTO{
			ID:        "agent_1",
			AccountID: "acc_123",
			Name:      "Updated Name",
			Provider:  "",
			Model:     "",
			Timezone:  "",
		})

		assert.NoError(t, err)
		assert.NotNil(t, agent)
		assert.Equal(t, "openai", agent.Provider)
		assert.Equal(t, "gpt-4", agent.Model)
		assert.Equal(t, "UTC", agent.Timezone)
		repo.AssertExpectations(t)
	})

	t.Run("should propagate repository error on update", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewUpdateAgentUseCase(repo)

		existing := &domain.Agent{
			ID:        "agent_1",
			AccountID: "acc_123",
			Name:      "Agent",
		}

		repo.On("GetByID", ctx, "agent_1").Return(existing, nil)
		repo.On("Update", ctx, mock.Anything).Return(errors.New("db error"))

		agent, err := uc.Execute(ctx, UpdateAgentDTO{
			ID:        "agent_1",
			AccountID: "acc_123",
			Name:      "Updated",
		})

		assert.Nil(t, agent)
		assert.Error(t, err)
		assert.Equal(t, "db error", err.Error())
		repo.AssertExpectations(t)
	})
}

// --- ToggleAgentStatusUseCase ---

func TestToggleAgentStatusUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should toggle agent from active to inactive", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewToggleAgentStatusUseCase(repo)

		existing := &domain.Agent{
			ID:        "agent_1",
			AccountID: "acc_123",
			IsActive:  true,
		}

		repo.On("GetByID", ctx, "agent_1").Return(existing, nil)
		repo.On("Update", ctx, mock.MatchedBy(func(a *domain.Agent) bool {
			return a.ID == "agent_1" && !a.IsActive
		})).Return(nil)

		err := uc.Execute(ctx, "acc_123", "agent_1", false)

		assert.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("should toggle agent from inactive to active", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewToggleAgentStatusUseCase(repo)

		existing := &domain.Agent{
			ID:        "agent_1",
			AccountID: "acc_123",
			IsActive:  false,
		}

		repo.On("GetByID", ctx, "agent_1").Return(existing, nil)
		repo.On("Update", ctx, mock.MatchedBy(func(a *domain.Agent) bool {
			return a.ID == "agent_1" && a.IsActive
		})).Return(nil)

		err := uc.Execute(ctx, "acc_123", "agent_1", true)

		assert.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("should return ErrAgentNotFound when agent does not exist", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewToggleAgentStatusUseCase(repo)

		repo.On("GetByID", ctx, "nonexistent").Return(nil, nil)

		err := uc.Execute(ctx, "acc_123", "nonexistent", true)

		assert.ErrorIs(t, err, domain.ErrAgentNotFound)
		repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
		repo.AssertExpectations(t)
	})

	t.Run("should return ErrAgentNotFound when agent belongs to different account", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewToggleAgentStatusUseCase(repo)

		existing := &domain.Agent{
			ID:        "agent_1",
			AccountID: "acc_other",
			IsActive:  true,
		}
		repo.On("GetByID", ctx, "agent_1").Return(existing, nil)

		err := uc.Execute(ctx, "acc_123", "agent_1", false)

		assert.ErrorIs(t, err, domain.ErrAgentNotFound)
		repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
		repo.AssertExpectations(t)
	})

	t.Run("should propagate repository error on update", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewToggleAgentStatusUseCase(repo)

		existing := &domain.Agent{
			ID:        "agent_1",
			AccountID: "acc_123",
			IsActive:  true,
		}

		repo.On("GetByID", ctx, "agent_1").Return(existing, nil)
		repo.On("Update", ctx, mock.Anything).Return(errors.New("db error"))

		err := uc.Execute(ctx, "acc_123", "agent_1", false)

		assert.Error(t, err)
		assert.Equal(t, "db error", err.Error())
		repo.AssertExpectations(t)
	})
}

// Production, 2026-09-09: four of six agents carried provider=google with
// model=gpt-4o. The two defaults were independent constants, so asking for
// google without naming a model handed back an OpenAI one.
func TestCreateAgent_DefaultModelFollowsTheProvider(t *testing.T) {
	for _, c := range []struct{ provider, want string }{
		{"google", domain.DefaultGoogleModel},
		{"gemini", domain.DefaultGoogleModel},
		{"openai", domain.DefaultOpenAIModel},
		{"", domain.DefaultOpenAIModel},
	} {
		got := c.provider
		if got == "" {
			got = domain.AIProviderOpenAI
		}
		model := domain.DefaultModelFor(got)
		if model != c.want {
			t.Fatalf("provider %q defaulted to %q, want %q", c.provider, model, c.want)
		}
		if err := domain.ValidateModelForProvider(got, model); err != nil {
			t.Fatalf("provider %q default %q does not pair: %v", c.provider, model, err)
		}
	}
}
