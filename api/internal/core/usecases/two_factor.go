package usecases

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"

	"github.com/pquerna/otp/totp"
	"github.com/replider/api/internal/core/domain"
	"golang.org/x/crypto/bcrypt"
)

// TwoFactorUseCase owns TOTP enrollment, verification, and disablement
// for any user. The owner-panel gate that *requires* 2FA for super-
// admins is layered on top in phase 2 (login flow + middleware) — this
// use case is enrollment-grade scaffolding: opt-in for now.
//
// The flow is two-phase by design:
//  1. Setup: generate a fresh secret, persist it staged (totp_enabled
//     stays false), return secret + provisioning URI to the client
//     so it can render a QR. The user scans with an authenticator
//     app and enters a 6-digit code.
//  2. Verify: the handler calls Verify with the code; on success we
//     flip totp_enabled to true and stamp the recovery codes.
//
// This means a half-finished enrollment (Setup without Verify) does
// nothing — the secret is staged but never gates anything.
type TwoFactorUseCase struct {
	userRepo domain.UserRepository
	issuer   string
	// replay refuses a TOTP code that was already accepted for the user
	// (same 30s step), per RFC 6238 §5.2.
	replay *totpReplayGuard
}

// NewTwoFactorUseCase wires the use case. issuer is the label that
// shows up in authenticator apps (e.g. "Replider"). Keep it stable —
// rotating it forces every enrolled user to re-pair, since most
// authenticator apps key entries by issuer + account.
func NewTwoFactorUseCase(userRepo domain.UserRepository, issuer string) *TwoFactorUseCase {
	if issuer == "" {
		issuer = "Replider"
	}
	return &TwoFactorUseCase{userRepo: userRepo, issuer: issuer, replay: newTOTPReplayGuard()}
}

// SetupResult is what the Setup endpoint returns to the client. The
// secret is the canonical Base32 string for manual entry; otpauth_url
// is the provisioning URI (QR-encodable) that authenticator apps
// understand. Both are returned because mobile apps render QRs while
// web/desktop fallbacks need the manual string.
type SetupResult struct {
	Secret     string `json:"secret"`
	OTPAuthURL string `json:"otpauth_url"`
}

// Setup generates a fresh TOTP secret for the given user and stages
// it without flipping totp_enabled. Calling Setup on a user that
// already has 2FA enabled rotates the secret and disables the
// previous one — the previous codes stop working immediately. The
// caller (handler) should treat this as a destructive op and confirm
// with the user.
func (uc *TwoFactorUseCase) Setup(ctx context.Context, userID, email string) (*SetupResult, error) {
	if userID == "" {
		return nil, &domain.AppError{Code: "INVALID_REQUEST", Message: "user is required"}
	}
	// The /me/2fa routes run under authMw only, which sets userID on the
	// gin context but NOT userEmail — the JWT carries no email, and only
	// the super-admin middleware fetches the user row to stamp it. So the
	// handler passes an empty email here for a normal enrollment; resolve
	// it from the user row (the authenticator's AccountName label) instead
	// of failing with "user is required".
	if email == "" {
		u, err := uc.userRepo.GetByID(ctx, userID)
		if err != nil || u == nil {
			return nil, &domain.AppError{Code: "INVALID_REQUEST", Message: "user is required"}
		}
		email = u.Email
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      uc.issuer,
		AccountName: email,
	})
	if err != nil {
		return nil, fmt.Errorf("totp generate: %w", err)
	}
	if err := uc.userRepo.SetTOTPSecret(ctx, userID, key.Secret()); err != nil {
		// Surface a typed *AppError (e.g. TOTP_CIPHER_UNCONFIGURED when the
		// server has no TOTP_KEK) as-is — wrapping it with fmt.Errorf would
		// hide the type from respondError's assertion and collapse a clear
		// 4xx into an opaque 500 "internal server error".
		if appErr, ok := err.(*domain.AppError); ok {
			return nil, appErr
		}
		return nil, fmt.Errorf("persist totp secret: %w", err)
	}
	return &SetupResult{
		Secret:     key.Secret(),
		OTPAuthURL: key.URL(),
	}, nil
}

// Status reports whether the user has 2FA enrolled, read from the user
// row. The handler can't rely on a gin-context flag here: the
// /me/2fa/status route runs under authMw only, which never stamps a
// "totpEnabled" value, so a context read was always false even after a
// successful enrollment — the card showed "disabled" forever.
func (uc *TwoFactorUseCase) Status(ctx context.Context, userID string) (bool, error) {
	if userID == "" {
		return false, &domain.AppError{Code: "INVALID_REQUEST", Message: "user is required"}
	}
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, domain.ErrUserNotFound
	}
	return user.TOTPEnabled, nil
}

// VerifyResult is returned on the first successful Verify, including
// the plaintext recovery codes — shown ONCE to the user, never
// retrievable again. UI must persuade the user to write them down.
type VerifyResult struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

// Verify checks a 6-digit TOTP code against the staged secret. On
// the first success, it generates 10 recovery codes (8 chars each),
// bcrypt-hashes them for storage, and flips totp_enabled to true.
// Subsequent successful Verify calls (e.g. after Setup re-enrollment)
// behave the same way — fresh recovery codes replace any stale ones.
func (uc *TwoFactorUseCase) Verify(ctx context.Context, userID, code string) (*VerifyResult, error) {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, domain.ErrUserNotFound
	}
	if user.TOTPSecret == nil || *user.TOTPSecret == "" {
		return nil, &domain.AppError{Code: "TOTP_NOT_STAGED", Message: "Run setup before verify"}
	}
	if !uc.replay.accept(userID, strings.TrimSpace(code), *user.TOTPSecret) {
		return nil, &domain.AppError{Code: "TOTP_INVALID_CODE", Message: "Code did not match — check the time on your device and try again"}
	}
	plainCodes, hashed, err := generateRecoveryCodes(10)
	if err != nil {
		return nil, fmt.Errorf("generate recovery codes: %w", err)
	}
	if err := uc.userRepo.EnableTOTP(ctx, userID, hashed); err != nil {
		return nil, fmt.Errorf("enable totp: %w", err)
	}
	return &VerifyResult{RecoveryCodes: plainCodes}, nil
}

// VerifyForLogin checks a TOTP code OR a recovery code against an
// already-enabled user. Used by the (phase-2) login MFA pivot. The
// recovery branch consumes the code so it cannot be reused.
//
// Returns nil on success. Returns TOTP_INVALID_CODE on miss; never
// reveals which branch failed.
func (uc *TwoFactorUseCase) VerifyForLogin(ctx context.Context, userID, code string) error {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil || !user.TOTPEnabled || user.TOTPSecret == nil {
		return &domain.AppError{Code: "TOTP_NOT_ENABLED", Message: "User has no 2FA enrolled"}
	}
	trimmed := strings.TrimSpace(code)
	if uc.replay.accept(userID, trimmed, *user.TOTPSecret) {
		_ = uc.userRepo.StampTOTPVerified(ctx, userID)
		return nil
	}
	for _, hashed := range user.TOTPRecoveryCodes {
		if bcrypt.CompareHashAndPassword([]byte(hashed), []byte(trimmed)) == nil {
			ok, err := uc.userRepo.ConsumeTOTPRecoveryCode(ctx, userID, hashed)
			if err != nil {
				return fmt.Errorf("consume recovery code: %w", err)
			}
			if !ok {
				continue
			}
			_ = uc.userRepo.StampTOTPVerified(ctx, userID)
			return nil
		}
	}
	return &domain.AppError{Code: "TOTP_INVALID_CODE", Message: "Code did not match"}
}

// Disable tears down 2FA for a user. The handler must already have
// proved possession (current TOTP code or recovery code) — this use
// case enforces that proof too, so a leaked auth JWT alone cannot
// disable 2FA. Pass an empty code to skip the proof when the call
// originates from an admin recovery path (not exposed publicly today).
func (uc *TwoFactorUseCase) Disable(ctx context.Context, userID, proofCode string) error {
	if proofCode == "" {
		return &domain.AppError{Code: "TOTP_PROOF_REQUIRED", Message: "Provide a current 2FA code or recovery code to disable"}
	}
	if err := uc.VerifyForLogin(ctx, userID, proofCode); err != nil {
		return err
	}
	return uc.userRepo.DisableTOTP(ctx, userID)
}

// generateRecoveryCodes emits N plaintext recovery codes plus their
// bcrypt hashes. Plaintext is in the shape "ABCD-EFGH" (Crockford
// alphabet, 8 chars + a dash for readability). The caller persists
// the hashes and shows the plaintext to the user exactly once.
func generateRecoveryCodes(n int) (plain []string, hashed []string, err error) {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ" // Crockford-ish (no I/L/O/U)
	plain = make([]string, 0, n)
	hashed = make([]string, 0, n)
	for i := 0; i < n; i++ {
		raw := make([]byte, 8)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		for j, byteVal := range raw {
			b.WriteByte(alphabet[int(byteVal)%len(alphabet)])
			if j == 3 {
				b.WriteByte('-')
			}
		}
		code := b.String()
		h, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, nil, err
		}
		plain = append(plain, code)
		hashed = append(hashed, string(h))
	}
	return plain, hashed, nil
}

// secretFromBase32 is exposed for tests that want to forge a code
// against a known secret. Not used in production paths.
func secretFromBase32(s string) ([]byte, error) {
	return base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(s))
}

var _ = secretFromBase32 // keep referenced to avoid removing in cleanup
