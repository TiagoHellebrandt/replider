package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetAttendantPerformance_AdminSeesEveryone(t *testing.T) {
	ctx := context.Background()
	adminRole := "admin"

	userRepo := new(MockUserRepo)
	userRepo.On("GetByID", ctx, "admin-1").Return(&domain.User{
		ID: "admin-1", AccountID: "a1", Role: &adminRole,
	}, nil)

	analyticsRepo := new(MockAnalyticsRepo)
	analyticsRepo.On("GetAttendantPerformance", ctx, mock.MatchedBy(func(f domain.AttendantPerformanceFilter) bool {
		return f.AccountID == "a1" && f.UserID == nil
	})).Return([]domain.AttendantPerformance{
		{UserID: "u1", Name: "Ana"},
		{UserID: "u2", Name: "Bruno"},
	}, nil)

	uc := NewGetAttendantPerformanceUseCase(analyticsRepo, userRepo)
	rows, err := uc.Execute(ctx, "a1", "admin-1", GetAttendantPerformanceDTO{})

	assert.NoError(t, err)
	assert.Len(t, rows, 2)
	analyticsRepo.AssertExpectations(t)
}

func TestGetAttendantPerformance_AttendantScopedToSelf(t *testing.T) {
	ctx := context.Background()
	attendant := "attendant"

	userRepo := new(MockUserRepo)
	userRepo.On("GetByID", ctx, "u1").Return(&domain.User{
		ID: "u1", AccountID: "a1", Role: &attendant,
	}, nil)

	analyticsRepo := new(MockAnalyticsRepo)
	analyticsRepo.On("GetAttendantPerformance", ctx, mock.MatchedBy(func(f domain.AttendantPerformanceFilter) bool {
		return f.UserID != nil && *f.UserID == "u1"
	})).Return([]domain.AttendantPerformance{{UserID: "u1", Name: "Ana"}}, nil)

	uc := NewGetAttendantPerformanceUseCase(analyticsRepo, userRepo)
	// Caller passes UserID=nil; use case silently scopes to self.
	rows, err := uc.Execute(ctx, "a1", "u1", GetAttendantPerformanceDTO{})

	assert.NoError(t, err)
	assert.Len(t, rows, 1)
	analyticsRepo.AssertExpectations(t)
}

func TestGetAttendantPerformance_AttendantBlockedFromPeers(t *testing.T) {
	ctx := context.Background()
	attendant := "attendant"

	userRepo := new(MockUserRepo)
	userRepo.On("GetByID", ctx, "u1").Return(&domain.User{
		ID: "u1", AccountID: "a1", Role: &attendant,
	}, nil)

	analyticsRepo := new(MockAnalyticsRepo)

	peerID := "u2"
	uc := NewGetAttendantPerformanceUseCase(analyticsRepo, userRepo)
	rows, err := uc.Execute(ctx, "a1", "u1", GetAttendantPerformanceDTO{UserID: &peerID})

	assert.Equal(t, domain.ErrForbidden, err)
	assert.Nil(t, rows)
	analyticsRepo.AssertNotCalled(t, "GetAttendantPerformance")
}

func TestGetAttendantPerformance_BlocksUnknownUser(t *testing.T) {
	ctx := context.Background()

	userRepo := new(MockUserRepo)
	userRepo.On("GetByID", ctx, "ghost").Return(nil, nil)

	analyticsRepo := new(MockAnalyticsRepo)

	uc := NewGetAttendantPerformanceUseCase(analyticsRepo, userRepo)
	_, err := uc.Execute(ctx, "a1", "ghost", GetAttendantPerformanceDTO{})
	assert.Equal(t, domain.ErrForbidden, err)
}

func TestGetAttendantPerformance_BlocksCrossAccount(t *testing.T) {
	ctx := context.Background()
	adminRole := "admin"

	userRepo := new(MockUserRepo)
	userRepo.On("GetByID", ctx, "admin-other").Return(&domain.User{
		ID: "admin-other", AccountID: "other-acc", Role: &adminRole,
	}, nil)

	analyticsRepo := new(MockAnalyticsRepo)

	uc := NewGetAttendantPerformanceUseCase(analyticsRepo, userRepo)
	_, err := uc.Execute(ctx, "a1", "admin-other", GetAttendantPerformanceDTO{})
	assert.Equal(t, domain.ErrForbidden, err)
}

func TestPeriodWindow(t *testing.T) {
	now := time.Date(2026, 4, 18, 15, 30, 0, 0, time.UTC)

	from, to := periodWindow(PeriodToday, now)
	assert.Equal(t, time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC), from)
	assert.Equal(t, now, to)

	from, _ = periodWindow(PeriodLast7d, now)
	assert.Equal(t, now.AddDate(0, 0, -7), from)

	from, _ = periodWindow(PeriodMonthTD, now)
	assert.Equal(t, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), from)

	// Unknown period → default 30d.
	from, _ = periodWindow(AttendantPerformancePeriod("bogus"), now)
	assert.Equal(t, now.AddDate(0, 0, -30), from)
}
