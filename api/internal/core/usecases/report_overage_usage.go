package usecases

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/replider/api/internal/core/domain"
)

// OverageMeterPrices holds the Stripe `metered` price ids for each
// overage product (US-168). An empty id disables reporting for that
// meter — the reporter simply skips it, so a partial rollout (e.g.
// conversations only) is safe.
type OverageMeterPrices struct {
	ExtraConversation string
	ExtraToken        string
	ExtraAgent        string
}

// configured reports whether at least one meter price is set. When none
// are, the whole overage subsystem is dormant and the worker can skip
// the account-listing query entirely.
func (p OverageMeterPrices) configured() bool {
	return p.ExtraConversation != "" || p.ExtraToken != "" || p.ExtraAgent != ""
}

// ReportOverageUsageUseCase computes how far an account has exceeded its
// plan allowances this billing cycle and reports the excess to Stripe
// usage-based metered billing (US-168). It replaces the old "block on
// limit" behaviour with "bill the overage": every cycle the worker
// reports the running total for conversations, tokens and agents over
// the plan franchise, and Stripe invoices the difference at the metered
// rate.
//
// Reporting uses action=`set` (the running cycle total), so a repeated
// tick is idempotent — re-reporting the same total is a no-op for the
// invoice. The metered prices must be configured with
// `aggregate_usage=last_during_period` (see docs/billing.md).
type ReportOverageUsageUseCase struct {
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
	usageRepo   domain.UsageRepository
	agentRepo   domain.AgentRepository
	payment     domain.PaymentAdapter
	prices      OverageMeterPrices
}

func NewReportOverageUsageUseCase(
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	usageRepo domain.UsageRepository,
	agentRepo domain.AgentRepository,
	payment domain.PaymentAdapter,
	prices OverageMeterPrices,
) *ReportOverageUsageUseCase {
	return &ReportOverageUsageUseCase{
		accountRepo: accountRepo,
		planRepo:    planRepo,
		usageRepo:   usageRepo,
		agentRepo:   agentRepo,
		payment:     payment,
		prices:      prices,
	}
}

// Execute reports the current overage for one account and returns the
// number of meters reported (those with a configured price). It is a
// no-op returning 0 when the account isn't on an active self-serve
// subscription, or when no meter prices are configured.
func (uc *ReportOverageUsageUseCase) Execute(ctx context.Context, accountID string) (int, error) {
	if !uc.prices.configured() {
		return 0, nil
	}

	account, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if account == nil {
		return 0, domain.ErrAccountNotFound
	}

	// Only self-serve, actively-subscribed accounts are billed for
	// overage. Trial / contract / past_due accounts are governed by the
	// existing access rules, not metered billing.
	if account.SubscriptionStatus != domain.SubscriptionStatusActive {
		return 0, nil
	}
	if account.StripeSubscriptionID == nil || *account.StripeSubscriptionID == "" {
		return 0, nil
	}
	if account.PlanID == nil {
		return 0, nil
	}

	plan, err := uc.planRepo.GetByID(ctx, *account.PlanID)
	if err != nil {
		return 0, err
	}
	if plan == nil {
		return 0, nil
	}

	usage, err := uc.usageRepo.GetByAccountID(ctx, accountID)
	if err != nil {
		return 0, err
	}

	subID := *account.StripeSubscriptionID
	cycle := int64(0)
	if usage != nil {
		cycle = usage.BillingCycleStart.Unix()
	}

	// Conversations overage.
	convUsed := int64(0)
	tokensUsed := int64(0)
	addonTokens := int64(0)
	if usage != nil {
		if usage.ConversationsCount != nil {
			convUsed = int64(*usage.ConversationsCount)
		}
		if usage.AITokensUsed != nil {
			tokensUsed = *usage.AITokensUsed
		}
		if usage.AddonTokens != nil {
			addonTokens = *usage.AddonTokens
		}
	}

	extraConversations := overage(convUsed, int64(account.EffectiveMaxConversationsPerMonth(plan)))
	// The addon bucket extends the token franchise before overage kicks in.
	extraTokens := overage(tokensUsed, account.EffectiveMaxAITokensPerMonth(plan)+addonTokens)

	// Agents overage. Agents are a live (not accumulated) resource, so we
	// report the current count over the cap.
	agents, err := uc.agentRepo.ListByAccountID(ctx, accountID)
	if err != nil {
		return 0, err
	}
	extraAgents := overage(int64(len(agents)), int64(account.EffectiveMaxAgents(plan)))

	meters := []struct {
		name     string
		priceID  string
		quantity int64
	}{
		{"extra_conversation", uc.prices.ExtraConversation, extraConversations},
		{"extra_token", uc.prices.ExtraToken, extraTokens},
		{"extra_agent", uc.prices.ExtraAgent, extraAgents},
	}

	reported := 0
	for _, m := range meters {
		if m.priceID == "" {
			continue
		}
		idemKey := fmt.Sprintf("overage_%s_%s_%d", m.name, accountID, cycle)
		if err := uc.payment.ReportMeteredUsage(subID, m.priceID, m.quantity, idemKey); err != nil {
			// Don't abort the whole account on one meter — report what we
			// can and surface the failure. A transient Stripe error is
			// retried on the next worker tick (action=set is idempotent).
			slog.Warn("overage: failed to report meter",
				"account_id", accountID, "meter", m.name, "quantity", m.quantity, "error", err)
			return reported, err
		}
		reported++
	}

	return reported, nil
}

// overage returns max(0, used-limit). A limit <= 0 is treated as "no
// overage" (unlimited / unconfigured caps never bill).
func overage(used, limit int64) int64 {
	if limit <= 0 {
		return 0
	}
	if used <= limit {
		return 0
	}
	return used - limit
}
