package usecases

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

type DismissOnboardingChecklistUseCase struct {
	accountRepo domain.AccountRepository
}

func NewDismissOnboardingChecklistUseCase(accountRepo domain.AccountRepository) *DismissOnboardingChecklistUseCase {
	return &DismissOnboardingChecklistUseCase{accountRepo: accountRepo}
}

func (uc *DismissOnboardingChecklistUseCase) Execute(ctx context.Context, accountID string) error {
	return uc.accountRepo.DismissOnboardingChecklist(ctx, accountID, time.Now())
}
