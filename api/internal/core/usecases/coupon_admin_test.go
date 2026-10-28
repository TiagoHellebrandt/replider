package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type stubStripeCoupon struct {
	createdCode     string
	createdPercent  int
	deactivatedID   string
	redeemedByPromo map[string]int
	createCouponID  string
	createPromoID   string
	createErr       error
}

func (s *stubStripeCoupon) CreateCoupon(code string, percentOff int, max *int, expires int64) (string, string, error) {
	s.createdCode = code
	s.createdPercent = percentOff
	if s.createErr != nil {
		return "", "", s.createErr
	}
	return s.createCouponID, s.createPromoID, nil
}
func (s *stubStripeCoupon) DeactivatePromotionCode(promoID string) error {
	s.deactivatedID = promoID
	return nil
}
func (s *stubStripeCoupon) GetPromotionCodeRedeemed(promoID string) (int, error) {
	if s.redeemedByPromo == nil {
		return 0, nil
	}
	return s.redeemedByPromo[promoID], nil
}

func makeCouponUC(t *testing.T) (*CouponAdminUseCase, *MockCouponRepo, *stubStripeCoupon, *MockAuditLogRepo) {
	t.Helper()
	repo := new(MockCouponRepo)
	stripe := &stubStripeCoupon{createCouponID: "cp_123", createPromoID: "promo_456"}
	audit := new(MockAuditLogRepo)
	audit.On("Create", mock.Anything, mock.Anything).Return(nil).Maybe()
	return NewCouponAdminUseCase(repo, stripe, audit), repo, stripe, audit
}

func TestCouponAdmin_Create_RejectsInvalidCode(t *testing.T) {
	uc, _, _, _ := makeCouponUC(t)
	_, err := uc.Create(context.Background(), CreateCouponDTO{
		ActorEmail: "ops@x.com", Code: "ab", PercentOff: 10,
	})
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "COUPON_INVALID_CODE", appErr.Code)
}

func TestCouponAdmin_Create_RejectsBadPercent(t *testing.T) {
	uc, _, _, _ := makeCouponUC(t)
	_, err := uc.Create(context.Background(), CreateCouponDTO{
		ActorEmail: "ops@x.com", Code: "WELCOME10", PercentOff: 0,
	})
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "COUPON_INVALID_PERCENT", appErr.Code)
}

func TestCouponAdmin_Create_RejectsDuplicateCode(t *testing.T) {
	uc, repo, _, _ := makeCouponUC(t)
	repo.On("GetByCode", mock.Anything, "WELCOME10").
		Return(&domain.Coupon{ID: "existing"}, nil)
	_, err := uc.Create(context.Background(), CreateCouponDTO{
		ActorEmail: "ops@x.com", Code: "WELCOME10", PercentOff: 10,
	})
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "COUPON_CODE_TAKEN", appErr.Code)
}

func TestCouponAdmin_Create_HappyPathPersistsAndReturns(t *testing.T) {
	uc, repo, stripe, _ := makeCouponUC(t)
	repo.On("GetByCode", mock.Anything, "WELCOME10").Return(nil, nil)
	repo.On("Insert", mock.Anything, mock.AnythingOfType("*domain.Coupon")).Return(nil)
	max := 100
	expires := time.Now().Add(24 * time.Hour)

	out, err := uc.Create(context.Background(), CreateCouponDTO{
		ActorEmail:     "ops@x.com",
		Code:           "WELCOME10",
		PercentOff:     10,
		MaxRedemptions: &max,
		ExpiresAt:      &expires,
	})
	assert.NoError(t, err)
	assert.Equal(t, "WELCOME10", out.Code)
	assert.Equal(t, 10, out.PercentOff)
	assert.Equal(t, "cp_123", out.StripeCouponID)
	assert.NotNil(t, out.StripePromoID)
	assert.Equal(t, "promo_456", *out.StripePromoID)
	assert.True(t, out.IsActive)
	assert.Equal(t, 10, stripe.createdPercent)
}

func TestCouponAdmin_LookupForPublic_MissReturnsInvalid(t *testing.T) {
	uc, repo, _, _ := makeCouponUC(t)
	repo.On("GetByCode", mock.Anything, "UNKNOWN").Return(nil, nil)
	out := uc.LookupForPublic(context.Background(), "unknown")
	assert.False(t, out.Valid)
	assert.Equal(t, "UNKNOWN", out.Code)
}

func TestCouponAdmin_LookupForPublic_ExpiredReturnsInvalid(t *testing.T) {
	uc, repo, _, _ := makeCouponUC(t)
	past := time.Now().Add(-1 * time.Hour)
	repo.On("GetByCode", mock.Anything, "WELCOME10").Return(&domain.Coupon{
		Code: "WELCOME10", PercentOff: 10, IsActive: true, ExpiresAt: &past,
	}, nil)
	out := uc.LookupForPublic(context.Background(), "WELCOME10")
	assert.False(t, out.Valid)
}

func TestCouponAdmin_LookupForPublic_FullyRedeemedReturnsInvalid(t *testing.T) {
	uc, repo, _, _ := makeCouponUC(t)
	max := 5
	repo.On("GetByCode", mock.Anything, "WELCOME10").Return(&domain.Coupon{
		Code: "WELCOME10", PercentOff: 10, IsActive: true,
		MaxRedemptions: &max, RedeemedCount: 5,
	}, nil)
	out := uc.LookupForPublic(context.Background(), "WELCOME10")
	assert.False(t, out.Valid)
}

func TestCouponAdmin_LookupForPublic_HappyPath(t *testing.T) {
	uc, repo, _, _ := makeCouponUC(t)
	repo.On("GetByCode", mock.Anything, "WELCOME10").Return(&domain.Coupon{
		Code: "WELCOME10", PercentOff: 10, IsActive: true,
	}, nil)
	out := uc.LookupForPublic(context.Background(), "WELCOME10")
	assert.True(t, out.Valid)
	assert.Equal(t, 10, out.PercentOff)
}

func TestCouponAdmin_Deactivate_FlipsLocalAndStripe(t *testing.T) {
	uc, repo, stripe, _ := makeCouponUC(t)
	promo := "promo_456"
	repo.On("GetByID", mock.Anything, "c-1").Return(&domain.Coupon{
		ID: "c-1", StripePromoID: &promo,
	}, nil)
	repo.On("SetActive", mock.Anything, "c-1", false).Return(nil)
	err := uc.Deactivate(context.Background(), "ops@x.com", "c-1")
	assert.NoError(t, err)
	assert.Equal(t, "promo_456", stripe.deactivatedID)
}
