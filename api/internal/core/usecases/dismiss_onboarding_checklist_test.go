package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDismissOnboardingChecklist_PassesNowToRepo(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)

	// We don't pin the exact timestamp because it's `time.Now()` inside
	// the use case; just assert the contract: account_id forwarded and
	// timestamp is non-zero, recent.
	accountRepo.On("DismissOnboardingChecklist", ctx, "acc_1", mock.MatchedBy(func(at time.Time) bool {
		return !at.IsZero() && time.Since(at) < time.Second
	})).Return(nil)

	uc := NewDismissOnboardingChecklistUseCase(accountRepo)
	err := uc.Execute(ctx, "acc_1")

	assert.NoError(t, err)
	accountRepo.AssertExpectations(t)
}
