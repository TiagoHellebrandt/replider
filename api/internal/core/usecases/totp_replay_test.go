package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTOTPReplayGuard_RefusesTheSameStepTwice(t *testing.T) {
	secret := newTestSecret(t)
	g := newTOTPReplayGuard()
	now := time.Date(2026, 9, 1, 12, 0, 15, 0, time.UTC)
	g.now = func() time.Time { return now }

	code, _ := totp.GenerateCode(secret, now)
	assert.True(t, g.accept("u1", code, secret), "first use")
	assert.False(t, g.accept("u1", code, secret), "replay inside the same step")
	assert.True(t, g.accept("u2", code, secret), "another user is a different counter")

	// The previous step's code (still inside the skew) is older than the
	// one already accepted: refused.
	older, _ := totp.GenerateCode(secret, now.Add(-totpStep))
	assert.False(t, g.accept("u1", older, secret))

	// The next step's code is newer: accepted, and then it too is spent.
	now = now.Add(totpStep)
	next, _ := totp.GenerateCode(secret, now)
	assert.True(t, g.accept("u1", next, secret))
	assert.False(t, g.accept("u1", next, secret))

	assert.False(t, g.accept("u3", "000000", secret), "a wrong code is still wrong")
}

// End to end through the login pivot: the second presentation of a code
// that just worked is TOTP_INVALID_CODE.
func TestTwoFactor_VerifyForLogin_RefusesReplayedCode(t *testing.T) {
	uc, userRepo := makeTwoFactorUC(t)
	secret := newTestSecret(t)
	userRepo.On("GetByID", mock.Anything, "u-1").Return(&domain.User{
		ID: "u-1", TOTPSecret: &secret, TOTPEnabled: true,
	}, nil)
	userRepo.On("StampTOTPVerified", mock.Anything, "u-1").Return(nil)

	code, _ := totp.GenerateCode(secret, time.Now())
	assert.NoError(t, uc.VerifyForLogin(context.Background(), "u-1", code))

	err := uc.VerifyForLogin(context.Background(), "u-1", code)
	var appErr *domain.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, "TOTP_INVALID_CODE", appErr.Code)
}
