package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestGetOnboardingProgress_AllMilestonesComplete(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)

	// The six the percentage counts are the SALES milestones. The bot flags are
	// set here too, and deliberately do not move the number: an account with a
	// chatbot and nothing to sell has not started.
	accountRepo.On("GetOnboardingProgress", ctx, "acc_1").Return(&domain.OnboardingProgress{
		AgentCreated:         true,
		PreviewChatTested:    true,
		KnowledgeAdded:       true,
		ChannelConnected:     true,
		ProductsAdded:        true,
		CustomersAdded:       true,
		ProposalMade:         true,
		FirstInboundReceived: true,
		TeamInvited:          true,
	}, nil)
	accountRepo.On("GetByID", ctx, "acc_1").Return(&domain.Account{
		ID:   "acc_1",
		Name: "Acme",
	}, nil)

	uc := NewGetOnboardingProgressUseCase(accountRepo)
	got, err := uc.Execute(ctx, "acc_1")

	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, 100, got.CompletionPercentage)
	assert.Equal(t, "Acme", got.AccountName)
	accountRepo.AssertExpectations(t)
}

func TestGetOnboardingProgress_PartialProgress(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)

	// 2 of the 6 SALES milestones done → 33%. The agent and preview flags are
	// set as well and count for nothing, which is the point of the change.
	accountRepo.On("GetOnboardingProgress", ctx, "acc_2").Return(&domain.OnboardingProgress{
		AgentCreated:      true,
		PreviewChatTested: true,
		ChannelConnected:  true,
		ProductsAdded:     true,
	}, nil)
	accountRepo.On("GetByID", ctx, "acc_2").Return(&domain.Account{
		ID:   "acc_2",
		Name: "Acme",
	}, nil)

	uc := NewGetOnboardingProgressUseCase(accountRepo)
	got, err := uc.Execute(ctx, "acc_2")

	assert.NoError(t, err)
	assert.Equal(t, 33, got.CompletionPercentage)
	// The bot flags still travel — they are real facts and the `full` cohort
	// renders them — they simply do not move the number.
	assert.True(t, got.AgentCreated)
	assert.True(t, got.PreviewChatTested)
	assert.True(t, got.ChannelConnected)
	assert.True(t, got.ProductsAdded)
	assert.False(t, got.CustomersAdded)
	assert.False(t, got.ProposalMade)
}

func TestGetOnboardingProgress_AccountNotFound(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)

	accountRepo.On("GetOnboardingProgress", ctx, "missing").Return(nil, nil)

	uc := NewGetOnboardingProgressUseCase(accountRepo)
	got, err := uc.Execute(ctx, "missing")

	assert.Nil(t, got)
	assert.Equal(t, domain.ErrAccountNotFound, err)
}

func TestGetOnboardingProgress_RepoError(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)

	accountRepo.On("GetOnboardingProgress", ctx, "acc_3").Return(nil, errors.New("db down"))

	uc := NewGetOnboardingProgressUseCase(accountRepo)
	_, err := uc.Execute(ctx, "acc_3")

	assert.Error(t, err)
}
