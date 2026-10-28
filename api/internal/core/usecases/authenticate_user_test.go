package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthenticateUserUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)
	hashStr := string(hashedPassword)

	t.Run("should login successfully and return is_new=false when agents exist", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		jwtService := new(MockJWTService)

		user := &domain.User{
			ID:              "user1",
			AccountID:       "acc1",
			Email:           "user@example.com",
			PasswordHash:    &hashStr,
			IsEmailVerified: true,
		}
		userRepo.On("GetByEmail", ctx, "user@example.com").Return(user, nil)
		jwtService.On("GenerateToken", user).Return("jwt-token-123", nil)
		accountRepo.On("GetByID", ctx, "acc1").Return(&domain.Account{ID: "acc1", OnboardingCompleted: true}, nil)

		uc := NewAuthenticateUserUseCase(userRepo, accountRepo, jwtService)
		resp, err := uc.Execute(ctx, AuthenticateUserDTO{
			Email:    "user@example.com",
			Password: "correct-password",
		})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "jwt-token-123", resp.Token)
		assert.False(t, resp.IsNew)
		userRepo.AssertExpectations(t)
		jwtService.AssertExpectations(t)
		accountRepo.AssertExpectations(t)
	})

	t.Run("should return is_new=true when account has no agents", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		jwtService := new(MockJWTService)

		admin := "admin"
		user := &domain.User{
			ID:              "user1",
			AccountID:       "acc1",
			Email:           "new@example.com",
			PasswordHash:    &hashStr,
			IsEmailVerified: true,
			Role:            &admin,
		}
		userRepo.On("GetByEmail", ctx, "new@example.com").Return(user, nil)
		jwtService.On("GenerateToken", user).Return("jwt-token-456", nil)
		accountRepo.On("GetByID", ctx, "acc1").Return(&domain.Account{ID: "acc1", OnboardingCompleted: false}, nil)

		uc := NewAuthenticateUserUseCase(userRepo, accountRepo, jwtService)
		resp, err := uc.Execute(ctx, AuthenticateUserDTO{
			Email:    "new@example.com",
			Password: "correct-password",
		})

		assert.NoError(t, err)
		assert.True(t, resp.IsNew)
	})

	// The wizard is the admin's: its first step is PATCH /accounts. A
	// vendedora invited before the admin finished it must NOT be sent
	// there (403 on step one, no Skip — a loop with no exit).
	t.Run("a vendedor is never is_new even when the account has not onboarded", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		jwtService := new(MockJWTService)

		vendedor := "vendedor"
		user := &domain.User{
			ID:              "user2",
			AccountID:       "acc1",
			Email:           "vend@example.com",
			PasswordHash:    &hashStr,
			IsEmailVerified: true,
			Role:            &vendedor,
		}
		userRepo.On("GetByEmail", ctx, "vend@example.com").Return(user, nil)
		jwtService.On("GenerateToken", user).Return("jwt-token-789", nil)
		accountRepo.On("GetByID", ctx, "acc1").Return(&domain.Account{ID: "acc1", OnboardingCompleted: false}, nil).Maybe()

		uc := NewAuthenticateUserUseCase(userRepo, accountRepo, jwtService)
		resp, err := uc.Execute(ctx, AuthenticateUserDTO{
			Email:    "vend@example.com",
			Password: "correct-password",
		})

		assert.NoError(t, err)
		assert.False(t, resp.IsNew)
	})

	t.Run("should return ErrInvalidCredentials for wrong password", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		jwtService := new(MockJWTService)

		user := &domain.User{
			ID:              "user1",
			Email:           "user@example.com",
			PasswordHash:    &hashStr,
			IsEmailVerified: true,
		}
		userRepo.On("GetByEmail", ctx, "user@example.com").Return(user, nil)

		uc := NewAuthenticateUserUseCase(userRepo, accountRepo, jwtService)
		resp, err := uc.Execute(ctx, AuthenticateUserDTO{
			Email:    "user@example.com",
			Password: "wrong-password",
		})

		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrInvalidCredentials, err)
	})

	t.Run("should return ErrInvalidCredentials when user not found", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		jwtService := new(MockJWTService)

		userRepo.On("GetByEmail", ctx, "unknown@example.com").Return(nil, nil)

		uc := NewAuthenticateUserUseCase(userRepo, accountRepo, jwtService)
		resp, err := uc.Execute(ctx, AuthenticateUserDTO{
			Email:    "unknown@example.com",
			Password: "any-password",
		})

		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrInvalidCredentials, err)
	})

	t.Run("should authenticate successfully even when email is not verified", func(t *testing.T) {
		// Email verification is no longer a hard login gate — we don't
		// want to lock users out of their own account when the SMTP
		// bounces or the mail lands in spam. The /me response still
		// exposes is_email_verified so the UI can nudge them.
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		jwtService := new(MockJWTService)

		user := &domain.User{
			ID:              "user1",
			AccountID:       "account1",
			Email:           "user@example.com",
			PasswordHash:    &hashStr,
			IsEmailVerified: false,
		}
		userRepo.On("GetByEmail", ctx, "user@example.com").Return(user, nil)
		accountRepo.On("GetByID", ctx, "account1").Return(&domain.Account{
			ID:                  "account1",
			OnboardingCompleted: true,
		}, nil)
		jwtService.On("GenerateToken", user).Return("token", nil)

		uc := NewAuthenticateUserUseCase(userRepo, accountRepo, jwtService)
		resp, err := uc.Execute(ctx, AuthenticateUserDTO{
			Email:    "user@example.com",
			Password: "correct-password",
		})

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "token", resp.Token)
	})

	t.Run("should return ErrInvalidCredentials for social-only user with no password", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		jwtService := new(MockJWTService)

		user := &domain.User{
			ID:              "user1",
			Email:           "social@example.com",
			PasswordHash:    nil,
			IsEmailVerified: true,
		}
		userRepo.On("GetByEmail", ctx, "social@example.com").Return(user, nil)

		uc := NewAuthenticateUserUseCase(userRepo, accountRepo, jwtService)
		resp, err := uc.Execute(ctx, AuthenticateUserDTO{
			Email:    "social@example.com",
			Password: "any-password",
		})

		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrInvalidCredentials, err)
	})

	t.Run("returns mfa_required when user has TOTP enabled, instead of a session JWT", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		accountRepo := new(MockAccountRepo)
		jwtService := new(MockJWTService)

		user := &domain.User{
			ID:           "user-mfa",
			AccountID:    "acc1",
			Email:        "mfa@example.com",
			PasswordHash: &hashStr,
			TOTPEnabled:  true,
		}
		userRepo.On("GetByEmail", ctx, "mfa@example.com").Return(user, nil)
		jwtService.On("GenerateMfaLoginToken", "user-mfa").Return("mfa-session-token", nil)

		uc := NewAuthenticateUserUseCase(userRepo, accountRepo, jwtService)
		resp, err := uc.Execute(ctx, AuthenticateUserDTO{
			Email:    "mfa@example.com",
			Password: "correct-password",
		})

		assert.NoError(t, err)
		assert.True(t, resp.MfaRequired)
		assert.Equal(t, "mfa-session-token", resp.MfaSession)
		assert.Empty(t, resp.Token, "no full JWT is issued until the verify-login round-trip")
		jwtService.AssertNotCalled(t, "GenerateToken")
		accountRepo.AssertNotCalled(t, "GetByID")
	})
}
