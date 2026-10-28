package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetLiveSupervision_AdminSuccess(t *testing.T) {
	ctx := context.Background()
	adminRole := "admin"

	userRepo := new(MockUserRepo)
	userRepo.On("GetByID", ctx, "admin-1").Return(&domain.User{
		ID: "admin-1", AccountID: "a1", Role: &adminRole,
	}, nil)

	now := time.Now()
	analyticsRepo := new(MockAnalyticsRepo)
	analyticsRepo.On("GetAttendantLiveSnapshot", ctx, "a1").Return([]domain.AttendantLiveRow{
		{UserID: "u1", Name: "Ana", OpenConvs: 3, OldestWaitingSince: &now},
		{UserID: "u2", Name: "Bruno", OpenConvs: 0},
	}, nil)
	analyticsRepo.On("GetLiveQueueSnapshot", ctx, "a1").Return(&domain.LiveQueue{
		UnassignedCount: 5, OldestWaitingSince: &now,
	}, nil)

	uc := NewGetLiveSupervisionUseCase(analyticsRepo, userRepo)
	snap, err := uc.Execute(ctx, "a1", "admin-1")

	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Attendants, 2)
	assert.Equal(t, int64(3), snap.Attendants[0].OpenConvs)
	assert.Equal(t, int64(5), snap.Queue.UnassignedCount)
}

func TestGetLiveSupervision_AttendantBlocked(t *testing.T) {
	ctx := context.Background()
	attendantRole := "attendant"

	userRepo := new(MockUserRepo)
	userRepo.On("GetByID", ctx, "u1").Return(&domain.User{
		ID: "u1", AccountID: "a1", Role: &attendantRole,
	}, nil)

	analyticsRepo := new(MockAnalyticsRepo)

	uc := NewGetLiveSupervisionUseCase(analyticsRepo, userRepo)
	snap, err := uc.Execute(ctx, "a1", "u1")

	// 403, não 401: a credencial está boa, o papel é que não alcança. Com
	// 401 o app entende "sua sessão morreu" e apaga a sessão inteira.
	assert.Equal(t, domain.ErrForbidden, err)
	assert.Nil(t, snap)
	analyticsRepo.AssertNotCalled(t, "GetAttendantLiveSnapshot")
	analyticsRepo.AssertNotCalled(t, "GetLiveQueueSnapshot")
}

func TestGetLiveSupervision_QueueErrorDegradesGracefully(t *testing.T) {
	ctx := context.Background()
	adminRole := "admin"

	userRepo := new(MockUserRepo)
	userRepo.On("GetByID", ctx, "admin-1").Return(&domain.User{
		ID: "admin-1", AccountID: "a1", Role: &adminRole,
	}, nil)

	analyticsRepo := new(MockAnalyticsRepo)
	analyticsRepo.On("GetAttendantLiveSnapshot", ctx, "a1").Return([]domain.AttendantLiveRow{
		{UserID: "u1", Name: "Ana", OpenConvs: 1},
	}, nil)
	// Queue call blows up — snapshot should still return with zeroed
	// queue rather than failing the whole panel.
	analyticsRepo.On("GetLiveQueueSnapshot", ctx, "a1").Return((*domain.LiveQueue)(nil), errors.New("db transient"))

	uc := NewGetLiveSupervisionUseCase(analyticsRepo, userRepo)
	snap, err := uc.Execute(ctx, "a1", "admin-1")

	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Attendants, 1)
	assert.Equal(t, int64(0), snap.Queue.UnassignedCount)
	assert.Nil(t, snap.Queue.OldestWaitingSince)
}

func TestGetLiveSupervision_BlocksCrossAccount(t *testing.T) {
	ctx := context.Background()
	adminRole := "admin"

	userRepo := new(MockUserRepo)
	userRepo.On("GetByID", ctx, "admin-other").Return(&domain.User{
		ID: "admin-other", AccountID: "other-acc", Role: &adminRole,
	}, nil)

	analyticsRepo := new(MockAnalyticsRepo)

	uc := NewGetLiveSupervisionUseCase(analyticsRepo, userRepo)
	_, err := uc.Execute(ctx, "a1", "admin-other")
	assert.Equal(t, domain.ErrForbidden, err)
	// Nothing should be called once the role check fails.
	_ = mock.Anything
}

// O dono da conta é quem mais precisa deste painel, e era o único que não
// entrava: a checagem comparava o papel por igualdade de string, e "owner" não
// é literalmente "admin". Como a recusa saía como 401, o app apagava a sessão
// e devolvia a pessoa ao login — que a trazia de volta para cá, para ser
// expulsa de novo.
func TestGetLiveSupervision_OwnerIsAdminEnough(t *testing.T) {
	ctx := context.Background()
	ownerRole := "owner"

	userRepo := new(MockUserRepo)
	userRepo.On("GetByID", ctx, "owner-1").Return(&domain.User{
		ID: "owner-1", AccountID: "a1", Role: &ownerRole,
	}, nil)

	analyticsRepo := new(MockAnalyticsRepo)
	analyticsRepo.On("GetAttendantLiveSnapshot", ctx, "a1").Return([]domain.AttendantLiveRow{
		{UserID: "u1", Name: "Ana", OpenConvs: 2},
	}, nil)
	analyticsRepo.On("GetLiveQueueSnapshot", ctx, "a1").Return(&domain.LiveQueue{
		UnassignedCount: 1,
	}, nil)

	uc := NewGetLiveSupervisionUseCase(analyticsRepo, userRepo)
	snap, err := uc.Execute(ctx, "a1", "owner-1")

	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Attendants, 1)
}

// Um gestor vê o time inteiro em toda parte, menos aqui — o que fazia o painel
// discordar do resto do produto sobre quem supervisiona.
func TestGetLiveSupervision_RoleTableIsTheOnlyRule(t *testing.T) {
	ctx := context.Background()
	for role, allowed := range map[string]bool{
		"owner":     true,
		"admin":     true,
		"gestor":    false,
		"vendedor":  false,
		"attendant": false,
	} {
		r := role
		userRepo := new(MockUserRepo)
		userRepo.On("GetByID", ctx, "u1").Return(&domain.User{
			ID: "u1", AccountID: "a1", Role: &r,
		}, nil)

		analyticsRepo := new(MockAnalyticsRepo)
		analyticsRepo.On("GetAttendantLiveSnapshot", ctx, "a1").
			Return([]domain.AttendantLiveRow{}, nil)
		analyticsRepo.On("GetLiveQueueSnapshot", ctx, "a1").
			Return(&domain.LiveQueue{}, nil)

		_, err := NewGetLiveSupervisionUseCase(analyticsRepo, userRepo).
			Execute(ctx, "a1", "u1")

		if allowed {
			assert.NoError(t, err, role)
		} else {
			assert.Equal(t, domain.ErrForbidden, err, role)
		}
	}
}
