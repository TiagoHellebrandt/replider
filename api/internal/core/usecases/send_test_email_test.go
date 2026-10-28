package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/adapters/email"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestSendSMTPTest_Success(t *testing.T) {
	ctx := context.Background()
	adminRole := "admin"
	user := &domain.User{ID: "u1", AccountID: "a1", Email: "admin@example.com", Name: "Admin", Role: &adminRole}

	userRepo := new(MockUserRepo)
	userRepo.On("GetByID", ctx, "u1").Return(user, nil)

	mail := new(MockEmailService)
	mail.On("SendTestEmail", "admin@example.com", "Admin").Return(nil)

	uc := NewSendSMTPTestUseCase(userRepo, mail)
	err := uc.Execute(ctx, "a1", "u1")

	assert.NoError(t, err)
	mail.AssertExpectations(t)
}

func TestSendSMTPTest_RejectsNonAdmin(t *testing.T) {
	ctx := context.Background()
	attendant := "attendant"
	user := &domain.User{ID: "u1", AccountID: "a1", Email: "att@example.com", Role: &attendant}

	userRepo := new(MockUserRepo)
	userRepo.On("GetByID", ctx, "u1").Return(user, nil)

	mail := new(MockEmailService)

	uc := NewSendSMTPTestUseCase(userRepo, mail)
	err := uc.Execute(ctx, "a1", "u1")

	assert.Equal(t, domain.ErrForbidden, err)
	mail.AssertNotCalled(t, "SendTestEmail")
}

func TestSendSMTPTest_WrapsNotConfigured(t *testing.T) {
	ctx := context.Background()
	adminRole := "admin"
	user := &domain.User{ID: "u1", AccountID: "a1", Email: "admin@example.com", Name: "Admin", Role: &adminRole}

	userRepo := new(MockUserRepo)
	userRepo.On("GetByID", ctx, "u1").Return(user, nil)

	mail := new(MockEmailService)
	mail.On("SendTestEmail", "admin@example.com", "Admin").Return(email.ErrSMTPNotConfigured)

	uc := NewSendSMTPTestUseCase(userRepo, mail)
	err := uc.Execute(ctx, "a1", "u1")

	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "SMTP_NOT_CONFIGURED", appErr.Code)
}

func TestSendSMTPTest_WrapsTransportError(t *testing.T) {
	ctx := context.Background()
	adminRole := "admin"
	user := &domain.User{ID: "u1", AccountID: "a1", Email: "admin@example.com", Name: "Admin", Role: &adminRole}

	userRepo := new(MockUserRepo)
	userRepo.On("GetByID", ctx, "u1").Return(user, nil)

	mail := new(MockEmailService)
	mail.On("SendTestEmail", "admin@example.com", "Admin").Return(errors.New("connection refused"))

	uc := NewSendSMTPTestUseCase(userRepo, mail)
	err := uc.Execute(ctx, "a1", "u1")

	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok)
	assert.Equal(t, "SMTP_SEND_FAILED", appErr.Code)
}
