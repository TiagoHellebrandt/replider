package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// US-F001: REGISTRATION_ENABLED=false must close EVERY account-creating
// path — password signup and both social providers — while leaving login
// for existing users untouched.
func TestRegistrationPaused(t *testing.T) {
	ctx := context.Background()

	t.Run("password signup is refused before touching the DB", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		emailSvc := new(MockEmailService)

		uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailSvc, "http://localhost:8080")
		uc.SetRegistrationPaused(true)

		err := uc.Execute(ctx, CreateAccountDTO{
			AccountName:   "Acme Corp",
			AdminName:     "Admin",
			AdminEmail:    "admin@acme.com",
			Password:      "securepassword123",
			TermsAccepted: true,
		})

		assert.ErrorIs(t, err, domain.ErrRegistrationPaused)
		// No lookup, no writes, no verification mail.
		userRepo.AssertNotCalled(t, "GetByEmail", mock.Anything, mock.Anything)
		accountRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		usageRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	t.Run("password signup proceeds when registration is open", func(t *testing.T) {
		accountRepo := new(MockAccountRepo)
		userRepo := new(MockUserRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		emailSvc := new(MockEmailService)
		emailSvc.On("SendVerificationEmail", mock.AnythingOfType("string"), mock.AnythingOfType("string"), "http://localhost:8080", mock.AnythingOfType("string")).Return(nil).Maybe()

		uc := NewCreateAccountUseCase(accountRepo, userRepo, usageRepo, txManager, emailSvc, "http://localhost:8080")
		uc.SetRegistrationPaused(false)

		userRepo.On("GetByEmail", ctx, "admin@acme.com").Return(nil, nil)
		accountRepo.On("Create", ctx, mock.AnythingOfType("*domain.Account")).Return(nil)
		userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).Return(nil)
		usageRepo.On("Create", ctx, mock.AnythingOfType("*domain.AccountUsage")).Return(nil)
		userRepo.On("SetVerificationToken", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)

		err := uc.Execute(ctx, CreateAccountDTO{
			AccountName:   "Acme Corp",
			AdminName:     "Admin",
			AdminEmail:    "admin@acme.com",
			Password:      "securepassword123",
			TermsAccepted: true,
		})

		assert.NoError(t, err)
		accountRepo.AssertExpectations(t)
	})

	t.Run("google sign-up of an unknown email is refused", func(t *testing.T) {
		originalVerify := verifyGoogleTokenFunc
		t.Cleanup(func() { verifyGoogleTokenFunc = originalVerify })
		verifyGoogleTokenFunc = func(string) (*GoogleTokenInfo, error) {
			return &GoogleTokenInfo{
				Sub:           "google-999",
				Aud:           "my-client-id",
				Email:         "stranger@gmail.com",
				EmailVerified: "true",
				Name:          "Stranger",
			}, nil
		}

		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		userRepo.On("GetByGoogleID", ctx, "google-999").Return(nil, nil)
		userRepo.On("GetByEmail", ctx, "stranger@gmail.com").Return(nil, nil)

		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "my-client-id")
		uc.SetRegistrationPaused(true)

		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "valid-google-token"})

		assert.Nil(t, resp)
		assert.ErrorIs(t, err, domain.ErrRegistrationPaused)
		accountRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	t.Run("google login of an EXISTING user still works while paused", func(t *testing.T) {
		originalVerify := verifyGoogleTokenFunc
		t.Cleanup(func() { verifyGoogleTokenFunc = originalVerify })
		verifyGoogleTokenFunc = func(string) (*GoogleTokenInfo, error) {
			return &GoogleTokenInfo{
				Sub:           "google-123",
				Aud:           "my-client-id",
				Email:         "known@gmail.com",
				EmailVerified: "true",
				Name:          "Known User",
			}, nil
		}

		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		role := "admin"
		existing := &domain.User{
			ID:        "01HZZZZZZZZZZZZZZZZZZZZZZZ",
			AccountID: "01HAAAAAAAAAAAAAAAAAAAAAAA",
			Email:     "known@gmail.com",
			Name:      "Known User",
			Role:      &role,
			IsActive:  true,
		}
		userRepo.On("GetByGoogleID", ctx, "google-123").Return(existing, nil)
		userRepo.On("UpdatePicture", ctx, existing.ID, mock.Anything).Return(nil).Maybe()
		jwtService.On("GenerateToken", mock.AnythingOfType("*domain.User")).Return("jwt-token", nil)
		// Post-login onboarding lookup for existing users.
		accountRepo.On("GetByID", ctx, existing.AccountID).Return(&domain.Account{
			ID:                  existing.AccountID,
			OnboardingCompleted: true,
		}, nil)

		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "my-client-id")
		uc.SetRegistrationPaused(true)

		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "valid-google-token"})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, resp.IsNew)
	})

	t.Run("meta sign-up of an unknown email is refused", func(t *testing.T) {
		originalVerify := verifyMetaTokenFunc
		t.Cleanup(func() { verifyMetaTokenFunc = originalVerify })
		verifyMetaTokenFunc = func(string) (*MetaTokenInfo, error) {
			return &MetaTokenInfo{ID: "meta-999", Email: "stranger@fb.com", Name: "Stranger"}, nil
		}

		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		userRepo.On("GetByMetaID", ctx, "meta-999").Return(nil, nil)
		userRepo.On("GetByEmail", ctx, "stranger@fb.com").Return(nil, nil)

		uc := NewMetaAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService)
		uc.SetRegistrationPaused(true)

		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "valid-meta-token"})

		assert.Nil(t, resp)
		assert.ErrorIs(t, err, domain.ErrRegistrationPaused)
		accountRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})
}
