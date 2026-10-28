package usecases

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// OwnerSellersUseCase backs the owner panel's Vendedores section
// (owner-sellers US-001): one list of the platform account's users with
// their full seller state, and one atomic onboard action replacing the
// fragmented flag-then-remember-the-rotation flow. Onboarding sets
// is_seller, mints the ref code and joins the rotation in a single
// transaction; offboarding removes from the rotation and deactivates
// (starting the residual window) but NEVER touches the commission
// ledger — history stays payable/auditable.
type OwnerSellersUseCase struct {
	repo     domain.OwnerSellerRepository
	users    commissionUserGetter
	auditLog domain.AuditLogRepository
	// platformAccountID scopes everything; unset deployments get a 503
	// (OWNER_PLATFORM_NOT_CONFIGURED) instead of listing nothing.
	platformAccountID string
	// webBaseURL is the marketing-site origin used to render each
	// seller's full ?ref= signup link (same source as seller_ref.go).
	webBaseURL string
	// now is swappable in tests.
	now func() time.Time
}

func NewOwnerSellersUseCase(
	repo domain.OwnerSellerRepository,
	users commissionUserGetter,
	auditLog domain.AuditLogRepository,
	platformAccountID string,
	webBaseURL string,
) *OwnerSellersUseCase {
	return &OwnerSellersUseCase{
		repo:              repo,
		users:             users,
		auditLog:          auditLog,
		platformAccountID: platformAccountID,
		webBaseURL:        webBaseURL,
		now:               time.Now,
	}
}

func errOwnerPlatformNotConfigured() error {
	return &domain.AppError{Code: "OWNER_PLATFORM_NOT_CONFIGURED", Message: "PLATFORM_ACCOUNT_ID is not configured on this deployment"}
}

// List returns every platform-account user with seller state and
// rotation membership resolved. Rotation semantics mirror
// AutoAssignUseCase: no persisted row defaults to enabled for the
// platform account (US-020), and an empty participant list means every
// active admin/attendant rotates.
func (uc *OwnerSellersUseCase) List(ctx context.Context) ([]domain.OwnerSellerRow, error) {
	if uc.platformAccountID == "" {
		return nil, errOwnerPlatformNotConfigured()
	}
	rows, err := uc.repo.ListPlatformSellerRows(ctx, uc.platformAccountID)
	if err != nil {
		return nil, err
	}
	settings, err := uc.repo.GetAutoAssignSettings(ctx, uc.platformAccountID)
	if err != nil {
		return nil, err
	}
	enabled, participants := rotationState(settings)
	inList := make(map[string]struct{}, len(participants))
	for _, id := range participants {
		inList[id] = struct{}{}
	}
	for i := range rows {
		rows[i].InRotation = enabled && rowRotates(rows[i], inList)
		if code := rows[i].SellerRefCode; code != nil && *code != "" {
			link := uc.webBaseURL + "/register?ref=" + *code
			rows[i].ShareURL = &link
		}
	}
	return rows, nil
}

// SetSeller flags/unflags one platform user as vendedor, atomically.
// isSeller=true: flag + activate + mint ref code + join rotation.
// isSeller=false: unflag + deactivate (stamps the 12-month residual
// window) + leave rotation. The ledger is never touched either way.
// Returns the user's fresh row so the caller renders without a
// re-list.
func (uc *OwnerSellersUseCase) SetSeller(ctx context.Context, actorEmail, userID string, isSeller bool) (*domain.OwnerSellerRow, error) {
	if uc.platformAccountID == "" {
		return nil, errOwnerPlatformNotConfigured()
	}
	user, err := uc.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil || user.AccountID != uc.platformAccountID {
		return nil, errCommissionSellerNotFound()
	}

	err = uc.repo.InTx(ctx, func(tx domain.OwnerSellerRepository) error {
		if err := tx.SetSellerFlag(ctx, userID, isSeller); err != nil {
			return err
		}
		if isSeller {
			if err := tx.UpsertSellerActivity(ctx, userID, true, nil, nil); err != nil {
				return err
			}
			if _, err := tx.GetOrCreateSellerRefCode(ctx, userID); err != nil {
				return err
			}
			return uc.joinRotation(ctx, tx, userID)
		}
		now := uc.now()
		until := now.AddDate(0, residualMonths, 0)
		if err := tx.UpsertSellerActivity(ctx, userID, false, &now, &until); err != nil {
			return err
		}
		return uc.leaveRotation(ctx, tx, userID)
	})
	if err != nil {
		return nil, err
	}

	uc.audit(ctx, actorEmail, "seller.onboard", userID, isSeller, user.Role)
	slog.Info("owner sellers: seller onboard toggled",
		"actor", actorEmail, "seller_user_id", userID, "is_seller", isSeller)
	// Allowed but noteworthy: an admin seller earns commission/SLA on
	// accounts he can also administer (self-commission risk, US-003).
	// RoleSatisfies: the self-commission risk belongs to the account's owner as
	// much as to an admin — comparing against the string left the most serious
	// case as the only one that passed in silence.
	if isSeller && domain.RoleSatisfies(roleOf(user), domain.RoleAdminRole) {
		slog.Warn("owner sellers: admin flagged as seller",
			"actor", actorEmail, "seller_user_id", userID)
	}

	rows, err := uc.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].UserID == userID {
			return &rows[i], nil
		}
	}
	return nil, errCommissionSellerNotFound()
}

// joinRotation adds the user to the platform rotation. With no
// persisted row or an empty participant list every active
// admin/attendant already rotates — writing [user] there would RESTRICT
// the rotation to him alone, so only an explicit non-empty list gets
// the append.
func (uc *OwnerSellersUseCase) joinRotation(ctx context.Context, tx domain.OwnerSellerRepository, userID string) error {
	settings, err := tx.GetAutoAssignSettings(ctx, uc.platformAccountID)
	if err != nil {
		return err
	}
	enabled, participants := rotationState(settings)
	if len(participants) == 0 {
		return nil // everyone participates already
	}
	for _, id := range participants {
		if id == userID {
			return nil
		}
	}
	return tx.UpsertAutoAssignSettings(ctx, &domain.AutoAssignSettings{
		AccountID:          uc.platformAccountID,
		Enabled:            enabled,
		ParticipantUserIDs: append(participants, userID),
	})
}

// leaveRotation removes the user from the platform rotation. An empty
// participant list means "everyone" — removal has to materialize it
// (every other eligible member, explicitly) or the offboarded seller
// would keep receiving leads.
func (uc *OwnerSellersUseCase) leaveRotation(ctx context.Context, tx domain.OwnerSellerRepository, userID string) error {
	settings, err := tx.GetAutoAssignSettings(ctx, uc.platformAccountID)
	if err != nil {
		return err
	}
	enabled, participants := rotationState(settings)

	if len(participants) == 0 {
		rows, err := tx.ListPlatformSellerRows(ctx, uc.platformAccountID)
		if err != nil {
			return err
		}
		materialized := make([]string, 0, len(rows))
		wasRotating := false
		for _, r := range rows {
			if !rowRotates(r, nil) {
				continue
			}
			if r.UserID == userID {
				wasRotating = true
				continue
			}
			materialized = append(materialized, r.UserID)
		}
		if !wasRotating {
			return nil // wasn't in the implicit pool — nothing to persist
		}
		return tx.UpsertAutoAssignSettings(ctx, &domain.AutoAssignSettings{
			AccountID:          uc.platformAccountID,
			Enabled:            enabled,
			ParticipantUserIDs: materialized,
		})
	}

	kept := make([]string, 0, len(participants))
	for _, id := range participants {
		if id != userID {
			kept = append(kept, id)
		}
	}
	if len(kept) == len(participants) {
		return nil
	}
	return tx.UpsertAutoAssignSettings(ctx, &domain.AutoAssignSettings{
		AccountID:          uc.platformAccountID,
		Enabled:            enabled,
		ParticipantUserIDs: kept,
	})
}

// rotationState normalizes the persisted settings: no row defaults to
// enabled for the platform account (US-020) with the implicit
// everyone-participates empty list.
func rotationState(settings *domain.AutoAssignSettings) (enabled bool, participants []string) {
	if settings == nil {
		return true, nil
	}
	return settings.Enabled, settings.ParticipantUserIDs
}

// rowRotates mirrors AutoAssignSettingsUseCase.Update's eligibility:
// active members with role admin or attendant — exactly the pool
// GetActiveAttendants rotates over (US-021). inList nil or empty means
// the implicit everyone-participates default.
func rowRotates(r domain.OwnerSellerRow, inList map[string]struct{}) bool {
	if !r.IsActive || r.Role == nil || (*r.Role != "admin" && *r.Role != "attendant") {
		return false
	}
	if len(inList) == 0 {
		return true
	}
	_, ok := inList[r.UserID]
	return ok
}

// audit lands on the platform account with the actor email in the
// payload (audit_logs.account_id is NOT NULL + FK, actor_id is a
// users(id) FK — the owner-panel pattern from ComputeCommissionsUseCase).
func (uc *OwnerSellersUseCase) audit(ctx context.Context, actorEmail, action, entityID string, isSeller bool, role *string) {
	if uc.auditLog == nil {
		return
	}
	payload := map[string]any{"actor": actorEmail, "is_seller": isSeller}
	if role != nil {
		payload["target_role"] = *role
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = uc.auditLog.Create(ctx, &domain.AuditLog{
		ID:         ulid.Make().String(),
		AccountID:  uc.platformAccountID,
		Action:     action,
		EntityType: "commission",
		EntityID:   entityID,
		Payload:    raw,
		CreatedAt:  time.Now().UTC(),
	})
}
