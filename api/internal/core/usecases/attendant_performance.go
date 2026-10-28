package usecases

import (
	"context"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// AttendantPerformancePeriod is the fixed-window selector the Team
// screen exposes. Free-form date ranges were rejected during planning
// in favor of 4 chips — 80/20, simpler UX, cacheable later.
type AttendantPerformancePeriod string

const (
	PeriodToday   AttendantPerformancePeriod = "today"
	PeriodLast7d  AttendantPerformancePeriod = "7d"
	PeriodLast30d AttendantPerformancePeriod = "30d"
	PeriodMonthTD AttendantPerformancePeriod = "mtd"
	defaultPeriod                            = PeriodLast30d
)

// GetAttendantPerformanceUseCase fetches per-attendant metrics for the
// Settings → Time screen and the per-member drill-down. Role gating:
//   - admin: sees every attendant in the account (UserID nil → list),
//     or may drill into any single user.
//   - attendant: may only see themselves. Passing a different UserID
//     returns ErrUnauthorized so UI can't trivially leak a colleague's
//     numbers.
type GetAttendantPerformanceUseCase struct {
	analyticsRepo domain.AnalyticsRepository
	userRepo      domain.UserRepository
}

func NewGetAttendantPerformanceUseCase(
	analyticsRepo domain.AnalyticsRepository,
	userRepo domain.UserRepository,
) *GetAttendantPerformanceUseCase {
	return &GetAttendantPerformanceUseCase{
		analyticsRepo: analyticsRepo,
		userRepo:      userRepo,
	}
}

// GetAttendantPerformanceDTO is the input surface the handler binds.
// Period is one of the supported chips; unrecognised values fall back
// to PeriodLast30d. UserID is optional — nil means "every attendant".
type GetAttendantPerformanceDTO struct {
	Period AttendantPerformancePeriod
	UserID *string
}

// Execute resolves [from, to] from the period, enforces role/ownership,
// and hands off to the repository. Returns a stable ordering: assigned
// conversations desc, then name asc (ties broken deterministically).
func (uc *GetAttendantPerformanceUseCase) Execute(
	ctx context.Context,
	accountID string,
	callerID string,
	dto GetAttendantPerformanceDTO,
) ([]domain.AttendantPerformance, error) {
	caller, err := uc.userRepo.GetByID(ctx, callerID)
	if err != nil {
		return nil, err
	}
	if caller == nil || caller.AccountID != accountID {
		return nil, domain.ErrForbidden
	}

	// RoleSatisfies, not string equality: an OWNER is not literally "admin", and
	// was left out of what is theirs by definition. And the refusal is 403 — with
	// a 401 the app concludes the credential died and clears the session.
	isAdmin := domain.RoleSatisfies(roleOf(caller), domain.RoleAdminRole)

	// Attendants are limited to their own numbers. An attendant trying
	// to list everyone (UserID=nil) is silently scoped to themselves —
	// safer default than 403 because the UI may legitimately want a
	// "my metrics" view from the same endpoint.
	if !isAdmin {
		if dto.UserID != nil && *dto.UserID != callerID {
			return nil, domain.ErrForbidden
		}
		dto.UserID = &callerID
	}

	from, to := periodWindow(dto.Period, time.Now().UTC())
	return uc.analyticsRepo.GetAttendantPerformance(ctx, domain.AttendantPerformanceFilter{
		AccountID: accountID,
		From:      from,
		To:        to,
		UserID:    dto.UserID,
	})
}

// periodWindow maps a period chip to [from, to). now() is passed in so
// tests can pin the clock. Unknown periods degrade to 30d.
func periodWindow(p AttendantPerformancePeriod, now time.Time) (time.Time, time.Time) {
	to := now
	switch p {
	case PeriodToday:
		return startOfDay(now), to
	case PeriodLast7d:
		return now.AddDate(0, 0, -7), to
	case PeriodMonthTD:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()), to
	case PeriodLast30d:
		fallthrough
	default:
		return now.AddDate(0, 0, -30), to
	}
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
