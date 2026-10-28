package domain

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// UserClaims represents the JWT claims for an authenticated user.
// ImpersonatingActor is set ONLY when a super-admin minted the token
// via /v1/owner/accounts/:id/impersonate; the dashboard reads it to
// render a banner and the audit middleware stamps it on every write
// performed during the impersonation session.
type UserClaims struct {
	UserID             string `json:"user_id"`
	UserName           string `json:"user_name"`
	AccountID          string `json:"account_id"`
	Role               string `json:"role"`
	ImpersonatingActor string `json:"impersonating_actor,omitempty"`
	jwt.RegisteredClaims
}

// ImpersonationClaims is the input bag the JWTService expects when
// minting a short-lived "act as a customer" token.
type ImpersonationClaims struct {
	UserID             string
	UserName           string
	AccountID          string
	Role               string
	ImpersonatingActor string
	ExpiresAt          time.Time
}

// MfaLoginClaims is the payload of the short-lived token returned to
// the client when a 2FA-enabled user has just passed password auth.
// The client trades it back via /auth/2fa/verify-login with a TOTP
// code; we then mint the real UserClaims JWT. Marked with audience
// "mfa_login" so it cannot be presented to any other endpoint.
type MfaLoginClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// JWTService defines the operations for generating and validating tokens
type JWTService interface {
	GenerateToken(user *User) (string, error)
	ValidateToken(token string) (*UserClaims, error)
	// GenerateImpersonationToken mints a short-lived token with the
	// supplied claims. Used exclusively by the owner panel — regular
	// auth flows must not have access to this method (it's wired on
	// the same service for proximity, but only the owner usecase
	// calls it).
	GenerateImpersonationToken(claims ImpersonationClaims) (string, error)
	// GenerateMfaLoginToken mints a 5-minute token that only the
	// /auth/2fa/verify-login endpoint accepts. Returned to the client
	// after password auth when the user has 2FA enabled.
	GenerateMfaLoginToken(userID string) (string, error)
	// ValidateMfaLoginToken validates a token minted by
	// GenerateMfaLoginToken. Refuses any token without the "mfa_login"
	// audience — so a regular UserClaims JWT can't be replayed here.
	ValidateMfaLoginToken(token string) (*MfaLoginClaims, error)
}
