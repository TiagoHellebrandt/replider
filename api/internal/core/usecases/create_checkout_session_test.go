package usecases

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateCheckoutSession_Success(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	paymentAdpt := new(MockPaymentAdapter)

	planID := "plan_123"
	stripePriceID := "price_abc"
	// Regression (US-003): a BRL account checking out a BRL plan still
	// works — the session binds the plan's (BRL) Stripe price.
	account := &domain.Account{
		ID:       "acc_123",
		Name:     "Test Account",
		PlanID:   &planID,
		Currency: domain.CurrencyBRL,
	}
	plan := &domain.Plan{
		ID:            planID,
		Name:          "Pro",
		Currency:      domain.CurrencyBRL,
		StripePriceID: &stripePriceID,
	}

	accountRepo.On("GetByID", ctx, "acc_123").Return(account, nil)
	planRepo.On("GetByID", ctx, planID).Return(plan, nil)
	paymentAdpt.On("EnsureCustomer", "acc_123").Return("cus_123", nil)
	accountRepo.On("SetStripeCustomerIfAbsent", ctx, "acc_123", "cus_123").Return("cus_123", nil)
	paymentAdpt.On("CreateCheckoutSession", stripePriceID, "acc_123", "cus_123", "https://example.com/success", "https://example.com/cancel", "").
		Return("https://checkout.stripe.com/session_123", nil)

	uc := NewCreateCheckoutSessionUseCase(accountRepo, planRepo, nil, paymentAdpt)
	url, err := uc.Execute(ctx, CreateCheckoutSessionDTO{
		AccountID:  "acc_123",
		SuccessURL: "https://example.com/success",
		CancelURL:  "https://example.com/cancel",
	})

	assert.NoError(t, err)
	assert.Equal(t, "https://checkout.stripe.com/session_123", url)
	accountRepo.AssertExpectations(t)
	planRepo.AssertExpectations(t)
	paymentAdpt.AssertExpectations(t)
}

// TestCreateCheckoutSession_USDPlanForUSDAccount locks in US-003: a USD
// account checking out a USD plan produces a session bound to the plan's
// USD Stripe price (the session currency IS the Price's currency).
func TestCreateCheckoutSession_USDPlanForUSDAccount(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	paymentAdpt := new(MockPaymentAdapter)

	usdPrice := "price_usd_core"
	account := &domain.Account{ID: "acc_us", Currency: domain.CurrencyUSD}
	plan := &domain.Plan{ID: "core_usd", Name: "Core", Currency: domain.CurrencyUSD, StripePriceID: &usdPrice}

	accountRepo.On("GetByID", ctx, "acc_us").Return(account, nil)
	planRepo.On("GetByID", ctx, "core_usd").Return(plan, nil)
	paymentAdpt.On("EnsureCustomer", "acc_us").Return("cus_us", nil)
	accountRepo.On("SetStripeCustomerIfAbsent", ctx, "acc_us", "cus_us").Return("cus_us", nil)
	paymentAdpt.On("CreateCheckoutSession", usdPrice, "acc_us", "cus_us", "https://s", "https://c", "").
		Return("https://checkout.stripe.com/usd", nil)

	uc := NewCreateCheckoutSessionUseCase(accountRepo, planRepo, nil, paymentAdpt)
	url, err := uc.Execute(ctx, CreateCheckoutSessionDTO{
		AccountID: "acc_us", PlanID: "core_usd", SuccessURL: "https://s", CancelURL: "https://c",
	})

	assert.NoError(t, err)
	assert.Equal(t, "https://checkout.stripe.com/usd", url)
	paymentAdpt.AssertExpectations(t)
}

// TestCreateCheckoutSession_CurrencyMismatchBlocked locks in US-003: an
// account cannot check out a plan priced in the other currency — the
// /plans listing never offers one, so only a hand-crafted request gets here.
func TestCreateCheckoutSession_CurrencyMismatchBlocked(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	paymentAdpt := new(MockPaymentAdapter)

	brlPrice := "price_brl_pro"
	account := &domain.Account{ID: "acc_us", Currency: domain.CurrencyUSD}
	plan := &domain.Plan{ID: "pro_brl", Name: "Pro", Currency: domain.CurrencyBRL, StripePriceID: &brlPrice}

	accountRepo.On("GetByID", ctx, "acc_us").Return(account, nil)
	planRepo.On("GetByID", ctx, "pro_brl").Return(plan, nil)

	uc := NewCreateCheckoutSessionUseCase(accountRepo, planRepo, nil, paymentAdpt)
	url, err := uc.Execute(ctx, CreateCheckoutSessionDTO{
		AccountID: "acc_us", PlanID: "pro_brl", SuccessURL: "https://s", CancelURL: "https://c",
	})

	assert.Empty(t, url)
	assert.Equal(t, domain.ErrPlanCurrencyMismatch, err)
	paymentAdpt.AssertNotCalled(t, "CreateCheckoutSession", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	paymentAdpt.AssertNotCalled(t, "EnsureCustomer", mock.Anything)
}

// TestCreateCheckoutSession_AnnualInterval locks in US-169: when the caller
// asks for annual billing and the plan carries an annual Stripe price, the
// checkout session is created against that annual price, not the monthly one.
func TestCreateCheckoutSession_AnnualInterval(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	paymentAdpt := new(MockPaymentAdapter)

	planID := "plan_123"
	monthlyPrice := "price_monthly"
	annualPrice := "price_annual"
	account := &domain.Account{ID: "acc_123", Name: "Test Account", PlanID: &planID}
	plan := &domain.Plan{
		ID:                  planID,
		Name:                "Pro",
		StripePriceID:       &monthlyPrice,
		StripePriceIDAnnual: &annualPrice,
	}

	accountRepo.On("GetByID", ctx, "acc_123").Return(account, nil)
	planRepo.On("GetByID", ctx, planID).Return(plan, nil)
	paymentAdpt.On("EnsureCustomer", "acc_123").Return("cus_123", nil)
	accountRepo.On("SetStripeCustomerIfAbsent", ctx, "acc_123", "cus_123").Return("cus_123", nil)
	// Bound to the ANNUAL price.
	paymentAdpt.On("CreateCheckoutSession", annualPrice, "acc_123", "cus_123", "https://s", "https://c", "").
		Return("https://checkout.stripe.com/annual", nil)

	uc := NewCreateCheckoutSessionUseCase(accountRepo, planRepo, nil, paymentAdpt)
	url, err := uc.Execute(ctx, CreateCheckoutSessionDTO{
		AccountID: "acc_123", SuccessURL: "https://s", CancelURL: "https://c",
		Interval: domain.BillingIntervalAnnual,
	})

	assert.NoError(t, err)
	assert.Equal(t, "https://checkout.stripe.com/annual", url)
	paymentAdpt.AssertExpectations(t)
}

// TestCreateCheckoutSession_AnnualFallsBackToMonthly locks in US-169's
// incremental-rollout guarantee: asking for annual on a plan that has no
// annual price configured falls back to the monthly price rather than
// blocking the upgrade.
func TestCreateCheckoutSession_AnnualFallsBackToMonthly(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	paymentAdpt := new(MockPaymentAdapter)

	planID := "plan_123"
	monthlyPrice := "price_monthly"
	account := &domain.Account{ID: "acc_123", Name: "Test Account", PlanID: &planID}
	plan := &domain.Plan{ID: planID, Name: "Pro", StripePriceID: &monthlyPrice} // no annual

	accountRepo.On("GetByID", ctx, "acc_123").Return(account, nil)
	planRepo.On("GetByID", ctx, planID).Return(plan, nil)
	paymentAdpt.On("EnsureCustomer", "acc_123").Return("cus_123", nil)
	accountRepo.On("SetStripeCustomerIfAbsent", ctx, "acc_123", "cus_123").Return("cus_123", nil)
	// Falls back to the MONTHLY price.
	paymentAdpt.On("CreateCheckoutSession", monthlyPrice, "acc_123", "cus_123", "https://s", "https://c", "").
		Return("https://checkout.stripe.com/monthly", nil)

	uc := NewCreateCheckoutSessionUseCase(accountRepo, planRepo, nil, paymentAdpt)
	url, err := uc.Execute(ctx, CreateCheckoutSessionDTO{
		AccountID: "acc_123", SuccessURL: "https://s", CancelURL: "https://c",
		Interval: domain.BillingIntervalAnnual,
	})

	assert.NoError(t, err)
	assert.Equal(t, "https://checkout.stripe.com/monthly", url)
	paymentAdpt.AssertExpectations(t)
}

func TestCreateCheckoutSession_AccountNotFound(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	paymentAdpt := new(MockPaymentAdapter)

	accountRepo.On("GetByID", ctx, "acc_missing").Return(nil, errors.New("not found"))

	uc := NewCreateCheckoutSessionUseCase(accountRepo, planRepo, nil, paymentAdpt)
	url, err := uc.Execute(ctx, CreateCheckoutSessionDTO{
		AccountID:  "acc_missing",
		SuccessURL: "https://example.com/success",
		CancelURL:  "https://example.com/cancel",
	})

	assert.Empty(t, url)
	assert.Equal(t, domain.ErrUnauthorized, err)
	accountRepo.AssertExpectations(t)
}

func TestCreateCheckoutSession_PlanNotFound(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	paymentAdpt := new(MockPaymentAdapter)

	planID := "plan_missing"
	account := &domain.Account{
		ID:     "acc_123",
		Name:   "Test Account",
		PlanID: &planID,
	}

	accountRepo.On("GetByID", ctx, "acc_123").Return(account, nil)
	planRepo.On("GetByID", ctx, planID).Return(nil, errors.New("not found"))

	uc := NewCreateCheckoutSessionUseCase(accountRepo, planRepo, nil, paymentAdpt)
	url, err := uc.Execute(ctx, CreateCheckoutSessionDTO{
		AccountID:  "acc_123",
		SuccessURL: "https://example.com/success",
		CancelURL:  "https://example.com/cancel",
	})

	assert.Empty(t, url)
	assert.Equal(t, domain.ErrPlanNotFound, err)
	accountRepo.AssertExpectations(t)
	planRepo.AssertExpectations(t)
}

func TestCreateCheckoutSession_AccountWithNilPlanID(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	paymentAdpt := new(MockPaymentAdapter)

	account := &domain.Account{
		ID:     "acc_123",
		Name:   "Test Account",
		PlanID: nil,
	}

	accountRepo.On("GetByID", ctx, "acc_123").Return(account, nil)

	uc := NewCreateCheckoutSessionUseCase(accountRepo, planRepo, nil, paymentAdpt)
	url, err := uc.Execute(ctx, CreateCheckoutSessionDTO{
		AccountID:  "acc_123",
		SuccessURL: "https://example.com/success",
		CancelURL:  "https://example.com/cancel",
	})

	assert.Empty(t, url)
	assert.Equal(t, domain.ErrPlanNotFound, err)
	accountRepo.AssertExpectations(t)
}

func TestCreateCheckoutSession_PlanMissingStripePriceID(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	paymentAdpt := new(MockPaymentAdapter)

	planID := "plan_partner"
	account := &domain.Account{
		ID:     "acc_123",
		Name:   "Test Account",
		PlanID: &planID,
	}
	plan := &domain.Plan{
		ID:            planID,
		Name:          "Partner",
		StripePriceID: nil,
	}

	accountRepo.On("GetByID", ctx, "acc_123").Return(account, nil)
	planRepo.On("GetByID", ctx, planID).Return(plan, nil)

	uc := NewCreateCheckoutSessionUseCase(accountRepo, planRepo, nil, paymentAdpt)
	url, err := uc.Execute(ctx, CreateCheckoutSessionDTO{
		AccountID:  "acc_123",
		SuccessURL: "https://example.com/success",
		CancelURL:  "https://example.com/cancel",
	})

	assert.Empty(t, url)
	assert.Equal(t, domain.ErrPlanNotPurchasable, err)
	accountRepo.AssertExpectations(t)
	planRepo.AssertExpectations(t)
}

func TestCreateCheckoutSession_PlanEmptyStripePriceID(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	paymentAdpt := new(MockPaymentAdapter)

	planID := "plan_empty"
	emptyPrice := ""
	account := &domain.Account{
		ID:     "acc_123",
		Name:   "Test Account",
		PlanID: &planID,
	}
	plan := &domain.Plan{
		ID:            planID,
		Name:          "Empty Price",
		StripePriceID: &emptyPrice,
	}

	accountRepo.On("GetByID", ctx, "acc_123").Return(account, nil)
	planRepo.On("GetByID", ctx, planID).Return(plan, nil)

	uc := NewCreateCheckoutSessionUseCase(accountRepo, planRepo, nil, paymentAdpt)
	url, err := uc.Execute(ctx, CreateCheckoutSessionDTO{
		AccountID:  "acc_123",
		SuccessURL: "https://example.com/success",
		CancelURL:  "https://example.com/cancel",
	})

	assert.Empty(t, url)
	assert.Equal(t, domain.ErrPlanNotPurchasable, err)
	accountRepo.AssertExpectations(t)
	planRepo.AssertExpectations(t)
}

func TestCreateCheckoutSession_StripeAdapterError(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	paymentAdpt := new(MockPaymentAdapter)

	planID := "plan_123"
	stripePriceID := "price_abc"
	account := &domain.Account{
		ID:     "acc_123",
		Name:   "Test Account",
		PlanID: &planID,
	}
	plan := &domain.Plan{
		ID:            planID,
		Name:          "Pro",
		StripePriceID: &stripePriceID,
	}

	accountRepo.On("GetByID", ctx, "acc_123").Return(account, nil)
	planRepo.On("GetByID", ctx, planID).Return(plan, nil)
	paymentAdpt.On("EnsureCustomer", "acc_123").Return("cus_123", nil)
	accountRepo.On("SetStripeCustomerIfAbsent", ctx, "acc_123", "cus_123").Return("cus_123", nil)
	paymentAdpt.On("CreateCheckoutSession", stripePriceID, "acc_123", "cus_123", "https://example.com/success", "https://example.com/cancel", "").
		Return("", errors.New("stripe connection error"))

	uc := NewCreateCheckoutSessionUseCase(accountRepo, planRepo, nil, paymentAdpt)
	url, err := uc.Execute(ctx, CreateCheckoutSessionDTO{
		AccountID:  "acc_123",
		SuccessURL: "https://example.com/success",
		CancelURL:  "https://example.com/cancel",
	})

	assert.Empty(t, url)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not create stripe checkout session")
	accountRepo.AssertExpectations(t)
	planRepo.AssertExpectations(t)
	paymentAdpt.AssertExpectations(t)
}

func TestCreateCheckoutSession_ResolvesCouponCodeToPromoID(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	couponRepo := new(MockCouponRepo)
	paymentAdpt := new(MockPaymentAdapter)

	planID := "plan_123"
	stripePriceID := "price_abc"
	promoID := "promo_456"
	accountRepo.On("GetByID", ctx, "acc_123").
		Return(&domain.Account{ID: "acc_123", PlanID: &planID}, nil)
	planRepo.On("GetByID", ctx, planID).
		Return(&domain.Plan{ID: planID, StripePriceID: &stripePriceID}, nil)
	couponRepo.On("GetByCode", ctx, "WELCOME10").Return(&domain.Coupon{
		Code: "WELCOME10", IsActive: true, StripePromoID: &promoID,
	}, nil)
	paymentAdpt.On("EnsureCustomer", "acc_123").Return("cus_123", nil)
	accountRepo.On("SetStripeCustomerIfAbsent", ctx, "acc_123", "cus_123").Return("cus_123", nil)
	paymentAdpt.On("CreateCheckoutSession", stripePriceID, "acc_123", "cus_123", "https://s", "https://c", "promo_456").
		Return("https://checkout.stripe.com/sess", nil)

	uc := NewCreateCheckoutSessionUseCase(accountRepo, planRepo, couponRepo, paymentAdpt)
	url, err := uc.Execute(ctx, CreateCheckoutSessionDTO{
		AccountID: "acc_123", SuccessURL: "https://s", CancelURL: "https://c",
		CouponCode: "WELCOME10",
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, url)
	paymentAdpt.AssertExpectations(t)
}

func TestCreateCheckoutSession_DropsCouponWhenInactive(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	couponRepo := new(MockCouponRepo)
	paymentAdpt := new(MockPaymentAdapter)

	planID := "plan_123"
	stripePriceID := "price_abc"
	promoID := "promo_456"
	accountRepo.On("GetByID", ctx, "acc_123").
		Return(&domain.Account{ID: "acc_123", PlanID: &planID}, nil)
	planRepo.On("GetByID", ctx, planID).
		Return(&domain.Plan{ID: planID, StripePriceID: &stripePriceID}, nil)
	couponRepo.On("GetByCode", ctx, "DEAD").Return(&domain.Coupon{
		Code: "DEAD", IsActive: false, StripePromoID: &promoID,
	}, nil)
	paymentAdpt.On("EnsureCustomer", "acc_123").Return("cus_123", nil)
	accountRepo.On("SetStripeCustomerIfAbsent", ctx, "acc_123", "cus_123").Return("cus_123", nil)
	// Empty promoID forwarded — inactive coupon must NOT be applied.
	paymentAdpt.On("CreateCheckoutSession", stripePriceID, "acc_123", "cus_123", "https://s", "https://c", "").
		Return("https://checkout.stripe.com/sess", nil)

	uc := NewCreateCheckoutSessionUseCase(accountRepo, planRepo, couponRepo, paymentAdpt)
	_, err := uc.Execute(ctx, CreateCheckoutSessionDTO{
		AccountID: "acc_123", SuccessURL: "https://s", CancelURL: "https://c",
		CouponCode: "DEAD",
	})
	assert.NoError(t, err)
	paymentAdpt.AssertExpectations(t)
}

// TestCreateCheckoutSession_CouponExhausted locks in US-013: a coupon whose
// live Stripe redemption count has hit its max_redemptions cap must block
// checkout with COUPON_EXHAUSTED before any session is created — closing
// the infinite-discount exploit.
func TestCreateCheckoutSession_CouponExhausted(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	couponRepo := new(MockCouponRepo)
	paymentAdpt := new(MockPaymentAdapter)

	planID := "plan_123"
	stripePriceID := "price_abc"
	promoID := "promo_capped"
	maxRedemptions := 100
	account := &domain.Account{ID: "acc_123", PlanID: &planID}
	plan := &domain.Plan{ID: planID, StripePriceID: &stripePriceID}
	coupon := &domain.Coupon{
		Code:           "LAUNCH",
		StripePromoID:  &promoID,
		MaxRedemptions: &maxRedemptions,
		IsActive:       true,
	}

	accountRepo.On("GetByID", ctx, "acc_123").Return(account, nil)
	planRepo.On("GetByID", ctx, planID).Return(plan, nil)
	couponRepo.On("GetByCode", ctx, "LAUNCH").Return(coupon, nil)
	// Stripe reports the cap is already reached.
	paymentAdpt.On("GetPromotionCodeRedeemed", promoID).Return(100, nil)

	uc := NewCreateCheckoutSessionUseCase(accountRepo, planRepo, couponRepo, paymentAdpt)
	url, err := uc.Execute(ctx, CreateCheckoutSessionDTO{
		AccountID:  "acc_123",
		SuccessURL: "https://example.com/s",
		CancelURL:  "https://example.com/c",
		CouponCode: "LAUNCH",
	})

	assert.Empty(t, url)
	assert.Equal(t, domain.ErrCouponExhausted, err)
	// No session created once the coupon is exhausted.
	paymentAdpt.AssertNotCalled(t, "CreateCheckoutSession", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	// Exhausted coupon returns before customer creation — no Stripe customer minted.
	paymentAdpt.AssertNotCalled(t, "EnsureCustomer", mock.Anything)
}

// TestCreateCheckoutSession_CouponWithinLimit confirms a coupon below its
// cap still applies and the session is created with the promo id.
func TestCreateCheckoutSession_CouponWithinLimit(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	couponRepo := new(MockCouponRepo)
	paymentAdpt := new(MockPaymentAdapter)

	planID := "plan_123"
	stripePriceID := "price_abc"
	promoID := "promo_ok"
	maxRedemptions := 100
	account := &domain.Account{ID: "acc_123", PlanID: &planID}
	plan := &domain.Plan{ID: planID, StripePriceID: &stripePriceID}
	coupon := &domain.Coupon{Code: "OK", StripePromoID: &promoID, MaxRedemptions: &maxRedemptions, IsActive: true}

	accountRepo.On("GetByID", ctx, "acc_123").Return(account, nil)
	planRepo.On("GetByID", ctx, planID).Return(plan, nil)
	couponRepo.On("GetByCode", ctx, "OK").Return(coupon, nil)
	paymentAdpt.On("GetPromotionCodeRedeemed", promoID).Return(42, nil)
	paymentAdpt.On("EnsureCustomer", "acc_123").Return("cus_123", nil)
	accountRepo.On("SetStripeCustomerIfAbsent", ctx, "acc_123", "cus_123").Return("cus_123", nil)
	paymentAdpt.On("CreateCheckoutSession", stripePriceID, "acc_123", "cus_123", "https://example.com/s", "https://example.com/c", promoID).
		Return("https://checkout.stripe.com/ok", nil)

	uc := NewCreateCheckoutSessionUseCase(accountRepo, planRepo, couponRepo, paymentAdpt)
	url, err := uc.Execute(ctx, CreateCheckoutSessionDTO{
		AccountID: "acc_123", SuccessURL: "https://example.com/s", CancelURL: "https://example.com/c", CouponCode: "OK",
	})

	assert.NoError(t, err)
	assert.Equal(t, "https://checkout.stripe.com/ok", url)
}

// TestCreateCheckoutSession_ReusesExistingCustomer locks in US-017: when the
// account already carries a stripe_customer_id, no new Stripe customer is
// minted — the persisted id is bound to the session directly.
func TestCreateCheckoutSession_ReusesExistingCustomer(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	paymentAdpt := new(MockPaymentAdapter)

	planID := "plan_123"
	stripePriceID := "price_abc"
	existing := "cus_existing"
	account := &domain.Account{ID: "acc_123", PlanID: &planID, StripeCustomerID: &existing}
	plan := &domain.Plan{ID: planID, StripePriceID: &stripePriceID}

	accountRepo.On("GetByID", ctx, "acc_123").Return(account, nil)
	planRepo.On("GetByID", ctx, planID).Return(plan, nil)
	paymentAdpt.On("CreateCheckoutSession", stripePriceID, "acc_123", "cus_existing", "https://s", "https://c", "").
		Return("https://checkout.stripe.com/reuse", nil)

	uc := NewCreateCheckoutSessionUseCase(accountRepo, planRepo, nil, paymentAdpt)
	url, err := uc.Execute(ctx, CreateCheckoutSessionDTO{
		AccountID: "acc_123", SuccessURL: "https://s", CancelURL: "https://c",
	})

	assert.NoError(t, err)
	assert.Equal(t, "https://checkout.stripe.com/reuse", url)
	// No customer creation / persist when one already exists.
	paymentAdpt.AssertNotCalled(t, "EnsureCustomer", mock.Anything)
	accountRepo.AssertNotCalled(t, "SetStripeCustomerIfAbsent", mock.Anything, mock.Anything, mock.Anything)
}

// TestCreateCheckoutSession_ConcurrentCheckoutsConvergeOnOneCustomer locks in
// US-017's core guarantee: two parallel checkouts produce a single Stripe
// customer. EnsureCustomer is idempotent on Stripe's side (idempotency key
// derived from the account id), and SetStripeCustomerIfAbsent's atomic
// COALESCE makes every concurrent caller converge on the first id written —
// so both sessions bind to the same customer.
func TestCreateCheckoutSession_ConcurrentCheckoutsConvergeOnOneCustomer(t *testing.T) {
	ctx := context.Background()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	paymentAdpt := new(MockPaymentAdapter)

	planID := "plan_123"
	stripePriceID := "price_abc"
	account := &domain.Account{ID: "acc_123", PlanID: &planID} // no customer yet
	plan := &domain.Plan{ID: planID, StripePriceID: &stripePriceID}

	accountRepo.On("GetByID", ctx, "acc_123").Return(account, nil)
	planRepo.On("GetByID", ctx, planID).Return(plan, nil)
	// Idempotency at Stripe → same customer regardless of how many callers.
	paymentAdpt.On("EnsureCustomer", "acc_123").Return("cus_winner", nil)
	// Atomic DB claim: whoever wins the row, every caller reads the winner.
	accountRepo.On("SetStripeCustomerIfAbsent", ctx, "acc_123", "cus_winner").Return("cus_winner", nil)
	// The session MUST be bound to the single converged customer.
	paymentAdpt.On("CreateCheckoutSession", stripePriceID, "acc_123", "cus_winner", "https://s", "https://c", "").
		Return("https://checkout.stripe.com/sess", nil)

	uc := NewCreateCheckoutSessionUseCase(accountRepo, planRepo, nil, paymentAdpt)

	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := uc.Execute(ctx, CreateCheckoutSessionDTO{
				AccountID: "acc_123", SuccessURL: "https://s", CancelURL: "https://c",
			})
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		assert.NoError(t, err)
	}
	// Every concurrent checkout bound to "cus_winner" — one customer total.
	for _, call := range paymentAdpt.Calls {
		if call.Method == "CreateCheckoutSession" {
			assert.Equal(t, "cus_winner", call.Arguments.String(2))
		}
	}
}
