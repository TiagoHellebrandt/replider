package usecases

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

// GetLiveSupervisionUseCase composes the two live-view queries
// (per-attendant load + unassigned queue summary) into a single snapshot
// the supervisor panel renders atomically. Admin-only: attendants don't
// need a birds-eye view of their peers and the data leaks anyway if an
// attendant can read other users' open_convs.
type GetLiveSupervisionUseCase struct {
	analyticsRepo domain.AnalyticsRepository
	userRepo      domain.UserRepository
}

func NewGetLiveSupervisionUseCase(
	analyticsRepo domain.AnalyticsRepository,
	userRepo domain.UserRepository,
) *GetLiveSupervisionUseCase {
	return &GetLiveSupervisionUseCase{
		analyticsRepo: analyticsRepo,
		userRepo:      userRepo,
	}
}

// Execute enforces admin-only + same-account scoping, then fans out to
// the two repo calls. Queue summary errors are non-fatal (logged then
// zeroed) — the attendants slice is the primary value of the panel and
// we'd rather ship it without the queue banner than show nothing.
func (uc *GetLiveSupervisionUseCase) Execute(
	ctx context.Context,
	accountID, callerID string,
) (*domain.LiveSupervisionSnapshot, error) {
	caller, err := uc.userRepo.GetByID(ctx, callerID)
	if err != nil {
		return nil, err
	}
	if caller == nil || caller.AccountID != accountID {
		return nil, domain.ErrForbidden
	}
	// RoleSatisfies, not string equality: an OWNER is not literally "admin" and
	// was left out of their own panel — the same regression the table in
	// domain/rbac.go exists to prevent.
	//
	// And the refusal is 403, never 401. This endpoint is polled every ten
	// seconds while the Team panel is open; answering 401 told the app "your
	// credential died", the app cleared the session, and the person landed on
	// login — from where `?from=` brought them back to the same screen, to be
	// thrown out again. An endpoint with the wrong status tore down the whole
	// session, in a loop.
	role := domain.NormalizeRole("")
	if caller.Role != nil {
		role = domain.NormalizeRole(*caller.Role)
	}
	if !domain.RoleSatisfies(role, domain.RoleAdminRole) {
		return nil, domain.ErrForbidden
	}

	attendants, err := uc.analyticsRepo.GetAttendantLiveSnapshot(ctx, accountID)
	if err != nil {
		return nil, err
	}
	queue, err := uc.analyticsRepo.GetLiveQueueSnapshot(ctx, accountID)
	if err != nil || queue == nil {
		// Degrade gracefully — render the attendant list without the
		// queue banner rather than 500 the whole panel.
		queue = &domain.LiveQueue{}
	}

	return &domain.LiveSupervisionSnapshot{
		Attendants: attendants,
		Queue:      *queue,
	}, nil
}
