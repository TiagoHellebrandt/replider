package usecases

import (
	"context"
	"log/slog"

	"github.com/replider/api/internal/adapters/email"
	"github.com/replider/api/internal/core/domain"
)

type VerifyEmailUseCase struct {
	userRepo     domain.UserRepository
	emailService email.EmailService
	appBaseURL   string
	accountRepo  domain.AccountRepository
}

// SetAccountRepo wires the account lookup used to localize the welcome
// email (US-018 review fixes). Optional — nil keeps pt-BR copy.
func (uc *VerifyEmailUseCase) SetAccountRepo(repo domain.AccountRepository) {
	uc.accountRepo = repo
}

func NewVerifyEmailUseCase(userRepo domain.UserRepository, emailService email.EmailService, appBaseURL string) *VerifyEmailUseCase {
	return &VerifyEmailUseCase{
		userRepo:     userRepo,
		emailService: emailService,
		appBaseURL:   appBaseURL,
	}
}

func (uc *VerifyEmailUseCase) Execute(ctx context.Context, token string) error {
	user, err := uc.userRepo.GetByVerificationToken(ctx, token)
	if err != nil {
		return err
	}
	if user == nil {
		return domain.ErrInvalidVerificationToken
	}
	if err := uc.userRepo.VerifyEmail(ctx, user.ID); err != nil {
		return err
	}

	// Send welcome email asynchronously. EmailService methods bound
	// their own SMTP socket with an internal 8s timeout, so this
	// goroutine has a hard ceiling without needing a context here.
	locale := emailLocaleFor(ctx, uc.accountRepo, user.AccountID)
	go func() {
		if err := uc.emailService.SendWelcomeEmail(user.Email, user.Name, uc.appBaseURL, locale); err != nil {
			slog.Error("failed to send welcome email", "email", user.Email, "error", err)
		}
	}()

	return nil
}
