package domain

import (
	"context"
	"time"
)

// Coupon is the local registry record for a Stripe-backed discount
// link. The percent off + max redemptions live in Stripe; this row
// pairs them with the public-facing code so the register flow can
// pre-fill `?coupon=` and the owner panel can render a list without
// a Stripe round-trip per row.
type Coupon struct {
	ID             string     `json:"id"`
	Code           string     `json:"code"`
	StripeCouponID string     `json:"stripe_coupon_id"`
	StripePromoID  *string    `json:"stripe_promo_id,omitempty"`
	PercentOff     int        `json:"percent_off"`
	MaxRedemptions *int       `json:"max_redemptions,omitempty"`
	RedeemedCount  int        `json:"redeemed_count"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	IsActive       bool       `json:"is_active"`
	CreatedBy      string     `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// ErrCouponExhausted is returned at checkout when a coupon's live Stripe
// redemption count has reached its max_redemptions cap — blocking the
// infinite-discount exploit before a session is created (US-013).
var ErrCouponExhausted = &AppError{Code: "COUPON_EXHAUSTED", Message: "this coupon has reached its redemption limit"}

// CouponRepository persists the local coupon registry.
type CouponRepository interface {
	Insert(ctx context.Context, c *Coupon) error
	GetByCode(ctx context.Context, code string) (*Coupon, error)
	GetByID(ctx context.Context, id string) (*Coupon, error)
	List(ctx context.Context, limit, offset int) ([]*Coupon, error)
	SetActive(ctx context.Context, id string, active bool) error
	BumpRedeemed(ctx context.Context, id string, count int) error
}
