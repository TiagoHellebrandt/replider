package usecases

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/replider/api/internal/core/domain"
)

// ProcessAbacatePayWebhookUseCase activates a plan when an AbacatePay
// charge (PIX or card) is paid. AbacatePay fires `billing.paid`; the
// account + plan ride on the charge metadata we set at checkout, so no
// external lookup table is needed.
//
// Activation reuses the same account-repo primitives as the Stripe
// webhook (SetPlan + ClearTrialExpiresAt + SetSubscriptionStatus +
// ResetUsage) so both providers converge on one activated state.
type ProcessAbacatePayWebhookUseCase struct {
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
	usageRepo   domain.UsageRepository
}

func NewProcessAbacatePayWebhookUseCase(
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	usageRepo domain.UsageRepository,
) *ProcessAbacatePayWebhookUseCase {
	return &ProcessAbacatePayWebhookUseCase{
		accountRepo: accountRepo,
		planRepo:    planRepo,
		usageRepo:   usageRepo,
	}
}

func (uc *ProcessAbacatePayWebhookUseCase) Execute(ctx context.Context, event *domain.AbacateWebhookEvent) error {
	if event == nil {
		return nil
	}
	switch event.Event {
	case "billing.paid":
		return uc.handlePaid(ctx, event.Data)
	default:
		slog.InfoContext(ctx, "abacatepay webhook ignored", "event", event.Event, "charge_id", event.Data.ID)
		return nil
	}
}

func (uc *ProcessAbacatePayWebhookUseCase) handlePaid(ctx context.Context, charge domain.AbacateCharge) error {
	accountID := charge.Metadata["account_id"]
	planID := charge.Metadata["plan_id"]
	if accountID == "" || planID == "" {
		slog.ErrorContext(ctx, "abacatepay billing.paid missing metadata — cannot activate plan, reconcile manually",
			"charge_id", charge.ID, "external_id", charge.ExternalID)
		return nil
	}

	plan, err := uc.planRepo.GetByID(ctx, planID)
	if err != nil || plan == nil {
		return fmt.Errorf("abacatepay webhook: resolve plan %s (account %s): %w", planID, accountID, err)
	}

	if err := uc.accountRepo.SetPlan(ctx, accountID, plan.ID); err != nil {
		return fmt.Errorf("abacatepay webhook: set plan %s on account %s: %w", plan.ID, accountID, err)
	}
	if err := uc.accountRepo.ClearTrialExpiresAt(ctx, accountID); err != nil {
		return fmt.Errorf("abacatepay webhook: clear trial on account %s: %w", accountID, err)
	}
	if err := uc.accountRepo.SetSubscriptionStatus(ctx, accountID, "active"); err != nil {
		return fmt.Errorf("abacatepay webhook: set subscription active on account %s: %w", accountID, err)
	}
	// Reset the usage counters for the new cycle — mirrors the Stripe
	// invoice.paid handler. Best-effort: a reset failure must not bounce
	// the webhook (AbacatePay would retry and re-activate needlessly).
	if uc.usageRepo != nil {
		if err := uc.usageRepo.ResetUsage(ctx, accountID); err != nil {
			slog.WarnContext(ctx, "abacatepay webhook: usage reset failed", "account_id", accountID, "error", err)
		}
	}

	slog.InfoContext(ctx, "abacatepay activated plan",
		"account_id", accountID, "plan_id", plan.ID, "plan_name", plan.Name, "charge_id", charge.ID)
	return nil
}
