package usecases

import (
	"context"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestVerifyEmailUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("should verify email successfully with valid token", func(t *testing.T) {
		userRepo := new(MockUserRepo)
		emailSvc := new(MockEmailService)

		user := &domain.User{ID: "user1", Email: "user@example.com"}
		userRepo.On("GetByVerificationToken", ctx, "valid-token").Return(user, nil)
		userRepo.On("VerifyEmail", ctx, "user1").Return(nil)
		emailSvc.On("SendWelcomeEmail", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		uc := NewVerifyEmailUseCase(userRepo, emailSvc, "http://localhost:8080")
		err := uc.Execute(ctx, "valid-token")

		assert.NoError(t, err)
		userRepo.AssertExpectations(t)
	})

	t.Run("should return ErrInvalidVerificationToken for invalid token", func(t *testing.T) {
		userRepo := new(MockUserRepo)

		userRepo.On("GetByVerificationToken", ctx, "bad-token").Return(nil, nil)

		uc := NewVerifyEmailUseCase(userRepo, new(MockEmailService), "http://localhost:8080")
		err := uc.Execute(ctx, "bad-token")

		assert.Equal(t, domain.ErrInvalidVerificationToken, err)
	})
}
