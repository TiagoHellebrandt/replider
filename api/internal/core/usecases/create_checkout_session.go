package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

type CreateCheckoutSessionUseCase struct {
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
	couponRepo  domain.CouponRepository
	paymentAdpt domain.PaymentAdapter
}

func NewCreateCheckoutSessionUseCase(accountRepo domain.AccountRepository, planRepo domain.PlanRepository, couponRepo domain.CouponRepository, paymentAdpt domain.PaymentAdapter) *CreateCheckoutSessionUseCase {
	return &CreateCheckoutSessionUseCase{
		accountRepo: accountRepo,
		planRepo:    planRepo,
		couponRepo:  couponRepo,
		paymentAdpt: paymentAdpt,
	}
}

type CreateCheckoutSessionDTO struct {
	AccountID  string
	PlanID     string // target plan to upgrade to
	SuccessURL string
	CancelURL  string
	// CouponCode is the public-facing slug from `?coupon=` on the
	// register / billing page. Optional — when empty, no discount is
	// applied. Invalid / inactive / expired codes are silently
	// ignored so a stale invite link doesn't block the customer
	// from upgrading.
	CouponCode string
	// Interval selects the monthly or annual Stripe price (US-169).
	// Empty / "monthly" → monthly price; "annual" → the plan's annual
	// price, falling back to monthly when the plan has none. The handler
	// normalizes this against domain.NormalizeBillingInterval before
	// reaching here.
	Interval string
}

func (uc *CreateCheckoutSessionUseCase) Execute(ctx context.Context, dto CreateCheckoutSessionDTO) (string, error) {
	// 1. Fetch Account
	account, err := uc.accountRepo.GetByID(ctx, dto.AccountID)
	if err != nil || account == nil {
		return "", domain.ErrUnauthorized // or ErrAccountNotFound
	}

	// 2. Fetch target Plan (from request, not account — supports trial → paid upgrade)
	planID := dto.PlanID
	if planID == "" && account.PlanID != nil {
		planID = *account.PlanID
	}
	if planID == "" {
		return "", domain.ErrPlanNotFound
	}
	plan, err := uc.planRepo.GetByID(ctx, planID)
	if err != nil || plan == nil {
		return "", domain.ErrPlanNotFound
	}

	if plan.StripePriceID == nil || *plan.StripePriceID == "" {
		slog.WarnContext(ctx, "checkout blocked: plan missing stripe_price_id",
			"plan_id", plan.ID, "plan_name", plan.Name, "account_id", dto.AccountID)
		return "", domain.ErrPlanNotPurchasable
	}

	// The session's currency comes from the Stripe Price the plan points
	// at, so a USD plan bills USD by construction. This guard keeps an
	// account from checking out a plan in the OTHER currency (the /plans
	// listing already filters by account currency; only a hand-crafted
	// request gets here). Empty on either side skips the check — plans/
	// accounts predating the currency columns are all BRL. (US-003)
	if account.Currency != "" && plan.Currency != "" && account.Currency != plan.Currency {
		slog.WarnContext(ctx, "checkout blocked: plan currency mismatch",
			"plan_id", plan.ID, "plan_currency", plan.Currency,
			"account_id", dto.AccountID, "account_currency", account.Currency)
		return "", domain.ErrPlanCurrencyMismatch
	}

	// 3. Resolve coupon code → Stripe promotion code id. Best-effort:
	// any miss (unknown / inactive / expired / no repo wired) drops
	// the promo silently so the checkout still proceeds.
	promoID := ""
	if dto.CouponCode != "" && uc.couponRepo != nil {
		c, err := uc.couponRepo.GetByCode(ctx, dto.CouponCode)
		if err == nil && c != nil && c.IsActive && c.StripePromoID != nil {
			expired := c.ExpiresAt != nil && time.Now().After(*c.ExpiresAt)
			if !expired {
				// US-013: enforce max_redemptions BEFORE creating the
				// session. Stripe would reject an over-cap redemption
				// mid-checkout, but pre-checking gives a clean
				// COUPON_EXHAUSTED instead of a confusing Stripe error,
				// and closes the window where many concurrent checkouts
				// each pass a stale local redeemed_count. We read the
				// live count from Stripe (source of truth for redemptions).
				if c.MaxRedemptions != nil {
					redeemed, rerr := uc.paymentAdpt.GetPromotionCodeRedeemed(*c.StripePromoID)
					if rerr != nil {
						// Best-effort: a Stripe read error must not block a
						// legitimate checkout — Stripe still enforces the cap
						// at redemption time. Log and proceed with the promo.
						slog.WarnContext(ctx, "coupon redemption check failed, proceeding",
							"coupon", c.Code, "error", rerr)
					} else if redeemed >= *c.MaxRedemptions {
						return "", domain.ErrCouponExhausted
					}
				}
				promoID = *c.StripePromoID
			}
		}
	}

	// 4. Ensure a single Stripe customer for this account BEFORE the
	// session, so a checkout double-click (or two tabs) can't spawn two
	// Stripe customers / subscriptions (US-017). EnsureCustomer is
	// idempotent on the Stripe side (key derived from account.ID);
	// SetStripeCustomerIfAbsent makes the local persist atomic so all
	// concurrent callers converge on the first id written.
	customerID := ""
	if account.StripeCustomerID != nil {
		customerID = *account.StripeCustomerID
	}
	if customerID == "" {
		created, err := uc.paymentAdpt.EnsureCustomer(account.ID)
		if err != nil {
			return "", fmt.Errorf("could not ensure stripe customer: %w", err)
		}
		customerID, err = uc.accountRepo.SetStripeCustomerIfAbsent(ctx, account.ID, created)
		if err != nil {
			return "", fmt.Errorf("could not persist stripe customer: %w", err)
		}
	}

	// 5. Generate Stripe Session bound to the resolved customer.
	// ResolveStripePriceID picks the annual price when the caller asked
	// for it and the plan has one, otherwise the monthly price (US-169).
	priceID := plan.ResolveStripePriceID(dto.Interval)
	url, err := uc.paymentAdpt.CreateCheckoutSession(priceID, account.ID, customerID, dto.SuccessURL, dto.CancelURL, promoID)
	if err != nil {
		return "", fmt.Errorf("could not create stripe checkout session: %w", err)
	}

	return url, nil
}
