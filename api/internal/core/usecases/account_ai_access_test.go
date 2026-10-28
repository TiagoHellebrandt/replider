package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

// Trial expiry outside HTTP: the middleware refused an expired trial on the
// dashboard while the webhook path, the overnight drafts and the MCP mount
// kept spending. This is the one rule they now share.

func TestAccountMayUseAI_TrialExpiredIsRefused(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	yesterday := now.Add(-24 * time.Hour)
	acc := &domain.Account{ID: "acc", SubscriptionStatus: domain.SubscriptionStatusTrialing, TrialExpiresAt: &yesterday}

	err := accountMayUseAI(acc, now)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTrialExpired)
}

func TestAccountMayUseAI_TrialStillRunningIsAllowed(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	tomorrow := now.Add(24 * time.Hour)
	acc := &domain.Account{ID: "acc", SubscriptionStatus: domain.SubscriptionStatusTrialing, TrialExpiresAt: &tomorrow}

	assert.NoError(t, accountMayUseAI(acc, now))
}

func TestAccountMayUseAI_TrialBypassWinsOverExpiry(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	yesterday := now.Add(-24 * time.Hour)
	acc := &domain.Account{ID: "acc", SubscriptionStatus: domain.SubscriptionStatusTrialing,
		TrialExpiresAt: &yesterday, TrialBypass: true}

	assert.NoError(t, accountMayUseAI(acc, now))
}

func TestAccountMayUseAI_SuspendedIsRefusedFirst(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	acc := &domain.Account{ID: "acc", SubscriptionStatus: domain.SubscriptionStatusActive,
		SuspendedAt: &now, TrialBypass: true}

	assert.ErrorIs(t, accountMayUseAI(acc, now), domain.ErrAccountSuspended)
}

func TestAccountMayUseAI_CanceledIsInactive(t *testing.T) {
	acc := &domain.Account{ID: "acc", SubscriptionStatus: "canceled"}
	assert.ErrorIs(t, accountMayUseAI(acc, time.Now()), ErrSubscriptionInactive)
}

func TestAccountMayUseAI_LookupFailureRefuses(t *testing.T) {
	accounts := &MockAccountRepo{}
	accounts.On("GetByID", mock.Anything, "acc").Return((*domain.Account)(nil), errors.New("db down"))

	err := AccountMayUseAI(context.Background(), accounts, "acc")

	require.Error(t, err, "a billing boundary does not err open")
}

func TestAccountMayUseAI_ExportedHelperAppliesTheRule(t *testing.T) {
	yesterday := time.Now().Add(-24 * time.Hour)
	accounts := &MockAccountRepo{}
	accounts.On("GetByID", mock.Anything, "acc").Return(&domain.Account{
		ID: "acc", SubscriptionStatus: domain.SubscriptionStatusTrialing, TrialExpiresAt: &yesterday,
	}, nil)

	assert.ErrorIs(t, AccountMayUseAI(context.Background(), accounts, "acc"), ErrTrialExpired)
}
