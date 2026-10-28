package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateBillingPortalSession_Success(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	paymentAdpt := new(MockPaymentAdapter)
	uc := NewCreateBillingPortalSessionUseCase(accountRepo, paymentAdpt)

	accountRepo.On("GetByID", mock.Anything, "acc_1").Return(&domain.Account{
		ID:               "acc_1",
		StripeCustomerID: strPtr("cus_123"),
	}, nil)
	paymentAdpt.On("CreateBillingPortalSession", "cus_123", "https://app.replider.com/billing").
		Return("https://billing.stripe.com/session/abc", nil)

	url, err := uc.Execute(context.Background(), CreateBillingPortalSessionDTO{
		AccountID: "acc_1",
		ReturnURL: "https://app.replider.com/billing",
	})

	assert.NoError(t, err)
	assert.Equal(t, "https://billing.stripe.com/session/abc", url)
	paymentAdpt.AssertExpectations(t)
}

func TestCreateBillingPortalSession_NoCustomer(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	paymentAdpt := new(MockPaymentAdapter)
	uc := NewCreateBillingPortalSessionUseCase(accountRepo, paymentAdpt)

	// Account on the trial that never completed a checkout — no Stripe
	// customer exists, so the portal cannot be opened.
	accountRepo.On("GetByID", mock.Anything, "acc_trial").Return(&domain.Account{
		ID:               "acc_trial",
		StripeCustomerID: nil,
	}, nil)

	url, err := uc.Execute(context.Background(), CreateBillingPortalSessionDTO{
		AccountID: "acc_trial",
		ReturnURL: "https://app.replider.com/billing",
	})

	assert.ErrorIs(t, err, domain.ErrNoCustomer)
	assert.Empty(t, url)
	// Must never reach the Stripe adapter when there's no customer.
	paymentAdpt.AssertNotCalled(t, "CreateBillingPortalSession", mock.Anything, mock.Anything)
}

func TestCreateBillingPortalSession_EmptyCustomerID(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	paymentAdpt := new(MockPaymentAdapter)
	uc := NewCreateBillingPortalSessionUseCase(accountRepo, paymentAdpt)

	accountRepo.On("GetByID", mock.Anything, "acc_empty").Return(&domain.Account{
		ID:               "acc_empty",
		StripeCustomerID: strPtr(""),
	}, nil)

	url, err := uc.Execute(context.Background(), CreateBillingPortalSessionDTO{
		AccountID: "acc_empty",
		ReturnURL: "https://app.replider.com/billing",
	})

	assert.ErrorIs(t, err, domain.ErrNoCustomer)
	assert.Empty(t, url)
	paymentAdpt.AssertNotCalled(t, "CreateBillingPortalSession", mock.Anything, mock.Anything)
}

func TestCreateBillingPortalSession_AccountNotFound(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	paymentAdpt := new(MockPaymentAdapter)
	uc := NewCreateBillingPortalSessionUseCase(accountRepo, paymentAdpt)

	accountRepo.On("GetByID", mock.Anything, "acc_missing").Return(nil, nil)

	url, err := uc.Execute(context.Background(), CreateBillingPortalSessionDTO{
		AccountID: "acc_missing",
		ReturnURL: "https://app.replider.com/billing",
	})

	assert.ErrorIs(t, err, domain.ErrAccountNotFound)
	assert.Empty(t, url)
	paymentAdpt.AssertNotCalled(t, "CreateBillingPortalSession", mock.Anything, mock.Anything)
}

func TestCreateBillingPortalSession_RepoError(t *testing.T) {
	accountRepo := new(MockAccountRepo)
	paymentAdpt := new(MockPaymentAdapter)
	uc := NewCreateBillingPortalSessionUseCase(accountRepo, paymentAdpt)

	dbErr := errors.New("db down")
	accountRepo.On("GetByID", mock.Anything, "acc_1").Return(nil, dbErr)

	url, err := uc.Execute(context.Background(), CreateBillingPortalSessionDTO{
		AccountID: "acc_1",
		ReturnURL: "https://app.replider.com/billing",
	})

	assert.ErrorIs(t, err, dbErr)
	assert.Empty(t, url)
	paymentAdpt.AssertNotCalled(t, "CreateBillingPortalSession", mock.Anything, mock.Anything)
}
