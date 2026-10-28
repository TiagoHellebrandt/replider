package usecases

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// ErrTrialExpired is the refusal for an account whose free trial has ended.
// Same code the HTTP subscription middleware answers (402 TRIAL_EXPIRED), so
// the non-HTTP paths — inbound AI replies, overnight drafts, MCP tools —
// refuse with the same vocabulary instead of silently spending.
var ErrTrialExpired = &domain.AppError{
	Code:    "TRIAL_EXPIRED",
	Message: "your free trial has ended — upgrade to continue",
}

// ErrSubscriptionInactive mirrors the middleware's SUBSCRIPTION_INACTIVE.
var ErrSubscriptionInactive = &domain.AppError{
	Code:    "SUBSCRIPTION_INACTIVE",
	Message: "your subscription is not active — please update your payment method",
}

// accountMayUseAI is the pure rule: may this account spend AI right now?
//
// It is the subscription middleware's decision table minus the HTTP: owner
// suspension first, then the bypass flag (owner self-test + Meta reviewer
// accounts), then the subscription status. Trial expiry lived ONLY in the
// middleware until 2026-09-01, so an expired trial kept getting AI replies
// through the webhook path and drafts overnight.
func accountMayUseAI(account *domain.Account, now time.Time) error {
	if account == nil {
		return domain.ErrAccountNotFound
	}
	if account.IsSuspended() {
		return domain.ErrAccountSuspended
	}
	if account.TrialBypass {
		return nil
	}
	switch account.SubscriptionStatus {
	case domain.SubscriptionStatusActive, "":
		// "" never comes from the DB (column default 'trialing'); it is a
		// legacy/in-memory row and refusing it would only break fixtures.
		return nil
	case domain.SubscriptionStatusTrialing:
		if account.IsTrialExpired(now) {
			return ErrTrialExpired
		}
		return nil
	case domain.SubscriptionStatusPastDue:
		// The past-due GRACE window is owned by UsageGuard.CheckAccountActive
		// (7 days); this helper only refuses what is unambiguously over.
		if account.IsPastDueGraceExpired(now) {
			return domain.ErrAccountSuspended
		}
		return nil
	default:
		return ErrSubscriptionInactive
	}
}

// AccountMayUseAI loads the account and applies accountMayUseAI. Exported
// for callers outside this package that spend AI without passing through
// the HTTP subscription middleware — the MCP mount in particular (the
// identity middleware in adapters/mcp should call this before dispatching
// a tool that drafts or generates).
//
// A lookup failure refuses: unlike the spend caps this is a billing
// boundary, and "we could not tell whether you are paying" is not a reason
// to spend on the platform's behalf.
func AccountMayUseAI(ctx context.Context, accounts domain.AccountRepository, accountID string) error {
	if accounts == nil {
		return nil
	}
	account, err := accounts.GetByID(ctx, accountID)
	if err != nil {
		return err
	}
	return accountMayUseAI(account, time.Now())
}
