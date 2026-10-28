package domain

import "time"

// Referral status values.
const (
	ReferralStatusPending  = "pending"
	ReferralStatusRewarded = "rewarded"
)

// Referral records that one account (referrer) brought in another
// (referee) via a ?ref=CODE signup link, and tracks the "1 month free
// for both" reward state (US-174). The reward fires once, when the
// referee makes their first paid subscription.
type Referral struct {
	ID                string     `json:"id"`
	ReferrerAccountID string     `json:"referrer_account_id"`
	RefereeAccountID  string     `json:"referee_account_id"`
	Status            string     `json:"status"`
	RewardAmountCents int64      `json:"reward_amount_cents"`
	RewardCurrency    string     `json:"reward_currency"`
	CreatedAt         time.Time  `json:"created_at"`
	RewardedAt        *time.Time `json:"rewarded_at,omitempty"`
}

// ReferralStats is the per-account summary surfaced on GET /me/referral
// so the dashboard can render "X friends joined, Y rewards earned".
type ReferralStats struct {
	// Total accounts that signed up with this account's referral code.
	TotalReferred int `json:"total_referred"`
	// Of those, how many converted to a paid plan and triggered a reward.
	TotalRewarded int `json:"total_rewarded"`
}
