package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestForgotPasswordUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should send reset email when user exists", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		emailSvc := new(MockEmailService)

		user := &domain.User{ID: "user1", Email: "user@example.com"}
		userRepo.On("GetByEmail", ctx, "user@example.com").Return(user, nil)
		userRepo.On("SetResetToken", ctx, "user1", mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)
		emailSvc.On("SendPasswordResetEmail", "user@example.com", mock.AnythingOfType("string"), "http://localhost:8080", mock.AnythingOfType("string")).Return(nil)

		uc := NewForgotPasswordUseCase(userRepo, emailSvc, "http://localhost:8080")
		err := uc.Execute(ctx, ForgotPasswordDTO{Email: "user@example.com"})

		assert.NoError(t, err)
		userRepo.AssertExpectations(t)
		emailSvc.AssertExpectations(t)
	})

	t.Run("should return nil when user not found to prevent email enumeration", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		emailSvc := new(MockEmailService)

		userRepo.On("GetByEmail", ctx, "unknown@example.com").Return(nil, nil)

		uc := NewForgotPasswordUseCase(userRepo, emailSvc, "http://localhost:8080")
		err := uc.Execute(ctx, ForgotPasswordDTO{Email: "unknown@example.com"})

		assert.NoError(t, err)
		// Email service should NOT be called
		emailSvc.AssertNotCalled(t, "SendPasswordResetEmail")
	})
}

func TestResetPasswordUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should reset password successfully with valid token", func(t *testing.T) {
		userRepo := new(MockUserRepo)

		user := &domain.User{
			ID:                  "user1",
			ResetToken:          strPtr("valid-token"),
			ResetTokenExpiresAt: timePtr(time.Now().Add(1 * time.Hour)),
		}
		userRepo.On("GetByResetToken", ctx, "valid-token").Return(user, nil)
		userRepo.On("UpdatePasswordHash", ctx, "user1", mock.AnythingOfType("string")).Return(nil)
		userRepo.On("ClearResetToken", ctx, "user1").Return(nil)

		uc := NewResetPasswordUseCase(userRepo)
		err := uc.Execute(ctx, ResetPasswordDTO{
			Token:       "valid-token",
			NewPassword: "new-secure-password",
		})

		assert.NoError(t, err)
		userRepo.AssertExpectations(t)
	})

	t.Run("should return ErrInvalidResetToken for invalid token", func(t *testing.T) {
		userRepo := new(MockUserRepo)

		userRepo.On("GetByResetToken", ctx, "bad-token").Return(nil, nil)

		uc := NewResetPasswordUseCase(userRepo)
		err := uc.Execute(ctx, ResetPasswordDTO{
			Token:       "bad-token",
			NewPassword: "new-password",
		})

		assert.Equal(t, domain.ErrInvalidResetToken, err)
	})
}

func strPtr(s string) *string        { return &s }
func timePtr(t time.Time) *time.Time { return &t }
