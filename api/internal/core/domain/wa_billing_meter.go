package domain

import (
	"context"
	"time"
)

// WABillingMeterRollup is one row of the per-(account, category,
// billing_period) aggregate the Stripe push worker emits. Quantity
// is the count of distinct billable wa_conversation_ids; the worker
// pushes that as a usage_record against the matching subscription_item.
type WABillingMeterRollup struct {
	AccountID                  string
	PricingCategory            string
	BillingPeriod              time.Time
	BillableCount              int
	TotalCount                 int
	StripeMarketingItemID      *string
	StripeUtilityItemID        *string
	StripeAuthenticationItemID *string
	StripeServiceItemID        *string
}

// SubscriptionItemID resolves the right Stripe item for the row's
// pricing_category. Returns "" when the plan doesn't meter that
// category (the conversation is then absorbed into the flat plan
// rate). Solution Partners tied to non-Stripe billing also leave
// these blank.
func (r *WABillingMeterRollup) SubscriptionItemID() string {
	switch r.PricingCategory {
	case "marketing":
		if r.StripeMarketingItemID != nil {
			return *r.StripeMarketingItemID
		}
	case "utility":
		if r.StripeUtilityItemID != nil {
			return *r.StripeUtilityItemID
		}
	case "authentication":
		if r.StripeAuthenticationItemID != nil {
			return *r.StripeAuthenticationItemID
		}
	case "service":
		if r.StripeServiceItemID != nil {
			return *r.StripeServiceItemID
		}
	}
	return ""
}

// WABillingMeterRepository persists per-conversation pricing rollup
// rows so the Stripe push worker can deduplicate across daily ticks.
type WABillingMeterRepository interface {
	// Aggregate materializes new rows for every distinct
	// (account, conversation, category) tuple seen between
	// periodStart (inclusive) and periodEnd (exclusive). Idempotent
	// at the SQL level via ON CONFLICT DO NOTHING.
	Aggregate(ctx context.Context, periodStart, periodEnd time.Time) error
	// ListPending returns un-metered rollups joined with the
	// account's plan so the worker has the Stripe item IDs ready.
	ListPending(ctx context.Context, batchSize int) ([]*WABillingMeterRollup, error)
	// MarkPushed stamps metered_at on every row in the
	// (account, category, billing_period) tuple. Returns the row
	// count for ops logging.
	MarkPushed(ctx context.Context, accountID, pricingCategory string, period time.Time) (int64, error)
}
