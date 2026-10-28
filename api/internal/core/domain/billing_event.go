package domain

import (
	"context"
	"time"
)

// Billing event types — the revenue-affecting Stripe events we ledger
// for MRR/churn/LTV analytics (US-071).
const (
	// BillingEventSubscriptionCreated is a first successful checkout.
	BillingEventSubscriptionCreated = "subscription_created"
	// BillingEventSubscriptionRenewed is a recurring invoice payment.
	BillingEventSubscriptionRenewed = "subscription_renewed"
	// BillingEventChurned is a subscription cancellation (amount 0).
	BillingEventChurned = "churned"
	// BillingEventPlanUpgraded records a paid-plan tier increase
	// (US-008, vendedores v3): the account moved to a plan with a higher
	// price. Recorded wherever plan_id changes (owner apply-contract
	// today); the commission recompute reads it to grant the upgrade
	// bonus. amount_cents is the NEW plan's monthly price.
	BillingEventPlanUpgraded = "plan_upgraded"
	// BillingEventPlanActivationFailed is an ALERT row: a paid checkout
	// completed but the Stripe price id could not be mapped to a plan, so
	// the account was left on its previous (likely trial) limits. Someone
	// paid and didn't get their plan — reconcile manually and fix the
	// plans.stripe_price_id wiring. amount_cents is what was paid.
	BillingEventPlanActivationFailed = "plan_activation_failed"
)

// BillingEvent is one immutable row in the revenue ledger. The table is
// append-only — analytics derive from the sequence, never from mutating
// a row in place.
type BillingEvent struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"account_id"`
	EventType   string    `json:"event_type"`
	Plan        string    `json:"plan"`
	AmountCents int64     `json:"amount_cents"`
	Currency    string    `json:"currency"`
	OccurredAt  time.Time `json:"occurred_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// BillingMetricsAggregate holds the raw counts the repository computes in
// SQL. Derived ratios (churn rate, ARPA, LTV) are calculated in the use
// case so the math stays testable without a DB.
type BillingMetricsAggregate struct {
	// MRRBRLCents is the sum of the latest non-churn amount per BRL
	// account (empty legacy currency counts as BRL). Kept separate from
	// USD (US-018 review fixes): cents of different currencies must
	// never be added raw — the use case converts at the owner rate.
	MRRBRLCents int64
	// MRRUSDCents is the same sum for USD accounts, in USD cents.
	MRRUSDCents int64
	// ActiveAccounts is the number of accounts whose latest event is not
	// a churn.
	ActiveAccounts int
	// OtherCurrencyAccounts counts active accounts whose latest event
	// carries an unsupported currency — excluded from both MRR sums.
	OtherCurrencyAccounts int
	// ChurnedInWindow is the count of churn events in the lookback window.
	ChurnedInWindow int
}

// BillingEventRepository is the port for the revenue ledger.
type BillingEventRepository interface {
	Create(ctx context.Context, e *BillingEvent) error
	// Aggregate returns MRR / active-account / churn counts. `since`
	// bounds the churn-window count only; MRR/active reflect current
	// state across all history.
	Aggregate(ctx context.Context, since time.Time) (*BillingMetricsAggregate, error)
}
