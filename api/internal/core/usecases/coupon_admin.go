package usecases

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// StripeCouponAdapter is the slim port the use case needs from the
// Stripe payment adapter. Lives here so core/ doesn't import
// internal/adapters/payment.
type StripeCouponAdapter interface {
	CreateCoupon(code string, percentOff int, maxRedemptions *int, expiresAt int64) (couponID string, promoID string, err error)
	DeactivatePromotionCode(promoID string) error
	GetPromotionCodeRedeemed(promoID string) (int, error)
}

// CouponAdminUseCase exposes the owner-panel CRUD over the Stripe-
// backed coupon registry. Public-facing redemption goes through
// CreateCheckoutSessionUseCase, which resolves the local code → Stripe
// promo id at checkout time.
type CouponAdminUseCase struct {
	repo     domain.CouponRepository
	stripe   StripeCouponAdapter
	auditLog domain.AuditLogRepository
}

func NewCouponAdminUseCase(repo domain.CouponRepository, stripe StripeCouponAdapter, auditLog domain.AuditLogRepository) *CouponAdminUseCase {
	return &CouponAdminUseCase{repo: repo, stripe: stripe, auditLog: auditLog}
}

// CreateCouponDTO captures the operator-facing form fields.
type CreateCouponDTO struct {
	ActorEmail     string
	Code           string
	PercentOff     int
	MaxRedemptions *int
	ExpiresAt      *time.Time
}

var couponCodeRegex = regexp.MustCompile(`^[A-Z0-9_-]{3,32}$`)

// Create stages a new coupon: validates → creates Stripe Coupon +
// PromotionCode → persists the local row. Returns the freshly-created
// domain.Coupon. The Stripe and DB calls are deliberately not wrapped
// in a tx — the second can fail and leave a dangling Stripe Coupon,
// which is acceptable because the operator can simply retry with a
// new code (Stripe coupons cost nothing).
func (uc *CouponAdminUseCase) Create(ctx context.Context, dto CreateCouponDTO) (*domain.Coupon, error) {
	dto.Code = strings.ToUpper(strings.TrimSpace(dto.Code))
	if !couponCodeRegex.MatchString(dto.Code) {
		return nil, &domain.AppError{Code: "COUPON_INVALID_CODE", Message: "Code must be 3-32 characters: A-Z, 0-9, '-' or '_'"}
	}
	if dto.PercentOff < 1 || dto.PercentOff > 100 {
		return nil, &domain.AppError{Code: "COUPON_INVALID_PERCENT", Message: "Percent must be between 1 and 100"}
	}
	if dto.MaxRedemptions != nil && *dto.MaxRedemptions < 1 {
		return nil, &domain.AppError{Code: "COUPON_INVALID_MAX", Message: "Max redemptions must be a positive integer"}
	}
	existing, err := uc.repo.GetByCode(ctx, dto.Code)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, &domain.AppError{Code: "COUPON_CODE_TAKEN", Message: "This code is already in use"}
	}
	expiresAt := int64(0)
	if dto.ExpiresAt != nil {
		expiresAt = dto.ExpiresAt.Unix()
	}
	stripeCouponID, promoID, err := uc.stripe.CreateCoupon(dto.Code, dto.PercentOff, dto.MaxRedemptions, expiresAt)
	if err != nil {
		return nil, err
	}
	c := &domain.Coupon{
		ID:             ulid.Make().String(),
		Code:           dto.Code,
		StripeCouponID: stripeCouponID,
		StripePromoID:  &promoID,
		PercentOff:     dto.PercentOff,
		MaxRedemptions: dto.MaxRedemptions,
		ExpiresAt:      dto.ExpiresAt,
		IsActive:       true,
		CreatedBy:      dto.ActorEmail,
	}
	if err := uc.repo.Insert(ctx, c); err != nil {
		return nil, err
	}
	if uc.auditLog != nil {
		_ = uc.auditLog.Create(ctx, &domain.AuditLog{
			ActorID:    &dto.ActorEmail,
			Action:     "coupon.create",
			EntityType: "coupon",
			EntityID:   c.ID,
		})
	}
	return c, nil
}

// List returns the registry rows. The owner panel calls this each
// time the screen mounts; we refresh the redeemed_count from Stripe
// in a best-effort loop so stale local counts get corrected without
// a manual sync button.
func (uc *CouponAdminUseCase) List(ctx context.Context, limit, offset int) ([]*domain.Coupon, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := uc.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	if uc.stripe == nil {
		return rows, nil
	}
	for _, row := range rows {
		if row.StripePromoID == nil || *row.StripePromoID == "" {
			continue
		}
		count, err := uc.stripe.GetPromotionCodeRedeemed(*row.StripePromoID)
		if err != nil || count == row.RedeemedCount {
			continue
		}
		row.RedeemedCount = count
		_ = uc.repo.BumpRedeemed(ctx, row.ID, count)
	}
	return rows, nil
}

// Deactivate flips is_active=false locally and tells Stripe to stop
// honouring further redemptions. Existing subscriptions keep their
// discount because we never delete the underlying Coupon — only the
// PromotionCode that exposed the slug.
func (uc *CouponAdminUseCase) Deactivate(ctx context.Context, actorEmail, id string) error {
	c, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if c == nil {
		return &domain.AppError{Code: "COUPON_NOT_FOUND", Message: "Coupon not found"}
	}
	if c.StripePromoID != nil {
		_ = uc.stripe.DeactivatePromotionCode(*c.StripePromoID)
	}
	if err := uc.repo.SetActive(ctx, id, false); err != nil {
		return err
	}
	if uc.auditLog != nil {
		_ = uc.auditLog.Create(ctx, &domain.AuditLog{
			ActorID:    &actorEmail,
			Action:     "coupon.deactivate",
			EntityType: "coupon",
			EntityID:   id,
		})
	}
	return nil
}

// PublicLookupResult is the slim shape returned from the unauthenticated
// /v1/coupons/:code endpoint — just enough for the register page to
// render "X% off applied" without leaking Stripe ids or audit fields.
type PublicLookupResult struct {
	Code       string `json:"code"`
	PercentOff int    `json:"percent_off"`
	Valid      bool   `json:"valid"`
}

// LookupForPublic resolves a public code into the discount metadata
// the register / billing page needs. Returns Valid=false on any miss
// (unknown / inactive / expired / fully redeemed) so the front-end
// can show a soft warning without leaking which axis failed.
func (uc *CouponAdminUseCase) LookupForPublic(ctx context.Context, code string) *PublicLookupResult {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return &PublicLookupResult{Code: code, Valid: false}
	}
	c, err := uc.repo.GetByCode(ctx, code)
	if err != nil || c == nil {
		return &PublicLookupResult{Code: code, Valid: false}
	}
	valid := c.IsActive
	if c.ExpiresAt != nil && time.Now().After(*c.ExpiresAt) {
		valid = false
	}
	if c.MaxRedemptions != nil && c.RedeemedCount >= *c.MaxRedemptions {
		valid = false
	}
	return &PublicLookupResult{
		Code:       c.Code,
		PercentOff: c.PercentOff,
		Valid:      valid,
	}
}
