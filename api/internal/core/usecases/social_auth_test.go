package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Google Auth Tests ---

func TestGoogleAuthUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	// Save original and restore after each test
	originalVerify := verifyGoogleTokenFunc
	t.Cleanup(func() { verifyGoogleTokenFunc = originalVerify })

	t.Run("should create new account and user for new Google user", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyGoogleTokenFunc = func(idToken string) (*GoogleTokenInfo, error) {
			return &GoogleTokenInfo{
				Sub:           "google-123",
				Aud:           "my-client-id",
				Email:         "newuser@gmail.com",
				EmailVerified: "true",
				Name:          "New User",
				Picture:       "https://photo.url",
			}, nil
		}

		// User not found by Google ID
		userRepo.On("GetByGoogleID", ctx, "google-123").Return(nil, nil)
		// User not found by email either
		userRepo.On("GetByEmail", ctx, "newuser@gmail.com").Return(nil, nil)
		// Transaction creates account, user, usage
		accountRepo.On("Create", ctx, mock.MatchedBy(func(a *domain.Account) bool {
			return a.Name == "New User" && a.SubscriptionStatus == "trialing" && len(a.ID) == 26
		})).Return(nil)
		userRepo.On("Create", ctx, mock.MatchedBy(func(u *domain.User) bool {
			return u.Email == "newuser@gmail.com" &&
				u.Name == "New User" &&
				*u.Role == "owner" &&
				*u.GoogleID == "google-123" &&
				u.Picture != nil && *u.Picture == "https://photo.url" &&
				u.IsActive &&
				u.IsEmailVerified &&
				len(u.ID) == 26
		})).Return(nil)
		usageRepo.On("Create", ctx, mock.MatchedBy(func(usage *domain.AccountUsage) bool {
			return len(usage.ID) == 26 && usage.AccountID != "" && usage.BillingCycleStart.Day() == 1
		})).Return(nil)
		jwtService.On("GenerateToken", mock.AnythingOfType("*domain.User")).Return("jwt-token-new", nil)

		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "my-client-id")
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "valid-google-token"})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "jwt-token-new", resp.Token)
		assert.True(t, resp.IsNew)
		assert.Equal(t, "newuser@gmail.com", resp.User.Email)
		accountRepo.AssertExpectations(t)
		userRepo.AssertExpectations(t)
		usageRepo.AssertExpectations(t)
		jwtService.AssertExpectations(t)
	})

	t.Run("should return token for existing user found by Google ID", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyGoogleTokenFunc = func(idToken string) (*GoogleTokenInfo, error) {
			return &GoogleTokenInfo{
				Sub:           "google-456",
				Aud:           "my-client-id",
				Email:         "existing@gmail.com",
				EmailVerified: "true",
				Name:          "Existing User",
			}, nil
		}

		existingUser := &domain.User{
			ID:        "user-existing",
			AccountID: "acc-existing",
			Email:     "existing@gmail.com",
		}
		userRepo.On("GetByGoogleID", ctx, "google-456").Return(existingUser, nil)
		jwtService.On("GenerateToken", existingUser).Return("jwt-token-existing", nil)
		// Existing user's account has completed onboarding
		accountRepo.On("GetByID", ctx, "acc-existing").Return(&domain.Account{
			ID: "acc-existing", OnboardingCompleted: true,
		}, nil)

		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "my-client-id")
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "valid-google-token"})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "jwt-token-existing", resp.Token)
		assert.False(t, resp.IsNew)
		assert.Equal(t, existingUser, resp.User)
		userRepo.AssertExpectations(t)
		jwtService.AssertExpectations(t)
	})

	t.Run("should link Google ID to existing user found by email", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyGoogleTokenFunc = func(idToken string) (*GoogleTokenInfo, error) {
			return &GoogleTokenInfo{
				Sub:           "google-789",
				Aud:           "my-client-id",
				Email:         "link@gmail.com",
				EmailVerified: "true",
				Name:          "Link User",
			}, nil
		}

		existingUser := &domain.User{
			ID:        "user-link",
			AccountID: "acc-link",
			Email:     "link@gmail.com",
		}
		// Not found by Google ID
		userRepo.On("GetByGoogleID", ctx, "google-789").Return(nil, nil)
		// Found by email
		userRepo.On("GetByEmail", ctx, "link@gmail.com").Return(existingUser, nil)
		// Link Google ID
		userRepo.On("SetGoogleID", ctx, "user-link", "google-789").Return(nil)
		// No SetPicture: Google returned no Picture, so we must NOT call it.
		jwtService.On("GenerateToken", existingUser).Return("jwt-token-linked", nil)
		accountRepo.On("GetByID", ctx, "acc-link").Return(&domain.Account{
			ID: "acc-link", OnboardingCompleted: true,
		}, nil)

		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "my-client-id")
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "valid-google-token"})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "jwt-token-linked", resp.Token)
		assert.False(t, resp.IsNew)
		userRepo.AssertExpectations(t)
		// Defensive: SetPicture must NOT have been called for this user
		userRepo.AssertNotCalled(t, "SetPicture", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should also persist picture when linking Google to existing user", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyGoogleTokenFunc = func(idToken string) (*GoogleTokenInfo, error) {
			return &GoogleTokenInfo{
				Sub:           "google-pic-link",
				Aud:           "my-client-id",
				Email:         "withpic@gmail.com",
				EmailVerified: "true",
				Name:          "With Picture",
				Picture:       "https://photo.example/avatar.jpg",
			}, nil
		}

		existingUser := &domain.User{
			ID:        "user-withpic",
			AccountID: "acc-withpic",
			Email:     "withpic@gmail.com",
		}
		userRepo.On("GetByGoogleID", ctx, "google-pic-link").Return(nil, nil)
		userRepo.On("GetByEmail", ctx, "withpic@gmail.com").Return(existingUser, nil)
		userRepo.On("SetGoogleID", ctx, "user-withpic", "google-pic-link").Return(nil)
		userRepo.On("SetPicture", ctx, "user-withpic", "https://photo.example/avatar.jpg").Return(nil)
		jwtService.On("GenerateToken", existingUser).Return("jwt-token-withpic", nil)
		accountRepo.On("GetByID", ctx, "acc-withpic").Return(&domain.Account{
			ID: "acc-withpic", OnboardingCompleted: true,
		}, nil)

		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "my-client-id")
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "valid-google-token"})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		// In-memory user is also updated so the response sent to the client
		// already includes the new picture without a roundtrip.
		assert.NotNil(t, resp.User.Picture)
		assert.Equal(t, "https://photo.example/avatar.jpg", *resp.User.Picture)
		userRepo.AssertExpectations(t)
	})

	t.Run("should fail when SetPicture errors during link", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyGoogleTokenFunc = func(idToken string) (*GoogleTokenInfo, error) {
			return &GoogleTokenInfo{
				Sub:           "google-pic-err",
				Aud:           "my-client-id",
				Email:         "picerr@gmail.com",
				EmailVerified: "true",
				Name:          "Pic Error",
				Picture:       "https://photo.example/p.jpg",
			}, nil
		}

		existingUser := &domain.User{ID: "user-picerr", Email: "picerr@gmail.com"}
		userRepo.On("GetByGoogleID", ctx, "google-pic-err").Return(nil, nil)
		userRepo.On("GetByEmail", ctx, "picerr@gmail.com").Return(existingUser, nil)
		userRepo.On("SetGoogleID", ctx, "user-picerr", "google-pic-err").Return(nil)
		userRepo.On("SetPicture", ctx, "user-picerr", "https://photo.example/p.jpg").
			Return(errors.New("db unavailable"))

		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "my-client-id")
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "token"})

		assert.Nil(t, resp)
		assert.EqualError(t, err, "db unavailable")
	})

	t.Run("should return ErrSocialAuthFailed for invalid Google token", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyGoogleTokenFunc = func(idToken string) (*GoogleTokenInfo, error) {
			return nil, errors.New("invalid token")
		}

		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "my-client-id")
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "bad-token"})

		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrSocialAuthFailed, err)
	})

	t.Run("should return ErrSocialAuthFailed when audience does not match", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyGoogleTokenFunc = func(idToken string) (*GoogleTokenInfo, error) {
			return &GoogleTokenInfo{
				Sub:           "google-aud",
				Aud:           "wrong-client-id",
				Email:         "user@gmail.com",
				EmailVerified: "true",
				Name:          "User",
			}, nil
		}

		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "my-client-id")
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "valid-but-wrong-aud"})

		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrSocialAuthFailed, err)
	})

	t.Run("should return ErrSocialAuthFailed when email is not verified", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyGoogleTokenFunc = func(idToken string) (*GoogleTokenInfo, error) {
			return &GoogleTokenInfo{
				Sub:           "google-unverified",
				Aud:           "my-client-id",
				Email:         "unverified@gmail.com",
				EmailVerified: "false",
				Name:          "Unverified",
			}, nil
		}

		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "my-client-id")
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "unverified-email-token"})

		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrSocialAuthFailed, err)
	})

	t.Run("refuses login when googleClientID is empty (fail-closed)", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyGoogleTokenFunc = func(idToken string) (*GoogleTokenInfo, error) {
			return &GoogleTokenInfo{
				Sub:           "google-noaud",
				Aud:           "any-audience",
				Email:         "noaud@gmail.com",
				EmailVerified: "true",
				Name:          "No Aud Check",
			}, nil
		}

		existingUser := &domain.User{ID: "user-noaud", AccountID: "acc-noaud", Email: "noaud@gmail.com"}
		userRepo.On("GetByGoogleID", ctx, "google-noaud").Return(existingUser, nil)
		jwtService.On("GenerateToken", existingUser).Return("jwt-noaud", nil)
		accountRepo.On("GetByID", ctx, "acc-noaud").Return(&domain.Account{
			ID: "acc-noaud", OnboardingCompleted: true,
		}, nil)

		// Empty googleClientID is a misconfiguration — fail closed rather than
		// accepting an id_token minted for any other OAuth client.
		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "")
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "token"})

		assert.ErrorIs(t, err, domain.ErrSocialAuthFailed)
		assert.Nil(t, resp)
	})

	t.Run("should return error when GetByGoogleID fails", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyGoogleTokenFunc = func(idToken string) (*GoogleTokenInfo, error) {
			return &GoogleTokenInfo{
				Sub:           "google-dberr",
				Aud:           "my-client-id",
				Email:         "dberr@gmail.com",
				EmailVerified: "true",
				Name:          "DB Error",
			}, nil
		}

		userRepo.On("GetByGoogleID", ctx, "google-dberr").Return(nil, errors.New("db connection error"))

		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "my-client-id")
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "token"})

		assert.Nil(t, resp)
		assert.EqualError(t, err, "db connection error")
	})

	t.Run("should return error when account creation fails in transaction", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyGoogleTokenFunc = func(idToken string) (*GoogleTokenInfo, error) {
			return &GoogleTokenInfo{
				Sub:           "google-txerr",
				Aud:           "my-client-id",
				Email:         "txerr@gmail.com",
				EmailVerified: "true",
				Name:          "Tx Error",
			}, nil
		}

		userRepo.On("GetByGoogleID", ctx, "google-txerr").Return(nil, nil)
		userRepo.On("GetByEmail", ctx, "txerr@gmail.com").Return(nil, nil)
		accountRepo.On("Create", ctx, mock.Anything).Return(errors.New("account create failed"))

		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "my-client-id")
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "token"})

		assert.Nil(t, resp)
		assert.EqualError(t, err, "account create failed")
	})

	t.Run("should return error when JWT generation fails", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyGoogleTokenFunc = func(idToken string) (*GoogleTokenInfo, error) {
			return &GoogleTokenInfo{
				Sub:           "google-jwterr",
				Aud:           "my-client-id",
				Email:         "jwterr@gmail.com",
				EmailVerified: "true",
				Name:          "JWT Error",
			}, nil
		}

		existingUser := &domain.User{ID: "user-jwterr", Email: "jwterr@gmail.com"}
		userRepo.On("GetByGoogleID", ctx, "google-jwterr").Return(existingUser, nil)
		jwtService.On("GenerateToken", existingUser).Return("", errors.New("jwt signing failed"))

		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "my-client-id")
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "token"})

		assert.Nil(t, resp)
		assert.EqualError(t, err, "jwt signing failed")
	})

	t.Run("should return error when SetGoogleID fails for email-linked user", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyGoogleTokenFunc = func(idToken string) (*GoogleTokenInfo, error) {
			return &GoogleTokenInfo{
				Sub:           "google-setid-err",
				Aud:           "my-client-id",
				Email:         "setid@gmail.com",
				EmailVerified: "true",
				Name:          "SetID Error",
			}, nil
		}

		existingUser := &domain.User{ID: "user-setid", Email: "setid@gmail.com"}
		userRepo.On("GetByGoogleID", ctx, "google-setid-err").Return(nil, nil)
		userRepo.On("GetByEmail", ctx, "setid@gmail.com").Return(existingUser, nil)
		userRepo.On("SetGoogleID", ctx, "user-setid", "google-setid-err").Return(errors.New("set google id failed"))

		uc := NewGoogleAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService, "my-client-id")
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "token"})

		assert.Nil(t, resp)
		assert.EqualError(t, err, "set google id failed")
	})
}

// --- Meta Auth Tests ---

func TestMetaAuthUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	originalVerify := verifyMetaTokenFunc
	t.Cleanup(func() { verifyMetaTokenFunc = originalVerify })

	t.Run("should create new account and user for new Meta user", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyMetaTokenFunc = func(accessToken string) (*MetaTokenInfo, error) {
			return &MetaTokenInfo{
				ID:    "meta-123",
				Name:  "Meta User",
				Email: "metauser@facebook.com",
			}, nil
		}

		userRepo.On("GetByMetaID", ctx, "meta-123").Return(nil, nil)
		userRepo.On("GetByEmail", ctx, "metauser@facebook.com").Return(nil, nil)
		accountRepo.On("Create", ctx, mock.MatchedBy(func(a *domain.Account) bool {
			return a.Name == "Meta User" && a.SubscriptionStatus == "trialing" && len(a.ID) == 26
		})).Return(nil)
		userRepo.On("Create", ctx, mock.MatchedBy(func(u *domain.User) bool {
			return u.Email == "metauser@facebook.com" &&
				u.Name == "Meta User" &&
				*u.Role == "owner" &&
				*u.MetaID == "meta-123" &&
				u.IsActive &&
				u.IsEmailVerified &&
				len(u.ID) == 26
		})).Return(nil)
		usageRepo.On("Create", ctx, mock.MatchedBy(func(usage *domain.AccountUsage) bool {
			return len(usage.ID) == 26 && usage.AccountID != "" && usage.BillingCycleStart.Day() == 1
		})).Return(nil)
		jwtService.On("GenerateToken", mock.AnythingOfType("*domain.User")).Return("jwt-meta-new", nil)

		uc := NewMetaAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService)
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "valid-meta-token"})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "jwt-meta-new", resp.Token)
		assert.True(t, resp.IsNew)
		assert.Equal(t, "metauser@facebook.com", resp.User.Email)
		accountRepo.AssertExpectations(t)
		userRepo.AssertExpectations(t)
		usageRepo.AssertExpectations(t)
		jwtService.AssertExpectations(t)
	})

	t.Run("should return token for existing user found by Meta ID", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyMetaTokenFunc = func(accessToken string) (*MetaTokenInfo, error) {
			return &MetaTokenInfo{
				ID:    "meta-456",
				Name:  "Existing Meta",
				Email: "existing@facebook.com",
			}, nil
		}

		existingUser := &domain.User{
			ID:        "user-meta-existing",
			AccountID: "acc-meta-existing",
			Email:     "existing@facebook.com",
		}
		userRepo.On("GetByMetaID", ctx, "meta-456").Return(existingUser, nil)
		jwtService.On("GenerateToken", existingUser).Return("jwt-meta-existing", nil)
		accountRepo.On("GetByID", ctx, "acc-meta-existing").Return(&domain.Account{
			ID: "acc-meta-existing", OnboardingCompleted: true,
		}, nil)

		uc := NewMetaAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService)
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "valid-meta-token"})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "jwt-meta-existing", resp.Token)
		assert.False(t, resp.IsNew)
		assert.Equal(t, existingUser, resp.User)
		userRepo.AssertExpectations(t)
		jwtService.AssertExpectations(t)
	})

	t.Run("should link Meta ID to existing user found by email", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyMetaTokenFunc = func(accessToken string) (*MetaTokenInfo, error) {
			return &MetaTokenInfo{
				ID:    "meta-789",
				Name:  "Link Meta",
				Email: "link@facebook.com",
			}, nil
		}

		existingUser := &domain.User{
			ID:        "user-meta-link",
			AccountID: "acc-meta-link",
			Email:     "link@facebook.com",
		}
		userRepo.On("GetByMetaID", ctx, "meta-789").Return(nil, nil)
		userRepo.On("GetByEmail", ctx, "link@facebook.com").Return(existingUser, nil)
		userRepo.On("SetMetaID", ctx, "user-meta-link", "meta-789").Return(nil)
		jwtService.On("GenerateToken", existingUser).Return("jwt-meta-linked", nil)
		accountRepo.On("GetByID", ctx, "acc-meta-link").Return(&domain.Account{
			ID: "acc-meta-link", OnboardingCompleted: true,
		}, nil)

		uc := NewMetaAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService)
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "valid-meta-token"})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "jwt-meta-linked", resp.Token)
		assert.False(t, resp.IsNew)
		userRepo.AssertExpectations(t)
	})

	t.Run("should return ErrSocialAuthFailed for invalid Meta token", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyMetaTokenFunc = func(accessToken string) (*MetaTokenInfo, error) {
			return nil, errors.New("invalid meta token")
		}

		uc := NewMetaAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService)
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "bad-meta-token"})

		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrSocialAuthFailed, err)
	})

	t.Run("should create new user when Meta has no email and no existing user", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyMetaTokenFunc = func(accessToken string) (*MetaTokenInfo, error) {
			return &MetaTokenInfo{
				ID:    "meta-noemail",
				Name:  "No Email User",
				Email: "", // Meta user without email permission
			}, nil
		}

		userRepo.On("GetByMetaID", ctx, "meta-noemail").Return(nil, nil)
		// Email is empty, so GetByEmail should NOT be called
		accountRepo.On("Create", ctx, mock.MatchedBy(func(a *domain.Account) bool {
			return a.Name == "No Email User" && len(a.ID) == 26
		})).Return(nil)
		userRepo.On("Create", ctx, mock.MatchedBy(func(u *domain.User) bool {
			return u.Email == "" && u.Name == "No Email User" && *u.MetaID == "meta-noemail"
		})).Return(nil)
		usageRepo.On("Create", ctx, mock.Anything).Return(nil)
		jwtService.On("GenerateToken", mock.AnythingOfType("*domain.User")).Return("jwt-noemail", nil)

		uc := NewMetaAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService)
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "token"})

		assert.NoError(t, err)
		assert.True(t, resp.IsNew)
		// GetByEmail should not have been called since email was empty
		userRepo.AssertNotCalled(t, "GetByEmail", mock.Anything, mock.Anything)
	})

	t.Run("should return error when GetByMetaID fails", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyMetaTokenFunc = func(accessToken string) (*MetaTokenInfo, error) {
			return &MetaTokenInfo{
				ID:    "meta-dberr",
				Name:  "DB Error",
				Email: "dberr@facebook.com",
			}, nil
		}

		userRepo.On("GetByMetaID", ctx, "meta-dberr").Return(nil, errors.New("db connection error"))

		uc := NewMetaAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService)
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "token"})

		assert.Nil(t, resp)
		assert.EqualError(t, err, "db connection error")
	})

	t.Run("should return error when account creation fails in transaction", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyMetaTokenFunc = func(accessToken string) (*MetaTokenInfo, error) {
			return &MetaTokenInfo{
				ID:    "meta-txerr",
				Name:  "Tx Error",
				Email: "txerr@facebook.com",
			}, nil
		}

		userRepo.On("GetByMetaID", ctx, "meta-txerr").Return(nil, nil)
		userRepo.On("GetByEmail", ctx, "txerr@facebook.com").Return(nil, nil)
		accountRepo.On("Create", ctx, mock.Anything).Return(errors.New("account create failed"))

		uc := NewMetaAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService)
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "token"})

		assert.Nil(t, resp)
		assert.EqualError(t, err, "account create failed")
	})

	t.Run("should return error when JWT generation fails", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyMetaTokenFunc = func(accessToken string) (*MetaTokenInfo, error) {
			return &MetaTokenInfo{
				ID:    "meta-jwterr",
				Name:  "JWT Error",
				Email: "jwterr@facebook.com",
			}, nil
		}

		existingUser := &domain.User{ID: "user-meta-jwterr", Email: "jwterr@facebook.com"}
		userRepo.On("GetByMetaID", ctx, "meta-jwterr").Return(existingUser, nil)
		jwtService.On("GenerateToken", existingUser).Return("", errors.New("jwt signing failed"))

		uc := NewMetaAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService)
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "token"})

		assert.Nil(t, resp)
		assert.EqualError(t, err, "jwt signing failed")
	})

	t.Run("should return error when SetMetaID fails for email-linked user", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		usageRepo := new(MockUsageRepo)
		txManager := new(MockTxManager)
		jwtService := new(MockJWTService)

		verifyMetaTokenFunc = func(accessToken string) (*MetaTokenInfo, error) {
			return &MetaTokenInfo{
				ID:    "meta-setid-err",
				Name:  "SetID Error",
				Email: "setid@facebook.com",
			}, nil
		}

		existingUser := &domain.User{ID: "user-meta-setid", Email: "setid@facebook.com"}
		userRepo.On("GetByMetaID", ctx, "meta-setid-err").Return(nil, nil)
		userRepo.On("GetByEmail", ctx, "setid@facebook.com").Return(existingUser, nil)
		userRepo.On("SetMetaID", ctx, "user-meta-setid", "meta-setid-err").Return(errors.New("set meta id failed"))

		uc := NewMetaAuthUseCase(userRepo, accountRepo, usageRepo, txManager, jwtService)
		resp, err := uc.Execute(ctx, SocialAuthDTO{IDToken: "token"})

		assert.Nil(t, resp)
		assert.EqualError(t, err, "set meta id failed")
	})
}
