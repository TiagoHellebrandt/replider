package usecases

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/replider/api/internal/adapters/email"
	"github.com/replider/api/internal/core/domain"
)

// ResendVerificationUseCase re-issues a verification token and mails a
// fresh link. Exists because the original SendVerificationEmail call
// on signup can fail (SMTP hiccup, wrong address typed by the user)
// and today the only recovery is to create a new account — bad UX.
//
// Security posture:
//   - We don't reveal whether the email exists. A non-existent or
//     already-verified address returns the SAME "check your inbox"
//     response as a successful re-send. No enumeration leak.
//   - The token lifespan is refreshed to 24h from now and the old one
//     is invalidated (same SetVerificationToken write).
//   - Rate-limiting is done at the HTTP layer (per IP + per account id
//     via middleware) — not in this use case.
type ResendVerificationUseCase struct {
	userRepo     domain.UserRepository
	emailService email.EmailService
	appBaseURL   string
	accountRepo  domain.AccountRepository
}

// SetAccountRepo wires the account lookup used to localize the email
// (US-018 review fixes). Optional — nil keeps pt-BR copy.
func (uc *ResendVerificationUseCase) SetAccountRepo(repo domain.AccountRepository) {
	uc.accountRepo = repo
}

func NewResendVerificationUseCase(userRepo domain.UserRepository, emailService email.EmailService, appBaseURL string) *ResendVerificationUseCase {
	return &ResendVerificationUseCase{
		userRepo:     userRepo,
		emailService: emailService,
		appBaseURL:   appBaseURL,
	}
}

// Execute always returns nil to the caller when the email is missing or
// already verified, so the handler can respond 200 without leaking
// account existence. Real errors (storage write, SMTP) surface normally.
func (uc *ResendVerificationUseCase) Execute(ctx context.Context, emailAddr string) error {
	user, err := uc.userRepo.GetByEmail(ctx, emailAddr)
	if err != nil {
		return err
	}
	// Silent no-op for enumeration resistance.
	if user == nil || user.IsEmailVerified {
		return nil
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return err
	}
	token := hex.EncodeToString(tokenBytes)
	expiresAt := time.Now().Add(24 * time.Hour)
	if err := uc.userRepo.SetVerificationToken(ctx, user.ID, token, expiresAt); err != nil {
		return err
	}

	// Non-blocking: SMTP latency shouldn't hold the HTTP ack.
	locale := emailLocaleFor(ctx, uc.accountRepo, user.AccountID)
	go func() {
		if err := uc.emailService.SendVerificationEmail(user.Email, token, uc.appBaseURL, locale); err != nil {
			slog.Error("resend verification email failed", "email", user.Email, "error", err)
		}
	}()
	return nil
}
