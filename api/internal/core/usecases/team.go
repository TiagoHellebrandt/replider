package usecases

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/email"
	"github.com/replider/api/internal/core/domain"
	"golang.org/x/crypto/bcrypt"
)

// --- ListTeamMembersUseCase ---

type ListTeamMembersUseCase struct {
	userRepo domain.UserRepository
}

func NewListTeamMembersUseCase(userRepo domain.UserRepository) *ListTeamMembersUseCase {
	return &ListTeamMembersUseCase{userRepo: userRepo}
}

type TeamMember struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Email           string     `json:"email"`
	Role            string     `json:"role"`
	IsActive        bool       `json:"is_active"`
	IsEmailVerified bool       `json:"is_email_verified"`
	InviteSentAt    *time.Time `json:"invite_sent_at,omitempty"`
	LastSeenAt      *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (uc *ListTeamMembersUseCase) Execute(ctx context.Context, accountID string) ([]TeamMember, error) {
	users, err := uc.userRepo.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}

	members := make([]TeamMember, 0, len(users))
	for _, u := range users {
		role := "attendant"
		if u.Role != nil {
			role = *u.Role
		}
		members = append(members, TeamMember{
			ID:              u.ID,
			Name:            u.Name,
			Email:           u.Email,
			Role:            role,
			IsActive:        u.IsActive,
			IsEmailVerified: u.IsEmailVerified,
			InviteSentAt:    u.InviteSentAt,
			LastSeenAt:      u.LastSeenAt,
			CreatedAt:       u.CreatedAt,
			UpdatedAt:       u.UpdatedAt,
		})
	}
	return members, nil
}

// --- InviteTeamMemberUseCase ---

type InviteTeamMemberUseCase struct {
	userRepo     domain.UserRepository
	accountRepo  domain.AccountRepository
	planRepo     domain.PlanRepository
	emailService email.EmailService
	appBaseURL   string
}

func NewInviteTeamMemberUseCase(
	userRepo domain.UserRepository,
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	emailService email.EmailService,
	appBaseURL string,
) *InviteTeamMemberUseCase {
	return &InviteTeamMemberUseCase{
		userRepo:     userRepo,
		accountRepo:  accountRepo,
		planRepo:     planRepo,
		emailService: emailService,
		appBaseURL:   appBaseURL,
	}
}

type InviteTeamMemberDTO struct {
	AccountID   string
	InviterName string
	Name        string
	Email       string
	Role        string
}

func (uc *InviteTeamMemberUseCase) Execute(ctx context.Context, dto InviteTeamMemberDTO) (*TeamMember, error) {
	// Validate role
	if dto.Role != "admin" && dto.Role != "attendant" {
		return nil, domain.ErrInvalidRole
	}

	// Enforce the seat cap. Active members (not deactivated) count
	// toward the cap; deactivated rows don't since they no longer take
	// a seat. Resolved via account.EffectiveMaxSeats so per-account
	// overrides apply and contract/enterprise plans (MaxSeats == 0,
	// "negotiate per deal") resolve to UnlimitedLimit instead of the
	// trial floor — otherwise a contract account is capped at 1 seat.
	// Falls back to a safe 1 when plan isn't resolvable.
	if uc.accountRepo != nil && uc.planRepo != nil {
		maxSeats := 1
		if acc, err := uc.accountRepo.GetByID(ctx, dto.AccountID); err == nil && acc != nil && acc.PlanID != nil {
			if plan, err := uc.planRepo.GetByID(ctx, *acc.PlanID); err == nil && plan != nil {
				maxSeats = acc.EffectiveMaxSeats(plan)
			}
		}
		existingUsers, err := uc.userRepo.ListByAccountID(ctx, dto.AccountID)
		if err != nil {
			return nil, err
		}
		activeCount := 0
		for _, u := range existingUsers {
			if u.IsActive {
				activeCount++
			}
		}
		if activeCount >= maxSeats {
			return nil, domain.NewPlanLimitError("seats", activeCount, maxSeats)
		}
	}

	// Check if user already exists
	existing, err := uc.userRepo.GetByEmail(ctx, dto.Email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, domain.ErrEmailAlreadyExists
	}

	// Generate a temporary password hash (user will reset via email)
	tempPassword := make([]byte, 16)
	if _, err := rand.Read(tempPassword); err != nil {
		return nil, err
	}
	hashed, err := bcrypt.GenerateFromPassword(tempPassword, 12)
	if err != nil {
		return nil, err
	}
	hashedStr := string(hashed)

	// Generate reset token so user can set their own password
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	resetToken := hex.EncodeToString(tokenBytes)

	now := time.Now()
	role := dto.Role
	user := &domain.User{
		ID:              ulid.Make().String(),
		AccountID:       dto.AccountID,
		Name:            dto.Name,
		Email:           dto.Email,
		PasswordHash:    &hashedStr,
		Role:            &role,
		IsActive:        true,
		IsEmailVerified: true, // Invited users don't need to verify
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// Set reset token so user can set their password
	expiresAt := now.Add(24 * time.Hour)
	if err := uc.userRepo.SetResetToken(ctx, user.ID, resetToken, expiresAt); err != nil {
		return nil, err
	}

	// Send invite email with password reset link
	if err := uc.emailService.SendTeamInviteEmail(dto.Email, dto.InviterName, resetToken, uc.appBaseURL,
		emailLocaleFor(ctx, uc.accountRepo, dto.AccountID)); err != nil {
		// Log but don't fail — user was created successfully
		// The admin can always trigger a password reset for the user
	}
	// Stamp invite_sent_at so the Team screen can render the "Pending"
	// badge. Best-effort — not fatal if the column write fails.
	_ = uc.userRepo.MarkInviteSent(ctx, user.ID)

	return &TeamMember{
		ID:              user.ID,
		Name:            user.Name,
		Email:           user.Email,
		Role:            role,
		IsActive:        user.IsActive,
		IsEmailVerified: user.IsEmailVerified,
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
	}, nil
}

// --- ResendInviteUseCase ---

type ResendInviteUseCase struct {
	userRepo     domain.UserRepository
	emailService email.EmailService
	appBaseURL   string
	accountRepo  domain.AccountRepository
}

// SetAccountRepo wires the account lookup used to localize the invite
// email (US-018 review fixes). Optional — nil keeps pt-BR copy.
func (uc *ResendInviteUseCase) SetAccountRepo(repo domain.AccountRepository) {
	uc.accountRepo = repo
}

func NewResendInviteUseCase(userRepo domain.UserRepository, emailService email.EmailService, appBaseURL string) *ResendInviteUseCase {
	return &ResendInviteUseCase{userRepo: userRepo, emailService: emailService, appBaseURL: appBaseURL}
}

type ResendInviteDTO struct {
	AccountID   string
	InviterName string
	TargetID    string
}

// Execute regenerates the reset token, emails the user, and stamps
// invite_sent_at = NOW(). Idempotent w.r.t. the user row — fine to
// spam multiple times (backend rate-limits the endpoint).
func (uc *ResendInviteUseCase) Execute(ctx context.Context, dto ResendInviteDTO) error {
	target, err := uc.userRepo.GetByID(ctx, dto.TargetID)
	if err != nil {
		return err
	}
	if target == nil || target.AccountID != dto.AccountID {
		return domain.ErrUserNotFound
	}
	// Only make sense to resend if the user hasn't completed signup.
	// Once they've verified / logged in, a password reset is the
	// right affordance, not a re-invite.
	if target.IsEmailVerified {
		return domain.ErrInviteAlreadyAccepted
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return err
	}
	resetToken := hex.EncodeToString(tokenBytes)
	expiresAt := time.Now().Add(24 * time.Hour)
	if err := uc.userRepo.SetResetToken(ctx, target.ID, resetToken, expiresAt); err != nil {
		return err
	}
	// Swallow email error (same as invite path) — invite_sent_at will
	// still flip so the admin sees the attempt was recorded.
	_ = uc.emailService.SendTeamInviteEmail(target.Email, dto.InviterName, resetToken, uc.appBaseURL,
		emailLocaleFor(ctx, uc.accountRepo, dto.AccountID))
	return uc.userRepo.MarkInviteSent(ctx, target.ID)
}

// --- UpdateMemberRoleUseCase ---

type UpdateMemberRoleUseCase struct {
	userRepo domain.UserRepository
}

func NewUpdateMemberRoleUseCase(userRepo domain.UserRepository) *UpdateMemberRoleUseCase {
	return &UpdateMemberRoleUseCase{userRepo: userRepo}
}

type UpdateMemberRoleDTO struct {
	AccountID string
	UserID    string
	Role      string
}

func (uc *UpdateMemberRoleUseCase) Execute(ctx context.Context, dto UpdateMemberRoleDTO) error {
	if dto.Role != "admin" && dto.Role != "attendant" {
		return domain.ErrInvalidRole
	}

	user, err := uc.userRepo.GetByID(ctx, dto.UserID)
	if err != nil {
		return err
	}
	if user == nil || user.AccountID != dto.AccountID {
		return domain.ErrMemberNotFound
	}

	return uc.userRepo.UpdateRole(ctx, dto.UserID, dto.Role)
}

// --- RemoveMemberUseCase ---

type RemoveMemberUseCase struct {
	userRepo domain.UserRepository
}

func NewRemoveMemberUseCase(userRepo domain.UserRepository) *RemoveMemberUseCase {
	return &RemoveMemberUseCase{userRepo: userRepo}
}

type RemoveMemberDTO struct {
	AccountID    string
	UserID       string
	RequestingID string // The user performing the action
}

func (uc *RemoveMemberUseCase) Execute(ctx context.Context, dto RemoveMemberDTO) error {
	// Cannot remove yourself
	if dto.UserID == dto.RequestingID {
		return domain.ErrCannotRemoveSelf
	}

	user, err := uc.userRepo.GetByID(ctx, dto.UserID)
	if err != nil {
		return err
	}
	if user == nil || user.AccountID != dto.AccountID {
		return domain.ErrMemberNotFound
	}

	// Cannot remove the last active admin — would orphan the account.
	// RoleSatisfies rather than equality: if the last administrator is the OWNER,
	// comparing against the string let them be removed and orphaned the
	// account.
	if domain.RoleSatisfies(roleOf(user), domain.RoleAdminRole) {
		admin, err := uc.userRepo.GetAdminByAccountID(ctx, dto.AccountID)
		if err != nil {
			return err
		}
		if admin != nil && admin.ID == dto.UserID {
			return domain.ErrCannotRemoveLastAdmin
		}
	}

	return uc.userRepo.Deactivate(ctx, dto.UserID)
}
