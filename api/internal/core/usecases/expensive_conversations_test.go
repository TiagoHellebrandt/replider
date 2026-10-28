package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetExpensiveConversations(t *testing.T) {
	ctx := context.Background()
	repo := new(MockAnalyticsRepo)
	uc := NewGetExpensiveConversationsUseCase(repo)

	want := []*domain.ExpensiveConversation{
		{ConversationID: "conv-1", ContactID: "+551199", TotalCostUSD: 1.23},
		{ConversationID: "conv-2", ContactID: "+551188", TotalCostUSD: 0.45},
	}
	// Use case must request exactly the top-10 cap.
	repo.On("GetTopExpensiveConversations", ctx, "acc-1", 10).Return(want, nil)

	got, err := uc.Execute(ctx, "acc-1")
	assert.NoError(t, err)
	assert.Equal(t, want, got)
	repo.AssertExpectations(t)
}

func TestGetExpensiveConversations_PassesLimit(t *testing.T) {
	repo := new(MockAnalyticsRepo)
	uc := NewGetExpensiveConversationsUseCase(repo)
	repo.On("GetTopExpensiveConversations", mock.Anything, "acc-x", TopExpensiveConversationsLimit).
		Return([]*domain.ExpensiveConversation{}, nil)

	_, err := uc.Execute(context.Background(), "acc-x")
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
