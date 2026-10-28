package usecases

import (
	"context"
	"errors"

	"github.com/replider/api/internal/adapters/email"
	"github.com/replider/api/internal/core/domain"
)

// SendSMTPTestUseCase sends a canary email to the requesting admin so they
// can verify that the SMTP config is healthy end-to-end. Useful after
// rotating SMTP credentials or on a fresh deploy.
type SendSMTPTestUseCase struct {
	userRepo domain.UserRepository
	email    email.EmailService
}

func NewSendSMTPTestUseCase(userRepo domain.UserRepository, emailSvc email.EmailService) *SendSMTPTestUseCase {
	return &SendSMTPTestUseCase{userRepo: userRepo, email: emailSvc}
}

// Execute validates the caller is an admin of accountID, then fires a
// test email to their own address. Returns:
//   - domain.ErrUnauthorized when the caller is not an admin or the user
//     doesn't belong to accountID
//   - AppError{Code: SMTP_NOT_CONFIGURED} when the server has no SMTP host
//     configured (the stub logger fallback is disabled for this path)
//   - AppError{Code: SMTP_SEND_FAILED} on transport errors
func (uc *SendSMTPTestUseCase) Execute(ctx context.Context, accountID, userID string) error {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil || user.AccountID != accountID {
		return domain.ErrForbidden
	}
	// RoleSatisfies and 403, for the usual two reasons: an owner is not literally
	// "admin", and a 401 makes the app conclude the session died.
	if !domain.RoleSatisfies(roleOf(user), domain.RoleAdminRole) {
		return domain.ErrForbidden
	}

	name := user.Name
	if name == "" {
		name = "Admin"
	}

	if err := uc.email.SendTestEmail(user.Email, name); err != nil {
		if errors.Is(err, email.ErrSMTPNotConfigured) {
			return &domain.AppError{Code: "SMTP_NOT_CONFIGURED", Message: "SMTP is not configured on the server"}
		}
		return &domain.AppError{Code: "SMTP_SEND_FAILED", Message: err.Error()}
	}
	return nil
}
