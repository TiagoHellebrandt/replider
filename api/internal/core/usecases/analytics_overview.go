package usecases

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

// GetAnalyticsOverviewUseCase serves the Mission 6 operational MVP
// payload. Dead-simple wrapper around the repo so the only policy
// here is "clamp the day window to {7, 30, 90}" — everything else is
// a SQL aggregation.
type GetAnalyticsOverviewUseCase struct {
	repo domain.AnalyticsRepository
}

func NewGetAnalyticsOverviewUseCase(repo domain.AnalyticsRepository) *GetAnalyticsOverviewUseCase {
	return &GetAnalyticsOverviewUseCase{repo: repo}
}

// allowedWindows locks the period selector to 7 / 30 / 90 days so
// ad-hoc values (1, 365, -1) can't be used to sidestep query caches
// or probe different windows. Anything outside snaps to 30.
var allowedWindows = map[int]struct{}{7: {}, 30: {}, 90: {}}

func (uc *GetAnalyticsOverviewUseCase) Execute(ctx context.Context, accountID string, days int) (*domain.AnalyticsOverview, error) {
	if _, ok := allowedWindows[days]; !ok {
		days = 30
	}
	return uc.repo.GetOverview(ctx, accountID, days)
}
