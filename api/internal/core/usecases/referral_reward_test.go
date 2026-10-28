package usecases

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stripe/stripe-go/v76"
)

// checkoutEvent builds a minimal checkout.session.completed event for the
// referral reward tests: account acc1 pays 9900 brl on customer cus_referee.
func checkoutEvent(t *testing.T) stripe.Event {
	return checkoutEventInCurrency(t, "brl")
}

func checkoutEventInCurrency(t *testing.T, currency string) stripe.Event {
	t.Helper()
	raw, _ := json.Marshal(map[string]interface{}{
		"id":                  "cs_ref",
		"client_reference_id": "acc1",
		"customer":            map[string]interface{}{"id": "cus_referee"},
		"subscription":        map[string]interface{}{"id": "sub_ref"},
		"amount_total":        9900,
		"currency":            currency,
	})
	return stripe.Event{
		ID:   "evt_referral_1",
		Type: "checkout.session.completed",
		Data: &stripe.EventData{Raw: raw},
	}
}

func TestProcessStripeWebhook_ReferralReward(t *testing.T) {
	ctx := context.Background()

	t.Run("first paid checkout credits both referrer and referee then marks rewarded", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		referralRepo := new(MockReferralRepository)
		payment := new(MockPaymentAdapter)

		accountRepo.On("UpdateAccountSubscription", ctx, "acc1",
			ptrStr("cus_referee"), ptrStr("sub_ref"),
			string(domain.SubscriptionStatusActive)).Return(nil)

		// acc1 was referred by acc_referrer; reward still pending.
		referralRepo.On("GetPendingByReferee", ctx, "acc1").
			Return(&domain.Referral{ID: "ref1", ReferrerAccountID: "acc_referrer", RefereeAccountID: "acc1", Status: domain.ReferralStatusPending}, nil)

		// Referrer already has a Stripe customer → credit it directly.
		referrerCustomer := "cus_referrer"
		accountRepo.On("GetByID", ctx, "acc_referrer").
			Return(&domain.Account{ID: "acc_referrer", StripeCustomerID: &referrerCustomer}, nil)

		payment.On("GrantReferralCredit", "cus_referee", int64(9900), "brl", mock.Anything, "referral_referee_ref1").Return(nil)
		payment.On("GrantReferralCredit", "cus_referrer", int64(9900), "brl", mock.Anything, "referral_referrer_ref1").Return(nil)

		referralRepo.On("MarkRewarded", ctx, "ref1", int64(9900), "brl", mock.AnythingOfType("time.Time")).Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		uc.SetReferralRewarder(referralRepo, payment)

		err := uc.Execute(ctx, checkoutEvent(t))
		assert.NoError(t, err)
		referralRepo.AssertExpectations(t)
		payment.AssertExpectations(t)
	})

	t.Run("cross-currency referral converts the referrer credit at the FX rate", func(t *testing.T) {
		// US-015 (review fixes): referee paid $99.00; the referrer is a BRL
		// account. The credit must be CONVERTED (default rate 550 BRL cents
		// per USD → R$544.50), never the same cents figure re-labeled brl
		// — the old behavior turned R$199 referees into $199 credits.
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		referralRepo := new(MockReferralRepository)
		payment := new(MockPaymentAdapter)

		accountRepo.On("UpdateAccountSubscription", ctx, "acc1",
			ptrStr("cus_referee"), ptrStr("sub_ref"),
			string(domain.SubscriptionStatusActive)).Return(nil)

		referralRepo.On("GetPendingByReferee", ctx, "acc1").
			Return(&domain.Referral{ID: "ref1", ReferrerAccountID: "acc_referrer", RefereeAccountID: "acc1", Status: domain.ReferralStatusPending}, nil)

		referrerCustomer := "cus_referrer"
		accountRepo.On("GetByID", ctx, "acc_referrer").
			Return(&domain.Account{ID: "acc_referrer", Currency: domain.CurrencyBRL, StripeCustomerID: &referrerCustomer}, nil)

		payment.On("GrantReferralCredit", "cus_referee", int64(9900), "usd", mock.Anything, "referral_referee_ref1").Return(nil)
		// 9900 usd cents * 550 / 100 = 54450 brl cents.
		payment.On("GrantReferralCredit", "cus_referrer", int64(54450), "brl", mock.Anything, "referral_referrer_ref1").Return(nil)

		referralRepo.On("MarkRewarded", ctx, "ref1", int64(9900), "usd", mock.AnythingOfType("time.Time")).Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		uc.SetReferralRewarder(referralRepo, payment)

		err := uc.Execute(ctx, checkoutEventInCurrency(t, "usd"))
		assert.NoError(t, err)
		payment.AssertExpectations(t)
	})

	t.Run("BRL referee crediting a USD referrer converts down, not across", func(t *testing.T) {
		// R$99.00 paid by the referee must become $18.00 for a USD
		// referrer (9900 * 100 / 550 = 1800), not $99.00.
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		referralRepo := new(MockReferralRepository)
		payment := new(MockPaymentAdapter)

		accountRepo.On("UpdateAccountSubscription", ctx, "acc1",
			ptrStr("cus_referee"), ptrStr("sub_ref"),
			string(domain.SubscriptionStatusActive)).Return(nil)

		referralRepo.On("GetPendingByReferee", ctx, "acc1").
			Return(&domain.Referral{ID: "ref1", ReferrerAccountID: "acc_referrer", RefereeAccountID: "acc1", Status: domain.ReferralStatusPending}, nil)

		referrerCustomer := "cus_referrer"
		accountRepo.On("GetByID", ctx, "acc_referrer").
			Return(&domain.Account{ID: "acc_referrer", Currency: domain.CurrencyUSD, StripeCustomerID: &referrerCustomer}, nil)

		payment.On("GrantReferralCredit", "cus_referee", int64(9900), "brl", mock.Anything, "referral_referee_ref1").Return(nil)
		payment.On("GrantReferralCredit", "cus_referrer", int64(1800), "usd", mock.Anything, "referral_referrer_ref1").Return(nil)

		referralRepo.On("MarkRewarded", ctx, "ref1", int64(9900), "brl", mock.AnythingOfType("time.Time")).Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		uc.SetReferralRewarder(referralRepo, payment)

		err := uc.Execute(ctx, checkoutEvent(t))
		assert.NoError(t, err)
		payment.AssertExpectations(t)
	})

	t.Run("cross-currency conversion honors the owner-configured rate", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		referralRepo := new(MockReferralRepository)
		payment := new(MockPaymentAdapter)

		accountRepo.On("UpdateAccountSubscription", ctx, "acc1",
			ptrStr("cus_referee"), ptrStr("sub_ref"),
			string(domain.SubscriptionStatusActive)).Return(nil)

		referralRepo.On("GetPendingByReferee", ctx, "acc1").
			Return(&domain.Referral{ID: "ref1", ReferrerAccountID: "acc_referrer", RefereeAccountID: "acc1", Status: domain.ReferralStatusPending}, nil)

		referrerCustomer := "cus_referrer"
		accountRepo.On("GetByID", ctx, "acc_referrer").
			Return(&domain.Account{ID: "acc_referrer", Currency: domain.CurrencyBRL, StripeCustomerID: &referrerCustomer}, nil)

		payment.On("GrantReferralCredit", "cus_referee", int64(9900), "usd", mock.Anything, "referral_referee_ref1").Return(nil)
		// Owner-set rate 600: 9900 * 600 / 100 = 59400 brl cents.
		payment.On("GrantReferralCredit", "cus_referrer", int64(59400), "brl", mock.Anything, "referral_referrer_ref1").Return(nil)

		referralRepo.On("MarkRewarded", ctx, "ref1", int64(9900), "usd", mock.AnythingOfType("time.Time")).Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		uc.SetReferralRewarder(referralRepo, payment)
		uc.SetPlatformSettings(&mockSettingsKV{vals: map[string]string{
			domain.SettingUSDBRLRateCents: "600",
		}})

		err := uc.Execute(ctx, checkoutEventInCurrency(t, "usd"))
		assert.NoError(t, err)
		payment.AssertExpectations(t)
	})

	t.Run("unpriceable referrer currency skips the referrer credit but not the referee's", func(t *testing.T) {
		// A currency outside brl/usd can't be converted — granting the raw
		// cents figure would be wrong in an unknown direction, so the
		// referrer credit is skipped (logged for manual reconciliation)
		// while the referee's same-currency credit and the rewarded mark
		// still land.
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		referralRepo := new(MockReferralRepository)
		payment := new(MockPaymentAdapter)

		accountRepo.On("UpdateAccountSubscription", ctx, "acc1",
			ptrStr("cus_referee"), ptrStr("sub_ref"),
			string(domain.SubscriptionStatusActive)).Return(nil)

		referralRepo.On("GetPendingByReferee", ctx, "acc1").
			Return(&domain.Referral{ID: "ref1", ReferrerAccountID: "acc_referrer", RefereeAccountID: "acc1", Status: domain.ReferralStatusPending}, nil)

		referrerCustomer := "cus_referrer"
		accountRepo.On("GetByID", ctx, "acc_referrer").
			Return(&domain.Account{ID: "acc_referrer", Currency: "eur", StripeCustomerID: &referrerCustomer}, nil)

		payment.On("GrantReferralCredit", "cus_referee", int64(9900), "usd", mock.Anything, "referral_referee_ref1").Return(nil)

		referralRepo.On("MarkRewarded", ctx, "ref1", int64(9900), "usd", mock.AnythingOfType("time.Time")).Return(nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		uc.SetReferralRewarder(referralRepo, payment)

		err := uc.Execute(ctx, checkoutEventInCurrency(t, "usd"))
		assert.NoError(t, err)
		payment.AssertExpectations(t)
		payment.AssertNotCalled(t, "GrantReferralCredit", "cus_referrer", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("non-referred account grants no credit", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		referralRepo := new(MockReferralRepository)
		payment := new(MockPaymentAdapter)

		accountRepo.On("UpdateAccountSubscription", ctx, "acc1",
			ptrStr("cus_referee"), ptrStr("sub_ref"),
			string(domain.SubscriptionStatusActive)).Return(nil)

		// No pending referral for this account.
		referralRepo.On("GetPendingByReferee", ctx, "acc1").Return(nil, nil)

		uc := newStripeWebhookUC(accountRepo, usageRepo)
		uc.SetReferralRewarder(referralRepo, payment)

		err := uc.Execute(ctx, checkoutEvent(t))
		assert.NoError(t, err)
		referralRepo.AssertExpectations(t)
		payment.AssertNotCalled(t, "GrantReferralCredit", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		referralRepo.AssertNotCalled(t, "MarkRewarded", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}
