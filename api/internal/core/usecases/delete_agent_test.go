package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDeleteAgentUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should delete agent owned by account", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewDeleteAgentUseCase(repo)

		repo.On("GetByID", ctx, "agent_1").Return(&domain.Agent{
			ID:        "agent_1",
			AccountID: "acc_123",
			Name:      "My Agent",
		}, nil)
		repo.On("Delete", ctx, "agent_1").Return(nil)

		err := uc.Execute(ctx, "acc_123", "agent_1")

		assert.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("should return ErrAgentNotFound when agent does not exist", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewDeleteAgentUseCase(repo)

		repo.On("GetByID", ctx, "nonexistent").Return(nil, nil)

		err := uc.Execute(ctx, "acc_123", "nonexistent")

		assert.ErrorIs(t, err, domain.ErrAgentNotFound)
		repo.AssertExpectations(t)
	})

	t.Run("should return ErrAgentNotFound when agent belongs to different account", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewDeleteAgentUseCase(repo)

		repo.On("GetByID", ctx, "agent_1").Return(&domain.Agent{
			ID:        "agent_1",
			AccountID: "acc_other",
			Name:      "Their Agent",
		}, nil)

		err := uc.Execute(ctx, "acc_123", "agent_1")

		assert.ErrorIs(t, err, domain.ErrAgentNotFound)
		repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
		repo.AssertExpectations(t)
	})

	t.Run("should propagate repo error from GetByID", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewDeleteAgentUseCase(repo)

		repo.On("GetByID", ctx, "agent_1").Return(nil, errors.New("db connection lost"))

		err := uc.Execute(ctx, "acc_123", "agent_1")

		assert.Error(t, err)
		assert.Equal(t, "db connection lost", err.Error())
		repo.AssertExpectations(t)
	})

	t.Run("should propagate repo error from Delete", func(t *testing.T) {
		repo := new(MockAgentRepo)
		uc := NewDeleteAgentUseCase(repo)

		repo.On("GetByID", ctx, "agent_1").Return(&domain.Agent{
			ID:        "agent_1",
			AccountID: "acc_123",
			Name:      "My Agent",
		}, nil)
		repo.On("Delete", ctx, "agent_1").Return(errors.New("delete failed"))

		err := uc.Execute(ctx, "acc_123", "agent_1")

		assert.Error(t, err)
		assert.Equal(t, "delete failed", err.Error())
		repo.AssertExpectations(t)
	})
}
