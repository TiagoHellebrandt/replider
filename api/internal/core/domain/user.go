package domain

import (
	"time"
)

// User roles. `admin` and `attendant` are per-account roles enforced
// by middleware.RequireRole. `super_admin` is platform-level and only
// granted via the SUPER_ADMIN_EMAILS env allowlist (resolved at the
// middleware layer, not stored in the users.role column today). Keep
// the enum exhaustive so callers don't sprinkle string literals.
const (
	RoleAdmin      = "admin"
	RoleAttendant  = "attendant"
	RoleSuperAdmin = "super_admin"
)

type User struct {
	ID           string  `json:"id"`
	AccountID    string  `json:"account_id"`
	Name         string  `json:"name"`
	Email        string  `json:"email"`
	PasswordHash *string `json:"-"` // nullable for social-only users
	Role         *string `json:"role" enums:"admin,attendant,super_admin"`
	IsActive     bool    `json:"is_active"`
	GoogleID     *string `json:"google_id,omitempty"`
	MetaID       *string `json:"meta_id,omitempty"`
	Picture      *string `json:"picture,omitempty"` // Profile picture URL (populated for Google sign-ins)
	// Signature is an optional short text the attendant can append to
	// MANUAL replies from the composer (US-018). Client-side insert only —
	// never applied to AI responses.
	Signature                  *string    `json:"signature,omitempty"`
	ResetToken                 *string    `json:"-"`
	ResetTokenExpiresAt        *time.Time `json:"-"`
	IsEmailVerified            bool       `json:"is_email_verified"`
	VerificationToken          *string    `json:"-"`
	VerificationTokenExpiresAt *time.Time `json:"-"`
	// LGPD: when the user explicitly accepted the Terms + Privacy Policy
	// (NULL = never; legacy users get a v0 backfill via migration). The
	// version is stored so a Terms update can require fresh acceptance.
	TermsAcceptedAt *time.Time `json:"terms_accepted_at,omitempty"`
	TermsVersion    *string    `json:"terms_version,omitempty"`
	// InviteSentAt captures the moment an admin invited (or re-invited)
	// this user. Pairs with !is_email_verified to decide whether the
	// Team screen renders a "Pending" badge + "Resend" action.
	InviteSentAt *time.Time `json:"invite_sent_at,omitempty"`
	// LastSeenAt is stamped by the auth middleware on JWT-validated
	// requests (throttled to once per 5 minutes at the SQL layer).
	// Nullable because brand-new invited users haven't logged in yet.
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
	// TOTPSecret is the Base32-encoded shared secret. It is staged on
	// enrollment and only goes "live" once TOTPEnabled flips to true.
	// Marshalled out (json:"-") because we never want to leak it past
	// the enrollment QR.
	TOTPSecret  *string `json:"-"`
	TOTPEnabled bool    `json:"totp_enabled"`
	// TOTPRecoveryCodes are bcrypt hashes of one-time recovery codes.
	// Plaintext is shown to the user exactly once, at enrollment.
	TOTPRecoveryCodes []string `json:"-"`
	// TOTPVerifiedAt stamps the most recent successful 2FA proof. The
	// owner middleware reads this in phase 2 to decide whether to
	// re-prompt the operator.
	TOTPVerifiedAt *time.Time `json:"-"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// CurrentTermsVersion is the version string applied to new acceptances.
// Bump this constant whenever the published Terms or Privacy Policy
// document changes materially.
const CurrentTermsVersion = "v1-2026-04"
