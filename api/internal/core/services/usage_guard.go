package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// UsageChecker verifies whether an account can still consume AI services
// within its plan limits. Abstracted as an interface for testability.
type UsageChecker interface {
	// Check verifies a single limit kind against pre-fetched plan and
	// usage data. Pass a non-nil account so per-account overrides
	// (Enterprise / Partner contracts) and the contract-plan
	// "0 = unlimited" rule apply; nil falls through to plan-level
	// limits with no overrides — kept for legacy callers and unit
	// tests that don't need the account context.
	Check(account *domain.Account, plan *domain.Plan, usage *domain.AccountUsage, kind string) error
	// CheckAllowance is the self-contained variant that fetches account,
	// plan, and usage before running the check.
	CheckAllowance(ctx context.Context, accountID string, kind string) error
	// CheckAccountActive returns ErrAccountSuspended if the account has
	// been past_due for longer than the grace period.
	CheckAccountActive(account *domain.Account) error
}

// UsageGuard enforces monthly conversation and token limits per plan.
type UsageGuard struct {
	usageRepo   domain.UsageRepository
	planRepo    domain.PlanRepository
	accountRepo domain.AccountRepository
}

func NewUsageGuard(
	usageRepo domain.UsageRepository,
	planRepo domain.PlanRepository,
	accountRepo domain.AccountRepository,
) *UsageGuard {
	return &UsageGuard{
		usageRepo:   usageRepo,
		planRepo:    planRepo,
		accountRepo: accountRepo,
	}
}

// Check verifies a usage limit against pre-fetched plan and usage data.
// kind must be "conversations" or "tokens". When account is non-nil
// and represents a contract plan, plan caps of 0 are treated as
// unlimited and per-account overrides take precedence over plan-level
// limits.
func (g *UsageGuard) Check(account *domain.Account, plan *domain.Plan, usage *domain.AccountUsage, kind string) error {
	switch kind {
	case "conversations":
		count := int32(0)
		if usage.ConversationsCount != nil {
			count = *usage.ConversationsCount
		}
		limit := plan.MaxConversationsPerMonth
		if account != nil {
			limit = account.EffectiveMaxConversationsPerMonth(plan)
		}
		if int(count) < limit {
			return nil
		}
		// Plan franchise exhausted. Overage-enabled plans let the
		// account keep going on a pay-per-conversation basis until the
		// cycle cap (Nx the plan price); plans without an overage rate
		// (Trial) hard-stop here. The cap is recomputed live rather
		// than read from cap_reached_at so enforcement doesn't lag the
		// daily overage worker.
		if plan.OverageConversationCents <= 0 {
			return domain.ErrUsageLimitExceeded
		}
		overageConvs := int(count) - limit
		overageCents := overageConvs * plan.OverageConversationCents
		capCents := int(math.Round(float64(plan.Price) * plan.OverageCapMultiplier))
		if capCents > 0 && overageCents >= capCents {
			return domain.ErrUsageLimitExceeded
		}
	case "tokens":
		// Contract plans (Partner / Enterprise) bring their own AI key,
		// so the platform stops accounting for tokens entirely. Skip
		// the limit check before touching addon math so a plan with
		// MaxAITokensPerMonth=0 doesn't accidentally trip the
		// "tokensUsed >= 0 == always blocked" branch.
		if account != nil && account.IsContractPlan(plan) {
			return nil
		}
		tokensUsed := int64(0)
		if usage.AITokensUsed != nil {
			tokensUsed = *usage.AITokensUsed
		}
		addonTokens := int64(0)
		if usage.AddonTokens != nil {
			addonTokens = *usage.AddonTokens
		}
		effectiveLimit := plan.MaxAITokensPerMonth + addonTokens
		// 10% grace period to avoid cutting mid-conversation
		hardLimit := effectiveLimit + (effectiveLimit / 10)
		if tokensUsed >= hardLimit {
			return domain.ErrUsageLimitExceeded
		}
	}
	return nil
}

// CheckAccountActive blocks AI access when the subscription is paused
// (suspend immediately — no grace, the customer asked to pause) or
// when the past_due grace period has expired.
func (g *UsageGuard) CheckAccountActive(account *domain.Account) error {
	if account != nil && account.SubscriptionStatus == domain.SubscriptionStatusPaused {
		return domain.ErrAccountSuspended
	}
	if account.IsPastDueGraceExpired(time.Now()) {
		return domain.ErrAccountSuspended
	}
	return nil
}

// CheckAllowance fetches account, plan, and usage, then delegates to Check.
func (g *UsageGuard) CheckAllowance(ctx context.Context, accountID string, kind string) error {
	account, err := g.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return domain.ErrAccountNotFound
	}

	var plan *domain.Plan
	if account.PlanID == nil {
		plan = domain.TrialPlan()
	} else {
		plan, err = g.planRepo.GetByID(ctx, *account.PlanID)
		if err != nil || plan == nil {
			plan = domain.TrialPlan()
		}
	}

	usage, err := g.usageRepo.GetByAccountID(ctx, accountID)
	if err != nil || usage == nil {
		return fmt.Errorf("could not fetch usage for account %s", accountID)
	}

	return g.Check(account, plan, usage, kind)
}
