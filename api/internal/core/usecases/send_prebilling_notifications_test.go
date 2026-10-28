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

// fixedNow pins "today" so the days-until math is deterministic.
var fixedNow = time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)

func newPrebillingUC(acc *MockAccountRepo, usr *MockUserRepo, pln *MockPlanRepo, mail *MockEmailService) *SendPrebillingNotificationsUseCase {
	return NewSendPrebillingNotificationsUseCase(acc, usr, pln, mail, "https://app.replider.com").
		WithClock(func() time.Time { return fixedNow })
}

func TestSendPrebilling_SendsAndMarksNotified(t *testing.T) {
	acc := new(MockAccountRepo)
	usr := new(MockUserRepo)
	pln := new(MockPlanRepo)
	mail := new(MockEmailService)

	planID := "plan_pro"
	periodEnd := fixedNow.Add(50 * time.Hour) // ~3 days out → "renews in 3 days"
	due := []*domain.PrebillingAccount{{AccountID: "acc_1", PlanID: &planID, PeriodEnd: periodEnd}}

	acc.On("ListAccountsDueForPrebilling", mock.Anything, mock.Anything).Return(due, nil)
	usr.On("GetAdminByAccountID", mock.Anything, "acc_1").
		Return(&domain.User{Email: "admin@acme.com", Name: "Ada"}, nil)
	pln.On("GetByID", mock.Anything, "plan_pro").Return(&domain.Plan{ID: "plan_pro", Name: "Pro"}, nil)
	mail.On("SendPrebillingEmail", "admin@acme.com", "Ada", "Pro", 3, "https://app.replider.com/settings?billing=manage", "").
		Return(nil)
	acc.On("MarkPrebillingNotified", mock.Anything, "acc_1", periodEnd).Return(nil)

	sent, err := newPrebillingUC(acc, usr, pln, mail).Execute(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 1, sent)
	mail.AssertExpectations(t)
	acc.AssertCalled(t, "MarkPrebillingNotified", mock.Anything, "acc_1", periodEnd)
}

func TestSendPrebilling_UsesWindowAsCutoff(t *testing.T) {
	acc := new(MockAccountRepo)
	usr := new(MockUserRepo)
	pln := new(MockPlanRepo)
	mail := new(MockEmailService)

	// Cutoff passed to the repo must be now + 72h (the default window).
	acc.On("ListAccountsDueForPrebilling", mock.Anything, fixedNow.Add(72*time.Hour)).
		Return([]*domain.PrebillingAccount{}, nil)

	sent, err := newPrebillingUC(acc, usr, pln, mail).Execute(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 0, sent)
	acc.AssertExpectations(t)
}

func TestSendPrebilling_SkipsWhenNoAdmin_NotMarked(t *testing.T) {
	acc := new(MockAccountRepo)
	usr := new(MockUserRepo)
	pln := new(MockPlanRepo)
	mail := new(MockEmailService)

	due := []*domain.PrebillingAccount{{AccountID: "acc_1", PeriodEnd: fixedNow.Add(24 * time.Hour)}}
	acc.On("ListAccountsDueForPrebilling", mock.Anything, mock.Anything).Return(due, nil)
	usr.On("GetAdminByAccountID", mock.Anything, "acc_1").Return((*domain.User)(nil), nil)

	sent, err := newPrebillingUC(acc, usr, pln, mail).Execute(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 0, sent)
	// No admin → no send, and crucially NOT marked (so it retries next tick).
	mail.AssertNotCalled(t, "SendPrebillingEmail", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	acc.AssertNotCalled(t, "MarkPrebillingNotified", mock.Anything, mock.Anything, mock.Anything)
}

func TestSendPrebilling_SendFailure_NotMarked(t *testing.T) {
	acc := new(MockAccountRepo)
	usr := new(MockUserRepo)
	pln := new(MockPlanRepo)
	mail := new(MockEmailService)

	planID := "plan_pro"
	due := []*domain.PrebillingAccount{{AccountID: "acc_1", PlanID: &planID, PeriodEnd: fixedNow.Add(24 * time.Hour)}}
	acc.On("ListAccountsDueForPrebilling", mock.Anything, mock.Anything).Return(due, nil)
	usr.On("GetAdminByAccountID", mock.Anything, "acc_1").
		Return(&domain.User{Email: "admin@acme.com", Name: "Ada"}, nil)
	pln.On("GetByID", mock.Anything, "plan_pro").Return(&domain.Plan{Name: "Pro"}, nil)
	mail.On("SendPrebillingEmail", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("smtp down"))

	sent, err := newPrebillingUC(acc, usr, pln, mail).Execute(context.Background())

	assert.NoError(t, err) // batch survives a single failure
	assert.Equal(t, 0, sent)
	acc.AssertNotCalled(t, "MarkPrebillingNotified", mock.Anything, mock.Anything, mock.Anything)
}

func TestSendPrebilling_RepoError_Propagates(t *testing.T) {
	acc := new(MockAccountRepo)
	usr := new(MockUserRepo)
	pln := new(MockPlanRepo)
	mail := new(MockEmailService)

	acc.On("ListAccountsDueForPrebilling", mock.Anything, mock.Anything).
		Return(([]*domain.PrebillingAccount)(nil), errors.New("db down"))

	sent, err := newPrebillingUC(acc, usr, pln, mail).Execute(context.Background())

	assert.Error(t, err)
	assert.Equal(t, 0, sent)
}
