package usecases

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/replider/api/internal/core/domain"
)

// stripeCheckoutDelegate is the narrow slice of the Stripe checkout use
// case the transparent checkout delegates to for non-BRL (card) accounts.
type stripeCheckoutDelegate interface {
	Execute(ctx context.Context, dto CreateCheckoutSessionDTO) (string, error)
}

// CreateTransparentCheckoutUseCase drives the country-routed checkout:
// BRL accounts pay via AbacatePay (PIX embedded + card hosted, Elo
// support, cheaper %); every other currency pays card via Stripe. It
// reuses the existing Stripe checkout use case for the non-BRL branch so
// there is one source of truth for coupons / customer creation / price
// resolution.
type CreateTransparentCheckoutUseCase struct {
	accountRepo    domain.AccountRepository
	planRepo       domain.PlanRepository
	abacate        domain.AbacatePayAdapter
	stripeCheckout stripeCheckoutDelegate
}

func NewCreateTransparentCheckoutUseCase(
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	abacate domain.AbacatePayAdapter,
	stripeCheckout stripeCheckoutDelegate,
) *CreateTransparentCheckoutUseCase {
	return &CreateTransparentCheckoutUseCase{
		accountRepo:    accountRepo,
		planRepo:       planRepo,
		abacate:        abacate,
		stripeCheckout: stripeCheckout,
	}
}

// TransparentCheckoutMethod selects the BRL payment rail.
const (
	CheckoutMethodPix  = "pix"
	CheckoutMethodCard = "card"
)

type CreateTransparentCheckoutDTO struct {
	AccountID  string
	PlanID     string
	Interval   string // monthly|annual (already normalized by the handler)
	Method     string // pix|card — only meaningful for BRL/AbacatePay
	CouponCode string
	SuccessURL string
	CancelURL  string
}

// TransparentCheckoutResult is the discriminated response. For a Stripe
// or AbacatePay card checkout only URL is set; for a BRL PIX checkout
// only Pix is set.
type TransparentCheckoutResult struct {
	Provider string            `json:"provider"` // "abacatepay" | "stripe"
	Method   string            `json:"method"`   // "pix" | "card"
	URL      string            `json:"url,omitempty"`
	Pix      *domain.PixCharge `json:"pix,omitempty"`
}

// isContractPlan reports whether a plan is sold under contract (no
// self-serve online price). Mirrors the Stripe checkout's guard: a plan
// with no real Stripe price id is not purchasable online.
func isContractPlan(p *domain.Plan) bool {
	return p.StripePriceID == nil || *p.StripePriceID == "" || *p.StripePriceID == "__trial_no_stripe__"
}

func (uc *CreateTransparentCheckoutUseCase) Execute(ctx context.Context, dto CreateTransparentCheckoutDTO) (*TransparentCheckoutResult, error) {
	account, err := uc.accountRepo.GetByID(ctx, dto.AccountID)
	if err != nil || account == nil {
		return nil, domain.ErrUnauthorized
	}

	planID := dto.PlanID
	if planID == "" && account.PlanID != nil {
		planID = *account.PlanID
	}
	if planID == "" {
		return nil, domain.ErrPlanNotFound
	}
	plan, err := uc.planRepo.GetByID(ctx, planID)
	if err != nil || plan == nil {
		return nil, domain.ErrPlanNotFound
	}

	// Non-BRL (or AbacatePay unavailable) → Stripe card checkout. Delegate
	// to the existing use case so all its guards (contract plans, coupons,
	// currency mismatch, idempotent customer) run unchanged.
	if account.Currency != domain.CurrencyBRL || !uc.abacate.Enabled() {
		url, err := uc.stripeCheckout.Execute(ctx, CreateCheckoutSessionDTO{
			AccountID:  dto.AccountID,
			PlanID:     dto.PlanID,
			SuccessURL: dto.SuccessURL,
			CancelURL:  dto.CancelURL,
			CouponCode: dto.CouponCode,
			Interval:   dto.Interval,
		})
		if err != nil {
			return nil, err
		}
		return &TransparentCheckoutResult{Provider: "stripe", Method: CheckoutMethodCard, URL: url}, nil
	}

	// BRL → AbacatePay. Contract plans (Partner/Enterprise) are sold
	// under contract and have no purchasable Stripe price — mirror the
	// same not-purchasable signal here, and require a real amount to
	// charge.
	if isContractPlan(plan) || plan.Price <= 0 {
		slog.WarnContext(ctx, "abacate checkout blocked: plan not purchasable online",
			"plan_id", plan.ID, "plan_price", plan.Price, "account_id", dto.AccountID)
		return nil, domain.ErrPlanNotPurchasable
	}

	metadata := map[string]string{
		"account_id": account.ID,
		"plan_id":    plan.ID,
		"interval":   dto.Interval,
	}
	externalID := fmt.Sprintf("acc_%s_plan_%s", account.ID, plan.ID)

	if dto.Method == CheckoutMethodCard {
		url, err := uc.abacate.CreateCardCheckout(ctx, domain.CardCheckoutInput{
			PlanName:    plan.Name,
			AmountCents: int64(plan.Price),
			ExternalID:  externalID,
			Metadata:    metadata,
			SuccessURL:  dto.SuccessURL,
			CancelURL:   dto.CancelURL,
		})
		if err != nil {
			return nil, fmt.Errorf("abacatepay card checkout: %w", err)
		}
		return &TransparentCheckoutResult{Provider: "abacatepay", Method: CheckoutMethodCard, URL: url}, nil
	}

	// Default: PIX (truly transparent — rendered inline by the app).
	charge, err := uc.abacate.CreatePixCharge(ctx, domain.PixChargeInput{
		AmountCents: int64(plan.Price),
		Description: fmt.Sprintf("Replider — plano %s", plan.Name),
		Metadata:    metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("abacatepay pix charge: %w", err)
	}
	return &TransparentCheckoutResult{Provider: "abacatepay", Method: CheckoutMethodPix, Pix: charge}, nil
}

// GetChargeStatus polls a transparent PIX charge so the app can detect
// payment without waiting on the webhook. Returns the raw AbacatePay
// status and a normalized "paid" flag.
func (uc *CreateTransparentCheckoutUseCase) GetChargeStatus(ctx context.Context, chargeID string) (status string, paid bool, err error) {
	if !uc.abacate.Enabled() {
		return "", false, domain.ErrPlanNotPurchasable
	}
	status, err = uc.abacate.GetChargeStatus(ctx, chargeID)
	if err != nil {
		return "", false, err
	}
	return status, status == "PAID" || status == "APPROVED", nil
}
