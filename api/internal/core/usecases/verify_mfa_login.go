package usecases

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

// VerifyMfaLoginUseCase trades a short-lived MFA session token + a
// 6-digit TOTP code (or a recovery code) for a regular UserClaims
// JWT. Used as the second step of the password-login flow when the
// user has 2FA enabled.
type VerifyMfaLoginUseCase struct {
	userRepo    domain.UserRepository
	accountRepo domain.AccountRepository
	jwtService  domain.JWTService
	twoFactorUC *TwoFactorUseCase
}

func NewVerifyMfaLoginUseCase(
	userRepo domain.UserRepository,
	accountRepo domain.AccountRepository,
	jwtService domain.JWTService,
	twoFactorUC *TwoFactorUseCase,
) *VerifyMfaLoginUseCase {
	return &VerifyMfaLoginUseCase{
		userRepo:    userRepo,
		accountRepo: accountRepo,
		jwtService:  jwtService,
		twoFactorUC: twoFactorUC,
	}
}

type VerifyMfaLoginDTO struct {
	MfaSession string `json:"mfa_session"`
	Code       string `json:"code"`
}

type VerifyMfaLoginResponse struct {
	Token string `json:"token"`
	IsNew bool   `json:"is_new"`
}

func (uc *VerifyMfaLoginUseCase) Execute(ctx context.Context, dto VerifyMfaLoginDTO) (*VerifyMfaLoginResponse, error) {
	if dto.MfaSession == "" || dto.Code == "" {
		return nil, &domain.AppError{Code: "INVALID_REQUEST", Message: "mfa_session and code are required"}
	}
	claims, err := uc.jwtService.ValidateMfaLoginToken(dto.MfaSession)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}
	if err := uc.twoFactorUC.VerifyForLogin(ctx, claims.UserID, dto.Code); err != nil {
		return nil, err
	}
	user, err := uc.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, domain.ErrInvalidCredentials
	}
	token, err := uc.jwtService.GenerateToken(user)
	if err != nil {
		return nil, err
	}
	isNew := false
	if account, err := uc.accountRepo.GetByID(ctx, user.AccountID); err == nil && account != nil && !account.OnboardingCompleted {
		isNew = true
	}
	return &VerifyMfaLoginResponse{Token: token, IsNew: isNew}, nil
}
