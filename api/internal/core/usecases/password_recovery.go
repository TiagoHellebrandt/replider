package usecases

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/replider/api/internal/adapters/email"
	"github.com/replider/api/internal/core/domain"
	"golang.org/x/crypto/bcrypt"
)

// --- Forgot Password ---

type ForgotPasswordUseCase struct {
	userRepo     domain.UserRepository
	emailService email.EmailService
	appBaseURL   string
	accountRepo  domain.AccountRepository
}

// SetAccountRepo wires the account lookup used to localize the reset
// email (US-018 review fixes). Optional — nil keeps pt-BR copy.
func (uc *ForgotPasswordUseCase) SetAccountRepo(repo domain.AccountRepository) {
	uc.accountRepo = repo
}

func NewForgotPasswordUseCase(userRepo domain.UserRepository, emailService email.EmailService, appBaseURL string) *ForgotPasswordUseCase {
	return &ForgotPasswordUseCase{
		userRepo:     userRepo,
		emailService: emailService,
		appBaseURL:   appBaseURL,
	}
}

type ForgotPasswordDTO struct {
	Email string `json:"email" binding:"required,email"`
}

func (uc *ForgotPasswordUseCase) Execute(ctx context.Context, dto ForgotPasswordDTO) error {
	user, err := uc.userRepo.GetByEmail(ctx, dto.Email)
	if err != nil || user == nil {
		// Return nil even if user not found — prevents email enumeration
		return nil
	}

	// Generate secure random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return err
	}
	token := hex.EncodeToString(tokenBytes)

	// Token expires in 1 hour
	expiresAt := time.Now().Add(1 * time.Hour)
	if err := uc.userRepo.SetResetToken(ctx, user.ID, token, expiresAt); err != nil {
		return err
	}

	locale := emailLocaleFor(ctx, uc.accountRepo, user.AccountID)
	return uc.emailService.SendPasswordResetEmail(user.Email, token, uc.appBaseURL, locale)
}

// --- Reset Password ---

type ResetPasswordUseCase struct {
	userRepo domain.UserRepository
}

func NewResetPasswordUseCase(userRepo domain.UserRepository) *ResetPasswordUseCase {
	return &ResetPasswordUseCase{userRepo: userRepo}
}

type ResetPasswordDTO struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

func (uc *ResetPasswordUseCase) Execute(ctx context.Context, dto ResetPasswordDTO) error {
	user, err := uc.userRepo.GetByResetToken(ctx, dto.Token)
	if err != nil || user == nil {
		return domain.ErrInvalidResetToken
	}

	// Hash the new password
	hashed, err := bcrypt.GenerateFromPassword([]byte(dto.NewPassword), 12)
	if err != nil {
		return err
	}

	if err := uc.userRepo.UpdatePasswordHash(ctx, user.ID, string(hashed)); err != nil {
		return err
	}

	// Invalidate the token
	return uc.userRepo.ClearResetToken(ctx, user.ID)
}
