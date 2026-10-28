package usecases

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

// GetExpensiveConversationsUseCase serves the US-030 cost-attribution
// dashboard: the account's conversations ranked by accumulated estimated
// AI provider cost. Thin wrapper around the repo — the only policy is the
// fixed top-10 cap per the acceptance criteria.
type GetExpensiveConversationsUseCase struct {
	repo domain.AnalyticsRepository
}

func NewGetExpensiveConversationsUseCase(repo domain.AnalyticsRepository) *GetExpensiveConversationsUseCase {
	return &GetExpensiveConversationsUseCase{repo: repo}
}

// TopExpensiveConversationsLimit is the fixed result cap (AC: "top 10").
const TopExpensiveConversationsLimit = 10

func (uc *GetExpensiveConversationsUseCase) Execute(ctx context.Context, accountID string) ([]*domain.ExpensiveConversation, error) {
	return uc.repo.GetTopExpensiveConversations(ctx, accountID, TopExpensiveConversationsLimit)
}
