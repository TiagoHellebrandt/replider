package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestEnsureInternalChannelUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("creates the internal channel when none exists", func(t *testing.T) {
		repo := &MockChannelRepo{}
		repo.On("ListByAccountID", ctx, "platform1").Return([]*domain.Channel{
			{ID: "ch1", AccountID: "platform1", Type: "whatsapp"},
		}, nil)
		repo.On("Create", ctx, mock.AnythingOfType("*domain.Channel")).Return(nil)

		uc := NewEnsureInternalChannelUseCase(repo)
		ch, err := uc.Execute(ctx, "platform1")

		assert.NoError(t, err)
		assert.NotNil(t, ch)
		assert.Equal(t, domain.ChannelTypeInternal, ch.Type)
		assert.Equal(t, "platform1", ch.AccountID)
		assert.True(t, ch.IsActive)
		assert.NotEmpty(t, ch.WebhookToken)
		repo.AssertExpectations(t)
	})

	t.Run("is idempotent — returns the existing internal channel without creating", func(t *testing.T) {
		existing := &domain.Channel{ID: "chX", AccountID: "platform1", Type: domain.ChannelTypeInternal}
		repo := &MockChannelRepo{}
		repo.On("ListByAccountID", ctx, "platform1").Return([]*domain.Channel{existing}, nil)

		uc := NewEnsureInternalChannelUseCase(repo)
		ch, err := uc.Execute(ctx, "platform1")

		assert.NoError(t, err)
		assert.Same(t, existing, ch)
		repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	t.Run("no-op when platform account id is unset", func(t *testing.T) {
		repo := &MockChannelRepo{}

		uc := NewEnsureInternalChannelUseCase(repo)
		ch, err := uc.Execute(ctx, "")

		assert.NoError(t, err)
		assert.Nil(t, ch)
		repo.AssertNotCalled(t, "ListByAccountID", mock.Anything, mock.Anything)
	})
}

func TestCreateChannelUseCase_RejectsInternalType(t *testing.T) {
	ctx := context.Background()
	repo := &MockChannelRepo{}
	agentRepo := new(MockAgentRepo)

	uc := NewCreateChannelUseCase(repo, agentRepo, nil, nil)
	ch, err := uc.Execute(ctx, CreateChannelDTO{
		AccountID: "acc1",
		Type:      domain.ChannelTypeInternal,
		Name:      "Sneaky",
	})

	assert.Nil(t, ch)
	var appErr *domain.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestDeleteChannelUseCase_RejectsInternalType(t *testing.T) {
	ctx := context.Background()
	repo := &MockChannelRepo{}
	repo.On("GetByID", ctx, "chI").Return(&domain.Channel{
		ID: "chI", AccountID: "platform1", Type: domain.ChannelTypeInternal,
	}, nil)

	uc := NewDeleteChannelUseCase(repo)
	err := uc.Execute(ctx, "platform1", "chI")

	var appErr *domain.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}
