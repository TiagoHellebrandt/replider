package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Account invites (docs/fluxo-criacao-conta.md, Fase 1).
//
// The product is sales-led: an account exists because WE decided it exists.
// With the public door closed (REGISTRATION_ENABLED=false), a one-shot invite
// is the only permission to create one. The invite carries the deal — which
// plan, which limits — so the account is born contracted, never on trial.

// AccountInvite is one permission to create one account.
type AccountInvite struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	// PlanID + Overrides are the contract the redeemed account is born on —
	// same shape the owner panel's apply-contract uses.
	PlanID    string                `json:"plan_id"`
	Overrides AccountLimitOverrides `json:"overrides"`
	// InvitedBy records the super-admin who issued it. Nullable: the emitter
	// outliving the invite matters less than the invite surviving the emitter.
	InvitedBy     *string    `json:"invited_by,omitempty"`
	ExpiresAt     time.Time  `json:"expires_at"`
	UsedAt        *time.Time `json:"used_at,omitempty"`
	UsedAccountID *string    `json:"used_account_id,omitempty"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// NewInviteToken mints the secret the customer receives and the hash the
// database keeps. The secret is shown ONCE and never stored — the same
// posture as the machine API keys.
func NewInviteToken() (secret, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	secret = hex.EncodeToString(raw)
	return secret, HashInviteToken(secret), nil
}

// HashInviteToken is the only transformation between the link and the row.
func HashInviteToken(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// AccountInviteRepository persists the permission and its one-shot consumption.
type AccountInviteRepository interface {
	Create(ctx context.Context, invite *AccountInvite, tokenHash string) error
	// FindValid returns the invite behind a hash when it is redeemable —
	// not used, not revoked, not expired. (nil, nil) otherwise: exists,
	// expired and revoked must be indistinguishable to a caller probing
	// tokens (anti-enumeration).
	FindValid(ctx context.Context, tokenHash string) (*AccountInvite, error)
	// Consume burns the invite atomically (WHERE used_at IS NULL … RETURNING).
	// false = someone else won the race, or the invite stopped being valid —
	// either way the caller must refuse.
	Consume(ctx context.Context, id, accountID string) (bool, error)
	// Unconsume is the best-effort revert for "consumed but account creation
	// failed" — a burned invite the owner has to reissue is the accepted cost
	// of a crash in that window, but a clean failure should not cost one.
	Unconsume(ctx context.Context, id string) error
	// Revoke cancels a pending invite. false = nothing pending to revoke.
	Revoke(ctx context.Context, id string) (bool, error)
	// ListOpen returns pending (unused, unrevoked, unexpired) invites for
	// the owner panel.
	ListOpen(ctx context.Context) ([]*AccountInvite, error)
}

// ErrInviteInvalid is deliberately ONE code for every refusal — nonexistent,
// expired, used, revoked, wrong e-mail. Distinguishing them would hand an
// enumeration oracle to whoever is guessing tokens.
var ErrInviteInvalid = &AppError{
	Code:    "INVITE_INVALID",
	Message: "convite inválido, expirado ou já utilizado — peça um novo link para quem convidou você",
}
