package usecases

import (
	"context"
	"errors"

	"github.com/replider/api/internal/core/domain"
	"golang.org/x/crypto/bcrypt"
)

// ──────────────────────────────── UpdateProfileUseCase ────────────────────────────────

type UpdateProfileDTO struct {
	UserID    string
	AccountID string
	Name      string
	Email     string
	// Signature: nil preserves the current one (older clients don't send
	// the field), "" clears it, anything else sets it. Max 280 chars —
	// it's a sign-off line, not a template.
	Signature *string
}

type UpdateProfileUseCase struct {
	userRepo domain.UserRepository
}

func NewUpdateProfileUseCase(userRepo domain.UserRepository) *UpdateProfileUseCase {
	return &UpdateProfileUseCase{userRepo: userRepo}
}

// Execute updates user name and email. It checks for email uniqueness first.
func (uc *UpdateProfileUseCase) Execute(ctx context.Context, dto UpdateProfileDTO) (*domain.User, error) {
	if dto.Name == "" || dto.Email == "" {
		return nil, &domain.AppError{Code: "VALIDATION_ERROR", Message: "name and email are required"}
	}
	if dto.Signature != nil && len(*dto.Signature) > 280 {
		return nil, &domain.AppError{Code: "VALIDATION_ERROR", Message: "signature must be at most 280 characters"}
	}

	// Check if email is taken by another user
	existing, err := uc.userRepo.GetByEmail(ctx, dto.Email)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.ID != dto.UserID {
		return nil, domain.ErrEmailAlreadyExists
	}

	if err := uc.userRepo.UpdateProfile(ctx, dto.UserID, dto.Name, dto.Email, dto.Signature); err != nil {
		return nil, err
	}

	// Return updated user
	user, err := uc.userRepo.GetByID(ctx, dto.UserID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// ──────────────────────────────── ChangePasswordUseCase ────────────────────────────────

type ChangePasswordDTO struct {
	UserID          string
	CurrentPassword string
	NewPassword     string
}

type ChangePasswordUseCase struct {
	userRepo domain.UserRepository
}

func NewChangePasswordUseCase(userRepo domain.UserRepository) *ChangePasswordUseCase {
	return &ChangePasswordUseCase{userRepo: userRepo}
}

// Execute verifies the current password and updates the hash if correct.
func (uc *ChangePasswordUseCase) Execute(ctx context.Context, dto ChangePasswordDTO) error {
	user, err := uc.userRepo.GetByID(ctx, dto.UserID)
	if err != nil || user == nil {
		return domain.ErrUnauthorized
	}

	// Verify current password
	hash := ""
	if user.PasswordHash != nil {
		hash = *user.PasswordHash
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(dto.CurrentPassword)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return domain.ErrInvalidCredentials
		}
		return domain.ErrInvalidCredentials
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(dto.NewPassword), 12)
	if err != nil {
		return err
	}

	return uc.userRepo.UpdatePasswordHash(ctx, dto.UserID, string(newHash))
}
