package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestPurchaseAddon_Success(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	paymentAdpt := new(MockPaymentAdapter)

	account := &domain.Account{
		ID:       "acc_123",
		Name:     "Test Account",
		Currency: domain.CurrencyBRL,
	}

	// Regression (US-003): a BRL account's addon session is priced in brl.
	accountRepo.On("GetByID", ctx, "acc_123").Return(account, nil)
	paymentAdpt.On("CreateAddonCheckoutSession", "addon_p", "Recarga P", int64(4700), "acc_123", "brl", "https://example.com/success", "https://example.com/cancel").
		Return("https://checkout.stripe.com/addon_session", nil)

	uc := NewPurchaseAddonUseCase(accountRepo, paymentAdpt)
	url, err := uc.Execute(ctx, PurchaseAddonDTO{
		AccountID:  "acc_123",
		PackID:     "addon_p",
		SuccessURL: "https://example.com/success",
		CancelURL:  "https://example.com/cancel",
	})

	assert.NoError(t, err)
	assert.Equal(t, "https://checkout.stripe.com/addon_session", url)
	accountRepo.AssertExpectations(t)
	paymentAdpt.AssertExpectations(t)
}

// TestPurchaseAddon_USDAccountPricedInUSD: a USD account buys the USD pack
// set, charged in dollars via ad-hoc Stripe price_data — the pack price
// comes from addonPacksUSD (the $9 tier), NOT the BRL cents figure.
func TestPurchaseAddon_USDAccountPricedInUSD(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	paymentAdpt := new(MockPaymentAdapter)

	account := &domain.Account{ID: "acc_us", Currency: domain.CurrencyUSD}
	accountRepo.On("GetByID", ctx, "acc_us").Return(account, nil)
	paymentAdpt.On("CreateAddonCheckoutSession", "addon_p", "Top-up S", int64(900), "acc_us", "usd", "https://example.com/success", "https://example.com/cancel").
		Return("https://checkout.stripe.com/usd_addon", nil)

	uc := NewPurchaseAddonUseCase(accountRepo, paymentAdpt)
	url, err := uc.Execute(ctx, PurchaseAddonDTO{
		AccountID:  "acc_us",
		PackID:     "addon_p",
		SuccessURL: "https://example.com/success",
		CancelURL:  "https://example.com/cancel",
	})

	assert.NoError(t, err)
	assert.Equal(t, "https://checkout.stripe.com/usd_addon", url)
	accountRepo.AssertExpectations(t)
	paymentAdpt.AssertExpectations(t)
}

func TestPurchaseAddon_InvalidPackID(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	paymentAdpt := new(MockPaymentAdapter)

	// Account resolves first now; the pack id is what's invalid.
	accountRepo.On("GetByID", ctx, "acc_123").
		Return(&domain.Account{ID: "acc_123", Currency: domain.CurrencyBRL}, nil)

	uc := NewPurchaseAddonUseCase(accountRepo, paymentAdpt)
	url, err := uc.Execute(ctx, PurchaseAddonDTO{
		AccountID:  "acc_123",
		PackID:     "addon_nonexistent",
		SuccessURL: "https://example.com/success",
		CancelURL:  "https://example.com/cancel",
	})

	assert.Empty(t, url)
	assert.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "INVALID_PACK", appErr.Code)
}

func TestPurchaseAddon_AccountNotFound(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	paymentAdpt := new(MockPaymentAdapter)

	accountRepo.On("GetByID", ctx, "acc_missing").Return(nil, errors.New("not found"))

	uc := NewPurchaseAddonUseCase(accountRepo, paymentAdpt)
	url, err := uc.Execute(ctx, PurchaseAddonDTO{
		AccountID:  "acc_missing",
		PackID:     "addon_m",
		SuccessURL: "https://example.com/success",
		CancelURL:  "https://example.com/cancel",
	})

	assert.Empty(t, url)
	assert.Equal(t, domain.ErrAccountNotFound, err)
	accountRepo.AssertExpectations(t)
}

func TestPurchaseAddon_StripeError(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	paymentAdpt := new(MockPaymentAdapter)

	account := &domain.Account{
		ID:   "acc_123",
		Name: "Test Account",
	}

	accountRepo.On("GetByID", ctx, "acc_123").Return(account, nil)
	paymentAdpt.On("CreateAddonCheckoutSession", "addon_g", "Recarga G", int64(29700), "acc_123", "brl", "https://example.com/success", "https://example.com/cancel").
		Return("", errors.New("stripe error"))

	uc := NewPurchaseAddonUseCase(accountRepo, paymentAdpt)
	url, err := uc.Execute(ctx, PurchaseAddonDTO{
		AccountID:  "acc_123",
		PackID:     "addon_g",
		SuccessURL: "https://example.com/success",
		CancelURL:  "https://example.com/cancel",
	})

	assert.Empty(t, url)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not create stripe addon checkout session")
	accountRepo.AssertExpectations(t)
	paymentAdpt.AssertExpectations(t)
}
