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

type MockAutoAssignSettingsRepo struct{ mock.Mock }

func (m *MockAutoAssignSettingsRepo) GetAutoAssignSettings(ctx context.Context, accountID string) (*domain.AutoAssignSettings, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AutoAssignSettings), args.Error(1)
}

func (m *MockAutoAssignSettingsRepo) UpsertAutoAssignSettings(ctx context.Context, settings *domain.AutoAssignSettings) error {
	return m.Called(ctx, settings).Error(0)
}

func activeMember(id string) *domain.User {
	role := "attendant"
	return &domain.User{ID: id, Role: &role, IsActive: true}
}

// --- AutoAssignSettingsUseCase ---

// US-020: with no persisted row, only the platform account reports the
// historical enabled default — customer accounts report disabled so the
// settings screen matches what the assigner actually does.
func TestAutoAssignSettings_GetDefaultWhenNoRow(t *testing.T) {
	ctx := context.Background()
	repo := new(MockAutoAssignSettingsRepo)
	repo.On("GetAutoAssignSettings", ctx, mock.Anything).Return(nil, nil)

	uc := NewAutoAssignSettingsUseCase(repo, new(MockUserRepo))
	uc.SetPlatformAccountID("acc-platform")

	view, err := uc.Get(ctx, "acc-1")
	assert.NoError(t, err)
	assert.False(t, view.Enabled, "customer account must default to disabled (US-020)")
	assert.Empty(t, view.ParticipantUserIDs)
	assert.Nil(t, view.UpdatedAt)

	view, err = uc.Get(ctx, "acc-platform")
	assert.NoError(t, err)
	assert.True(t, view.Enabled, "platform account keeps the historical enabled default")
}

// Unwired platform id (env absent) fails toward disabled for everyone.
func TestAutoAssignSettings_GetDefaultNoPlatformConfigured(t *testing.T) {
	ctx := context.Background()
	repo := new(MockAutoAssignSettingsRepo)
	repo.On("GetAutoAssignSettings", ctx, "acc-1").Return(nil, nil)

	uc := NewAutoAssignSettingsUseCase(repo, new(MockUserRepo))
	view, err := uc.Get(ctx, "acc-1")

	assert.NoError(t, err)
	assert.False(t, view.Enabled)
}

func TestAutoAssignSettings_GetPersisted(t *testing.T) {
	ctx := context.Background()
	repo := new(MockAutoAssignSettingsRepo)
	now := time.Now()
	repo.On("GetAutoAssignSettings", ctx, "acc-1").Return(&domain.AutoAssignSettings{
		AccountID:          "acc-1",
		Enabled:            false,
		ParticipantUserIDs: []string{"u1"},
		UpdatedAt:          now,
	}, nil)

	uc := NewAutoAssignSettingsUseCase(repo, new(MockUserRepo))
	view, err := uc.Get(ctx, "acc-1")

	assert.NoError(t, err)
	assert.False(t, view.Enabled)
	assert.Equal(t, []string{"u1"}, view.ParticipantUserIDs)
	assert.NotNil(t, view.UpdatedAt)
}

func TestAutoAssignSettings_UpdateValidatesParticipants(t *testing.T) {
	ctx := context.Background()
	repo := new(MockAutoAssignSettingsRepo)
	userRepo := new(MockUserRepo)
	userRepo.On("ListByAccountID", ctx, "acc-1").Return([]*domain.User{activeMember("u1"), activeMember("u2")}, nil)

	uc := NewAutoAssignSettingsUseCase(repo, userRepo)
	_, err := uc.Update(ctx, "acc-1", true, []string{"u1", "stranger"})

	assert.ErrorIs(t, err, domain.ErrInvalidAutoAssignParticipant)
	repo.AssertNotCalled(t, "UpsertAutoAssignSettings", mock.Anything, mock.Anything)
}

// US-021: the PUT validation pool must equal the rotation pool
// (GetActiveAttendants: role admin/attendant). An active super_admin
// passed the old is-active-only check but never received a
// conversation — a silently unrotatable participant. FAILS on the
// pre-US-021 code.
func TestAutoAssignSettings_UpdateRejectsNonRotatableRole(t *testing.T) {
	ctx := context.Background()
	repo := new(MockAutoAssignSettingsRepo)
	userRepo := new(MockUserRepo)
	super := activeMember("u2")
	role := "super_admin"
	super.Role = &role
	noRole := activeMember("u3")
	noRole.Role = nil
	userRepo.On("ListByAccountID", ctx, "acc-1").Return([]*domain.User{activeMember("u1"), super, noRole}, nil)

	uc := NewAutoAssignSettingsUseCase(repo, userRepo)

	_, err := uc.Update(ctx, "acc-1", true, []string{"u2"})
	assert.ErrorIs(t, err, domain.ErrInvalidAutoAssignParticipant,
		"super_admin is outside GetActiveAttendants' pool and must not be saveable")

	_, err = uc.Update(ctx, "acc-1", true, []string{"u3"})
	assert.ErrorIs(t, err, domain.ErrInvalidAutoAssignParticipant,
		"a member with no role never rotates and must not be saveable")

	repo.AssertNotCalled(t, "UpsertAutoAssignSettings", mock.Anything, mock.Anything)
}

func TestAutoAssignSettings_UpdateRejectsInactiveMember(t *testing.T) {
	ctx := context.Background()
	repo := new(MockAutoAssignSettingsRepo)
	userRepo := new(MockUserRepo)
	inactive := activeMember("u2")
	inactive.IsActive = false
	userRepo.On("ListByAccountID", ctx, "acc-1").Return([]*domain.User{activeMember("u1"), inactive}, nil)

	uc := NewAutoAssignSettingsUseCase(repo, userRepo)
	_, err := uc.Update(ctx, "acc-1", true, []string{"u2"})

	assert.ErrorIs(t, err, domain.ErrInvalidAutoAssignParticipant)
}

func TestAutoAssignSettings_UpdateUpsertsAndDedupes(t *testing.T) {
	ctx := context.Background()
	repo := new(MockAutoAssignSettingsRepo)
	userRepo := new(MockUserRepo)
	userRepo.On("ListByAccountID", ctx, "acc-1").Return([]*domain.User{activeMember("u1"), activeMember("u2")}, nil)

	var saved *domain.AutoAssignSettings
	repo.On("UpsertAutoAssignSettings", ctx, mock.Anything).Run(func(args mock.Arguments) {
		saved = args.Get(1).(*domain.AutoAssignSettings)
	}).Return(nil)
	repo.On("GetAutoAssignSettings", ctx, "acc-1").Return(&domain.AutoAssignSettings{
		AccountID: "acc-1", Enabled: true, ParticipantUserIDs: []string{"u1", "u2"}, UpdatedAt: time.Now(),
	}, nil)

	uc := NewAutoAssignSettingsUseCase(repo, userRepo)
	view, err := uc.Update(ctx, "acc-1", true, []string{"u1", "u1", "", "u2"})

	assert.NoError(t, err)
	assert.Equal(t, []string{"u1", "u2"}, saved.ParticipantUserIDs)
	assert.True(t, view.Enabled)
}

func TestAutoAssignSettings_UpdateEmptyListSkipsValidation(t *testing.T) {
	ctx := context.Background()
	repo := new(MockAutoAssignSettingsRepo)
	userRepo := new(MockUserRepo) // no expectations — must not be called

	repo.On("UpsertAutoAssignSettings", ctx, mock.Anything).Return(nil)
	repo.On("GetAutoAssignSettings", ctx, "acc-1").Return(&domain.AutoAssignSettings{
		AccountID: "acc-1", Enabled: false, ParticipantUserIDs: []string{}, UpdatedAt: time.Now(),
	}, nil)

	uc := NewAutoAssignSettingsUseCase(repo, userRepo)
	view, err := uc.Update(ctx, "acc-1", false, nil)

	assert.NoError(t, err)
	assert.False(t, view.Enabled)
	userRepo.AssertNotCalled(t, "ListByAccountID", mock.Anything, mock.Anything)
}

// --- AutoAssignUseCase with settings ---

func TestAutoAssign_DisabledBySettings(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)
	convRepo.On("GetActiveAttendants", ctx, "acc-1").Return([]string{"u1", "u2"}, nil)

	settingsRepo := new(MockAutoAssignSettingsRepo)
	settingsRepo.On("GetAutoAssignSettings", ctx, "acc-1").Return(&domain.AutoAssignSettings{
		AccountID: "acc-1", Enabled: false,
	}, nil)

	store := newFakeRotationStore()
	uc := NewAutoAssignUseCase(convRepo, store)
	uc.SetSettingsRepo(settingsRepo)

	assignee, err := uc.Assign(ctx, "conv-1", "acc-1")
	assert.NoError(t, err)
	assert.Nil(t, assignee)
	assert.Zero(t, store.calls)
}

func TestAutoAssign_ParticipantsFilterRotation(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)
	convRepo.On("GetActiveAttendants", ctx, "acc-1").Return([]string{"u1", "u2", "u3"}, nil)

	settingsRepo := new(MockAutoAssignSettingsRepo)
	settingsRepo.On("GetAutoAssignSettings", ctx, "acc-1").Return(&domain.AutoAssignSettings{
		AccountID: "acc-1", Enabled: true, ParticipantUserIDs: []string{"u3", "u1"},
	}, nil)

	uc := NewAutoAssignUseCase(convRepo, newFakeRotationStore())
	uc.SetSettingsRepo(settingsRepo)

	// Rotation only over u1/u3 (attendant order preserved), u2 skipped.
	a, _ := uc.Assign(ctx, "conv-1", "acc-1")
	assert.Equal(t, "u1", *a)
	b, _ := uc.Assign(ctx, "conv-2", "acc-1")
	assert.Equal(t, "u3", *b)
	c, _ := uc.Assign(ctx, "conv-3", "acc-1")
	assert.Equal(t, "u1", *c)
}

func TestAutoAssign_ParticipantsAllStale(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)
	convRepo.On("GetActiveAttendants", ctx, "acc-1").Return([]string{"u1", "u2"}, nil)

	settingsRepo := new(MockAutoAssignSettingsRepo)
	settingsRepo.On("GetAutoAssignSettings", ctx, "acc-1").Return(&domain.AutoAssignSettings{
		AccountID: "acc-1", Enabled: true, ParticipantUserIDs: []string{"gone-user"},
	}, nil)

	uc := NewAutoAssignUseCase(convRepo, newFakeRotationStore())
	uc.SetSettingsRepo(settingsRepo)

	assignee, err := uc.Assign(ctx, "conv-1", "acc-1")
	assert.NoError(t, err)
	assert.Nil(t, assignee)
}

// US-020 flipped this from fail-OPEN (the old test pinned the bug: a
// settings read error re-enabled auto-assign with the defaults) to
// fail-CLOSED — an error skips the assignment.
func TestAutoAssign_SettingsErrorFailsClosed(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)
	convRepo.On("GetActiveAttendants", ctx, "acc-1").Return([]string{"u1"}, nil)

	settingsRepo := new(MockAutoAssignSettingsRepo)
	settingsRepo.On("GetAutoAssignSettings", ctx, "acc-1").Return(nil, errors.New("db down"))

	store := newFakeRotationStore()
	uc := NewAutoAssignUseCase(convRepo, store)
	uc.SetSettingsRepo(settingsRepo)
	uc.SetPlatformAccountID("acc-1")

	assignee, err := uc.Assign(ctx, "conv-1", "acc-1")
	assert.NoError(t, err)
	assert.Nil(t, assignee)
	assert.Zero(t, store.calls)
}

// No persisted row on the PLATFORM account keeps the historical
// default-on rotation (US-020 narrowed this default to platform-only;
// the customer-account case lives in auto_assign_test.go).
func TestAutoAssign_NoRowKeepsDefaultBehavior(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)
	convRepo.On("GetActiveAttendants", ctx, "acc-1").Return([]string{"u1", "u2"}, nil)

	settingsRepo := new(MockAutoAssignSettingsRepo)
	settingsRepo.On("GetAutoAssignSettings", ctx, "acc-1").Return(nil, nil)

	uc := NewAutoAssignUseCase(convRepo, newFakeRotationStore())
	uc.SetSettingsRepo(settingsRepo)
	uc.SetPlatformAccountID("acc-1")

	a, _ := uc.Assign(ctx, "conv-1", "acc-1")
	assert.Equal(t, "u1", *a)
	b, _ := uc.Assign(ctx, "conv-2", "acc-1")
	assert.Equal(t, "u2", *b)
}
