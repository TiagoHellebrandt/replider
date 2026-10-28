package usecases

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/replider/api/internal/core/domain"
)

// The owner side of account invites (docs/fluxo-criacao-conta.md, Fase 1):
// emission, listing and revocation. The redeem side lives inside
// CreateAccountUseCase — the invite IS the permission that overrides the
// closed registration door, so the two must be one code path, not two.

const (
	inviteDefaultDays = 7
	inviteMaxDays     = 30
)

type InviteUseCase struct {
	invites  domain.AccountInviteRepository
	planRepo domain.PlanRepository
	// audit is best-effort but wired in production: an invite is a door into
	// the platform, and doors get logged.
	audit             domain.AuditLogRepository
	platformAccountID string
	now               func() time.Time
}

func NewInviteUseCase(
	invites domain.AccountInviteRepository,
	planRepo domain.PlanRepository,
) *InviteUseCase {
	return &InviteUseCase{invites: invites, planRepo: planRepo, now: time.Now}
}

// SetAudit wires the audit trail. audit_logs.account_id is NOT NULL, so the
// emission rows are filed under the PLATFORM account (the invited account
// does not exist yet).
func (uc *InviteUseCase) SetAudit(a domain.AuditLogRepository, platformAccountID string) {
	uc.audit = a
	uc.platformAccountID = platformAccountID
}

type CreateInviteDTO struct {
	ActorUserID   string
	ActorEmail    string
	Email         string
	PlanID        string
	Overrides     domain.AccountLimitOverrides
	ExpiresInDays int
}

// CreatedInvite carries the ONE exposure of the secret. It is never stored
// and never logged; losing it means revoking and reissuing.
type CreatedInvite struct {
	Invite *domain.AccountInvite `json:"invite"`
	Token  string                `json:"token"`
}

func (uc *InviteUseCase) Create(ctx context.Context, dto CreateInviteDTO) (*CreatedInvite, error) {
	email := strings.ToLower(strings.TrimSpace(dto.Email))
	if !domain.IsEmailAddress(email) {
		return nil, &domain.AppError{Code: "INVITE_EMAIL_INVALID", Message: "informe o e-mail do cliente"}
	}
	plan, err := uc.planRepo.GetByID(ctx, strings.TrimSpace(dto.PlanID))
	if err != nil || plan == nil {
		return nil, &domain.AppError{Code: "INVITE_PLAN_NOT_FOUND", Message: "plan_id não encontrado"}
	}

	days := dto.ExpiresInDays
	if days <= 0 {
		days = inviteDefaultDays
	}
	if days > inviteMaxDays {
		days = inviteMaxDays
	}

	secret, hash, err := domain.NewInviteToken()
	if err != nil {
		return nil, err
	}
	invite := &domain.AccountInvite{
		ID:        ulid.Make().String(),
		Email:     email,
		PlanID:    plan.ID,
		Overrides: dto.Overrides,
		ExpiresAt: uc.now().Add(time.Duration(days) * 24 * time.Hour),
		CreatedAt: uc.now(),
	}
	if actor := strings.TrimSpace(dto.ActorUserID); actor != "" {
		invite.InvitedBy = &actor
	}
	if err := uc.invites.Create(ctx, invite, hash); err != nil {
		return nil, err
	}

	uc.auditLog(ctx, "invite.created", dto.ActorEmail, map[string]any{
		"invite_id": invite.ID, "email": email, "plan_id": plan.ID,
		"expires_at": invite.ExpiresAt,
	})
	return &CreatedInvite{Invite: invite, Token: secret}, nil
}

func (uc *InviteUseCase) List(ctx context.Context) ([]*domain.AccountInvite, error) {
	return uc.invites.ListOpen(ctx)
}

// Plans is the unfiltered catalogue for the owner panel — the public /plans
// hides unlisted plans, which is every SDR contract tier.
func (uc *InviteUseCase) Plans(ctx context.Context) ([]*domain.Plan, error) {
	return uc.planRepo.ListAll(ctx)
}

func (uc *InviteUseCase) Revoke(ctx context.Context, actorEmail, id string) error {
	ok, err := uc.invites.Revoke(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return &domain.AppError{Code: "INVITE_NOT_PENDING", Message: "convite não está pendente"}
	}
	uc.auditLog(ctx, "invite.revoked", actorEmail, map[string]any{"invite_id": id})
	return nil
}

func (uc *InviteUseCase) auditLog(ctx context.Context, action, actorEmail string, payload map[string]any) {
	if uc.audit == nil || uc.platformAccountID == "" {
		return
	}
	// Actor e-mail goes in the payload, never in ActorID — that column is a
	// users(id) FK and an e-mail there silently fails the write (the exact
	// trap the commission audits already documented).
	payload["actor_email"] = actorEmail
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	entry := &domain.AuditLog{
		ID:         ulid.Make().String(),
		AccountID:  uc.platformAccountID,
		Action:     action,
		EntityType: "account_invite",
		Payload:    raw,
	}
	if err := uc.audit.Create(ctx, entry); err != nil {
		// Best-effort but never silent: the whole point of auditing a door.
		slog.WarnContext(ctx, "invite audit write failed", "action", action, "error", err)
	}
}

// Probe answers the register screen's "is this link any good?" BEFORE the
// person types a password. One uniform answer for every bad token.
type InviteProbe struct {
	Valid bool   `json:"valid"`
	Email string `json:"email,omitempty"`
}

func (uc *InviteUseCase) Probe(ctx context.Context, token string) InviteProbe {
	token = strings.TrimSpace(token)
	if token == "" {
		return InviteProbe{}
	}
	inv, err := uc.invites.FindValid(ctx, domain.HashInviteToken(token))
	if err != nil {
		// Fail closed: an unreadable invite is not a valid invite.
		slog.WarnContext(ctx, "invite probe lookup failed", "error", err)
		return InviteProbe{}
	}
	if inv == nil {
		return InviteProbe{}
	}
	// The e-mail leaks only to the holder of the secret, and the register
	// screen needs it to lock the field.
	return InviteProbe{Valid: true, Email: inv.Email}
}
