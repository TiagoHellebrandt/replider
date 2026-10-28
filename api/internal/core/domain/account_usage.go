package domain

import (
	"time"
)

// AccountUsage tracks usage metrics per account per billing cycle
type AccountUsage struct {
	ID                 string    `json:"id"`
	AccountID          string    `json:"account_id"`
	BillingCycleStart  time.Time `json:"billing_cycle_start"`
	ConversationsCount *int32    `json:"conversations_count"`
	AITokensUsed       *int64    `json:"ai_tokens_used"`
	AIInputTokensUsed  *int64    `json:"ai_input_tokens_used"`
	AIOutputTokensUsed *int64    `json:"ai_output_tokens_used"`
	AddonTokens        *int64    `json:"addon_tokens"`
	// DailyTokens is the rolling per-day AI token counter (US-025); it
	// resets lazily when DailyTokensDate rolls to a new day. DailyTokensDate
	// is nil until the account's first AI call.
	DailyTokens           int64      `json:"daily_tokens"`
	DailyTokensDate       *time.Time `json:"daily_tokens_date,omitempty"`
	StorageUsedMB         *int32     `json:"storage_used_mb"`
	BroadcastMessagesUsed *int32     `json:"broadcast_messages_used"`
	Warned80Percent       bool       `json:"warned_80_percent"`
	Warned100Percent      bool       `json:"warned_100_percent"`
	// Overage metering (see docs/OVERAGE_METERING_DESIGN.md).
	// OverageConversationsCount / OverageCentsCharged are the running
	// per-cycle tallies the overage worker advances as it bills Stripe.
	// CapReachedAt is stamped once the cycle cap (Nx plan price) is hit;
	// while non-nil the usage guard hard-stops further conversations.
	OverageConversationsCount int32      `json:"overage_conversations_count"`
	OverageCentsCharged       int32      `json:"overage_cents_charged"`
	CapReachedAt              *time.Time `json:"cap_reached_at,omitempty"`
	UpdatedAt                 time.Time  `json:"updated_at"`
}

// EffectiveDailyTokens returns the tokens consumed TODAY. The stored
// counter only means "today" when DailyTokensDate is the current date;
// on a fresh day (before the next AddUsage rolls it) it's stale, so we
// report 0. (US-025)
func (u *AccountUsage) EffectiveDailyTokens(now time.Time) int64 {
	if u == nil || u.DailyTokensDate == nil {
		return 0
	}
	y1, m1, d1 := u.DailyTokensDate.Date()
	y2, m2, d2 := now.Date()
	if y1 == y2 && m1 == m2 && d1 == d2 {
		return u.DailyTokens
	}
	return 0
}

// Overage charge statuses — see the overage_charges table comment.
const (
	OverageChargePending     = "pending"
	OverageChargeCharged     = "charged"
	OverageChargeArchived    = "archived"
	OverageChargeNeedsReview = "needs_review"
)

// OverageCharge is one journalled Stripe overage invoice item. The
// ledger of these rows is the settler's source of truth for what has
// been billed this cycle, making the bill-then-record step crash-safe
// (see the C2 note in docs/OVERAGE_METERING_DESIGN.md).
type OverageCharge struct {
	ID                        string
	AccountID                 string
	AmountCents               int32
	RunningTotalCents         int32
	OverageConversationsCount int32
	Capped                    bool
	Status                    string
	CreatedAt                 time.Time
	ChargedAt                 *time.Time
}

// OverageCandidate is a flattened account+plan+usage row the overage
// meter worker iterates each cycle. It carries everything needed to
// compute the cycle-to-date overage charge without further lookups.
type OverageCandidate struct {
	AccountID        string
	StripeCustomerID string
	// Currency is the account's billing currency ('brl'/'usd') — the
	// overage invoice item must be created in it (US-003).
	Currency                 string
	PlanPriceCents           int
	MaxConversationsPerMonth int
	OverageConversationCents int
	OverageCapMultiplier     float64
	ConversationsCount       int32
	OverageCentsCharged      int32
	CapReached               bool
}
