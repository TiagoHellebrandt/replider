package usecases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/replider/api/internal/core/domain"
)

// A valid Facebook token minted for SOME OTHER app must never grant a Replider
// session: /me validates any valid FB token, so only the /debug_token app_id
// check stops the confused-deputy account takeover.
func TestMetaAuth_RejectsTokenIssuedForAnotherApp(t *testing.T) {
	ctx := context.Background()

	origVerify := verifyMetaTokenFunc
	origBinding := verifyMetaAppBindingFunc
	t.Cleanup(func() {
		verifyMetaTokenFunc = origVerify
		verifyMetaAppBindingFunc = origBinding
	})

	// The token belongs to the victim (so /me would happily identify them)...
	verifyMetaTokenFunc = func(string) (*MetaTokenInfo, error) {
		return &MetaTokenInfo{ID: "victim-meta-id", Email: "victim@facebook.com"}, nil
	}
	// ...but /debug_token reveals it was issued for a DIFFERENT app.
	verifyMetaAppBindingFunc = func(appID, appSecret, token string) (*MetaTokenDebugInfo, error) {
		return &MetaTokenDebugInfo{AppID: "attacker-other-app", IsValid: true}, nil
	}

	uc := NewMetaAuthUseCase(new(MockUserRepo), new(MockAccountRepo), new(MockUsageRepo),
		new(MockTxManager), new(MockJWTService))
	uc.SetMetaAppCredentials("our-app-id", "our-app-secret")

	resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "victims-token-from-a-quiz-app"})

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, domain.ErrSocialAuthFailed)
}

// A token our app minted, once app-binding confirms it, proceeds normally.
func TestMetaAuth_AcceptsTokenIssuedForOurApp(t *testing.T) {
	ctx := context.Background()

	origVerify := verifyMetaTokenFunc
	origBinding := verifyMetaAppBindingFunc
	t.Cleanup(func() {
		verifyMetaTokenFunc = origVerify
		verifyMetaAppBindingFunc = origBinding
	})

	verifyMetaTokenFunc = func(string) (*MetaTokenInfo, error) {
		return &MetaTokenInfo{ID: "meta-ok", Email: "user@facebook.com"}, nil
	}
	verifyMetaAppBindingFunc = func(appID, appSecret, token string) (*MetaTokenDebugInfo, error) {
		return &MetaTokenDebugInfo{AppID: "our-app-id", IsValid: true}, nil
	}

	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	jwtService := new(MockJWTService)
	existing := &domain.User{ID: "u1", AccountID: "acc1", Email: "user@facebook.com"}
	userRepo.On("GetByMetaID", ctx, "meta-ok").Return(existing, nil)
	jwtService.On("GenerateToken", existing).Return("jwt-ok", nil)
	accountRepo.On("GetByID", ctx, "acc1").Return(&domain.Account{ID: "acc1", OnboardingCompleted: true}, nil)

	uc := NewMetaAuthUseCase(userRepo, accountRepo, new(MockUsageRepo), new(MockTxManager), jwtService)
	uc.SetMetaAppCredentials("our-app-id", "our-app-secret")

	resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "our-token"})

	assert.NoError(t, err)
	assert.Equal(t, "jwt-ok", resp.Token)
}

// An invalid token (revoked / expired) reported by /debug_token is rejected
// even if its app_id matches.
func TestMetaAuth_RejectsInvalidToken(t *testing.T) {
	ctx := context.Background()

	origVerify := verifyMetaTokenFunc
	origBinding := verifyMetaAppBindingFunc
	t.Cleanup(func() {
		verifyMetaTokenFunc = origVerify
		verifyMetaAppBindingFunc = origBinding
	})

	verifyMetaTokenFunc = func(string) (*MetaTokenInfo, error) {
		return &MetaTokenInfo{ID: "x", Email: "x@facebook.com"}, nil
	}
	verifyMetaAppBindingFunc = func(appID, appSecret, token string) (*MetaTokenDebugInfo, error) {
		return &MetaTokenDebugInfo{AppID: "our-app-id", IsValid: false}, nil
	}

	uc := NewMetaAuthUseCase(new(MockUserRepo), new(MockAccountRepo), new(MockUsageRepo),
		new(MockTxManager), new(MockJWTService))
	uc.SetMetaAppCredentials("our-app-id", "our-app-secret")

	resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "revoked"})
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, domain.ErrSocialAuthFailed)
}
