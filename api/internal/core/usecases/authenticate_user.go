package usecases

import (
	"context"
	"log/slog"

	"github.com/replider/api/internal/core/domain"
	"golang.org/x/crypto/bcrypt"
)

type AuthenticateUserUseCase struct {
	userRepo    domain.UserRepository
	accountRepo domain.AccountRepository
	jwtService  domain.JWTService
	// lockout counts failed logins per e-mail: 10 in 15 minutes answers
	// AUTH_LOCKED (429) until the window ends. Keyed by e-mail, not IP —
	// the IP the request appears to come from is the attacker's choice.
	lockout *loginLockout
}

func NewAuthenticateUserUseCase(userRepo domain.UserRepository, accountRepo domain.AccountRepository, jwtService domain.JWTService) *AuthenticateUserUseCase {
	return &AuthenticateUserUseCase{
		userRepo:    userRepo,
		accountRepo: accountRepo,
		jwtService:  jwtService,
		lockout:     newLoginLockout(),
	}
}

type AuthenticateUserDTO struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthenticateUserResponse struct {
	Token string `json:"token,omitempty"`
	IsNew bool   `json:"is_new"`
	// MfaRequired is true when the user has 2FA enabled. The client
	// receives MfaSession (instead of Token) and trades it back via
	// /auth/2fa/verify-login with a fresh TOTP code.
	MfaRequired bool   `json:"mfa_required,omitempty"`
	MfaSession  string `json:"mfa_session,omitempty"`
}

func (uc *AuthenticateUserUseCase) Execute(ctx context.Context, dto AuthenticateUserDTO) (*AuthenticateUserResponse, error) {
	// 0. Lockout — checked before the user lookup so a locked e-mail costs
	// no bcrypt and answers the same whether or not the account exists.
	if uc.lockout.locked(dto.Email) {
		return nil, ErrAuthLocked
	}

	// 1. Fetch user by email
	user, err := uc.userRepo.GetByEmail(ctx, dto.Email)
	if err != nil {
		slog.Error("failed to fetch user by email", "error", err)
		uc.lockout.fail(dto.Email)
		return nil, domain.ErrInvalidCredentials
	}

	if user == nil {
		uc.lockout.fail(dto.Email)
		return nil, domain.ErrInvalidCredentials
	}

	// 2. Verify password (social-only users have no password)
	if user.PasswordHash == nil {
		slog.Warn("user has no password", "user_id", user.ID)
		uc.lockout.fail(dto.Email)
		return nil, domain.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(dto.Password)); err != nil {
		slog.Warn("error comparing password", "error", err)
		uc.lockout.fail(dto.Email)
		return nil, domain.ErrInvalidCredentials
	}
	uc.lockout.reset(dto.Email)

	// Email verification is not a hard login gate — blocking here locks
	// users out of their own account when the verification mail bounces,
	// lands in spam, or arrives after they close the tab. The /me
	// response exposes is_email_verified so the UI can nudge users to
	// confirm when it matters.

	// 3. 2FA pivot — if the user has TOTP enabled, swap the full JWT
	// for a 5-minute MFA session token. The client must present that
	// to /auth/2fa/verify-login with a current TOTP (or recovery)
	// code to trade up for a real session.
	if user.TOTPEnabled {
		mfaToken, err := uc.jwtService.GenerateMfaLoginToken(user.ID)
		if err != nil {
			return nil, err
		}
		return &AuthenticateUserResponse{
			MfaRequired: true,
			MfaSession:  mfaToken,
		}, nil
	}

	// 4. Generate JWT
	token, err := uc.jwtService.GenerateToken(user)
	if err != nil {
		return nil, err
	}

	// 5. Check if onboarding is pending
	isNew := false
	account, err := uc.accountRepo.GetByID(ctx, user.AccountID)
	if err == nil && onboardingPendingFor(account, user) {
		isNew = true
	}

	return &AuthenticateUserResponse{
		Token: token,
		IsNew: isNew,
	}, nil
}
