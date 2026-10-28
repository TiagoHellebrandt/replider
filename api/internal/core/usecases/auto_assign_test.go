package usecases

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/replider/api/internal/core/domain"
)

// fakeRotationStore mimics the SQL semantics of
// postgres.AutoAssignRotationRepository in memory: the next pick is the
// smallest pool id greater than the persisted cursor (wrapping to the
// smallest), and claim + assignment are atomic — any failure leaves the
// cursor untouched. Sharing one store across two use-case instances
// models a deploy/restart (the cursor lives in the DB, not the
// process).
type fakeRotationStore struct {
	mu       sync.Mutex
	cursor   map[string]string
	assigned map[string]string // conversation_id -> assignee
	missing  map[string]bool   // conversations that "don't exist" → ErrNotFound, no advance
	failWith error             // forced failure → error, no advance
	calls    int
}

func newFakeRotationStore() *fakeRotationStore {
	return &fakeRotationStore{
		cursor:   map[string]string{},
		assigned: map[string]string{},
		missing:  map[string]bool{},
	}
}

func (f *fakeRotationStore) AssignNextRoundRobin(_ context.Context, conversationID, accountID string, pool []string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.failWith != nil {
		return "", f.failWith
	}
	if f.missing[conversationID] {
		return "", domain.ErrConversationNotFound
	}
	sorted := append([]string(nil), pool...)
	sort.Strings(sorted)
	next := ""
	if cur, ok := f.cursor[accountID]; ok {
		for _, id := range sorted {
			if id > cur {
				next = id
				break
			}
		}
	}
	if next == "" {
		next = sorted[0]
	}
	f.cursor[accountID] = next
	f.assigned[conversationID] = next
	return next, nil
}

func TestAutoAssign_RoundRobin(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)

	attendants := []string{"user-1", "user-2", "user-3"}
	convRepo.On("GetActiveAttendants", ctx, "acc-1").Return(attendants, nil)

	store := newFakeRotationStore()
	uc := NewAutoAssignUseCase(convRepo, store)
	uc.SetPlatformAccountID("acc-1")

	// First assignment → user-1
	assignee, err := uc.Assign(ctx, "conv-1", "acc-1")
	assert.NoError(t, err)
	assert.Equal(t, "user-1", *assignee)

	// Second → user-2
	assignee, err = uc.Assign(ctx, "conv-2", "acc-1")
	assert.NoError(t, err)
	assert.Equal(t, "user-2", *assignee)

	// Third → user-3
	assignee, err = uc.Assign(ctx, "conv-3", "acc-1")
	assert.NoError(t, err)
	assert.Equal(t, "user-3", *assignee)

	// Fourth → wraps back to user-1
	assignee, err = uc.Assign(ctx, "conv-4", "acc-1")
	assert.NoError(t, err)
	assert.Equal(t, "user-1", *assignee)
}

// US-021: the cursor lives in the store, not the use-case instance — a
// new instance over the same store (i.e. a restarted process over the
// same DB) continues the rotation instead of resetting to the
// lowest-ULID attendant. FAILS on the pre-US-021 code (in-memory
// counters map reset to user-1).
func TestAutoAssign_US021_RotationSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)
	convRepo.On("GetActiveAttendants", ctx, "acc-1").Return([]string{"user-1", "user-2", "user-3"}, nil)

	store := newFakeRotationStore()

	uc1 := NewAutoAssignUseCase(convRepo, store)
	uc1.SetPlatformAccountID("acc-1")
	a, err := uc1.Assign(ctx, "conv-1", "acc-1")
	assert.NoError(t, err)
	assert.Equal(t, "user-1", *a)
	b, err := uc1.Assign(ctx, "conv-2", "acc-1")
	assert.NoError(t, err)
	assert.Equal(t, "user-2", *b)

	// "Deploy": a fresh use case over the same persisted cursor.
	uc2 := NewAutoAssignUseCase(convRepo, store)
	uc2.SetPlatformAccountID("acc-1")
	c, err := uc2.Assign(ctx, "conv-3", "acc-1")
	assert.NoError(t, err)
	assert.Equal(t, "user-3", *c, "restart must continue the rotation, not reset to the lowest ULID")
}

// US-021: a failed assignment surfaces the error and does NOT advance
// the cursor — the skipped attendant keeps their turn.
func TestAutoAssign_US021_ErrorDoesNotAdvanceCursor(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)
	convRepo.On("GetActiveAttendants", ctx, "acc-1").Return([]string{"user-1", "user-2"}, nil)

	store := newFakeRotationStore()
	uc := NewAutoAssignUseCase(convRepo, store)
	uc.SetPlatformAccountID("acc-1")

	a, err := uc.Assign(ctx, "conv-1", "acc-1")
	assert.NoError(t, err)
	assert.Equal(t, "user-1", *a)

	store.failWith = errors.New("assign failed")
	assignee, err := uc.Assign(ctx, "conv-2", "acc-1")
	assert.Error(t, err)
	assert.Nil(t, assignee)

	store.failWith = nil
	b, err := uc.Assign(ctx, "conv-3", "acc-1")
	assert.NoError(t, err)
	assert.Equal(t, "user-2", *b, "the failed turn must not be skipped")
}

func TestAutoAssign_NoAttendants(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)

	convRepo.On("GetActiveAttendants", ctx, "acc-1").Return([]string{}, nil)

	store := newFakeRotationStore()
	uc := NewAutoAssignUseCase(convRepo, store)
	uc.SetPlatformAccountID("acc-1")
	assignee, err := uc.Assign(ctx, "conv-1", "acc-1")

	assert.NoError(t, err)
	assert.Nil(t, assignee)
	assert.Zero(t, store.calls, "empty pool must never reach the rotation store")
}

func TestAutoAssign_GetAttendantsError(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)

	convRepo.On("GetActiveAttendants", ctx, "acc-1").Return(nil, errors.New("db error"))

	uc := NewAutoAssignUseCase(convRepo, newFakeRotationStore())
	uc.SetPlatformAccountID("acc-1")
	assignee, err := uc.Assign(ctx, "conv-1", "acc-1")

	assert.Error(t, err)
	assert.Nil(t, assignee)
	assert.Equal(t, "db error", err.Error())
}

func TestAutoAssign_SingleAttendant(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)

	convRepo.On("GetActiveAttendants", ctx, "acc-1").Return([]string{"solo-user"}, nil)

	uc := NewAutoAssignUseCase(convRepo, newFakeRotationStore())
	uc.SetPlatformAccountID("acc-1")

	// All assignments should go to the same user
	for i := 0; i < 5; i++ {
		assignee, err := uc.Assign(ctx, "conv-"+string(rune('1'+i)), "acc-1")
		assert.NoError(t, err)
		assert.Equal(t, "solo-user", *assignee)
	}
}

func TestAutoAssign_MultipleAccounts(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)

	convRepo.On("GetActiveAttendants", ctx, "acc-1").Return([]string{"a1-user1", "a1-user2"}, nil)
	convRepo.On("GetActiveAttendants", ctx, "acc-2").Return([]string{"a2-user1", "a2-user2"}, nil)

	// Both accounts explicitly opted in (US-020) so each keeps its own
	// independent rotation cursor.
	settingsRepo := new(MockAutoAssignSettingsRepo)
	settingsRepo.On("GetAutoAssignSettings", ctx, mock.Anything).
		Return(&domain.AutoAssignSettings{Enabled: true}, nil)

	uc := NewAutoAssignUseCase(convRepo, newFakeRotationStore())
	uc.SetSettingsRepo(settingsRepo)

	// Account 1 gets user1
	a1, _ := uc.Assign(ctx, "conv-a1", "acc-1")
	assert.Equal(t, "a1-user1", *a1)

	// Account 2 gets user1 (independent cursor)
	a2, _ := uc.Assign(ctx, "conv-a2", "acc-2")
	assert.Equal(t, "a2-user1", *a2)

	// Account 1 gets user2 (continues its own cursor)
	a1b, _ := uc.Assign(ctx, "conv-a1b", "acc-1")
	assert.Equal(t, "a1-user2", *a1b)
}

func TestAutoAssign_ConcurrentSafety(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)

	convRepo.On("GetActiveAttendants", ctx, "acc-1").Return([]string{"u1", "u2", "u3"}, nil)

	uc := NewAutoAssignUseCase(convRepo, newFakeRotationStore())
	uc.SetPlatformAccountID("acc-1")

	var wg sync.WaitGroup
	results := make([]*string, 30)

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			assignee, _ := uc.Assign(ctx, "conv-"+string(rune(idx)), "acc-1")
			results[idx] = assignee
		}(i)
	}
	wg.Wait()

	// All results should be non-nil (no panics or race conditions)
	for i, r := range results {
		assert.NotNil(t, r, "result %d should not be nil", i)
	}
}

// --- US-020: auto-assign default-on is gated to the platform account ---

// A customer account with NO persisted settings row must NOT be
// auto-assigned — the always-on default silently changed existing
// customer inboxes. FAILS on the pre-US-020 code (it assigned).
func TestAutoAssign_US020_CustomerAccountDefaultOff(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)
	settingsRepo := new(MockAutoAssignSettingsRepo)
	settingsRepo.On("GetAutoAssignSettings", ctx, "acc-customer").Return(nil, nil)

	store := newFakeRotationStore()
	uc := NewAutoAssignUseCase(convRepo, store)
	uc.SetSettingsRepo(settingsRepo)
	uc.SetPlatformAccountID("acc-platform")

	assignee, err := uc.Assign(ctx, "conv-1", "acc-customer")

	assert.NoError(t, err)
	assert.Nil(t, assignee)
	assert.Zero(t, store.calls)
	convRepo.AssertNotCalled(t, "GetActiveAttendants", mock.Anything, mock.Anything)
}

// The platform account keeps the historical default-on behavior with no row.
func TestAutoAssign_US020_PlatformAccountDefaultOn(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)
	settingsRepo := new(MockAutoAssignSettingsRepo)
	settingsRepo.On("GetAutoAssignSettings", ctx, "acc-platform").Return(nil, nil)
	convRepo.On("GetActiveAttendants", ctx, "acc-platform").Return([]string{"user-1"}, nil)

	uc := NewAutoAssignUseCase(convRepo, newFakeRotationStore())
	uc.SetSettingsRepo(settingsRepo)
	uc.SetPlatformAccountID("acc-platform")

	assignee, err := uc.Assign(ctx, "conv-1", "acc-platform")

	assert.NoError(t, err)
	assert.Equal(t, "user-1", *assignee)
}

// A customer account that explicitly opted in (persisted Enabled=true
// row) is assigned normally.
func TestAutoAssign_US020_CustomerExplicitOptIn(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)
	settingsRepo := new(MockAutoAssignSettingsRepo)
	settingsRepo.On("GetAutoAssignSettings", ctx, "acc-customer").
		Return(&domain.AutoAssignSettings{AccountID: "acc-customer", Enabled: true}, nil)
	convRepo.On("GetActiveAttendants", ctx, "acc-customer").Return([]string{"user-1"}, nil)

	uc := NewAutoAssignUseCase(convRepo, newFakeRotationStore())
	uc.SetSettingsRepo(settingsRepo)
	uc.SetPlatformAccountID("acc-platform")

	assignee, err := uc.Assign(ctx, "conv-1", "acc-customer")

	assert.NoError(t, err)
	assert.Equal(t, "user-1", *assignee)
}

// A settings read error fails CLOSED — no assignment. The pre-US-020
// code failed OPEN (warned and assigned with the defaults), silently
// re-enabling the feature for accounts that turned it off. FAILS on
// the old code.
func TestAutoAssign_US020_SettingsErrorFailsClosed(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)
	settingsRepo := new(MockAutoAssignSettingsRepo)
	settingsRepo.On("GetAutoAssignSettings", ctx, "acc-platform").Return(nil, errors.New("db down"))
	convRepo.On("GetActiveAttendants", ctx, "acc-platform").Return([]string{"user-1"}, nil)

	store := newFakeRotationStore()
	uc := NewAutoAssignUseCase(convRepo, store)
	uc.SetSettingsRepo(settingsRepo)
	uc.SetPlatformAccountID("acc-platform")

	assignee, err := uc.Assign(ctx, "conv-1", "acc-platform")

	assert.NoError(t, err)
	assert.Nil(t, assignee)
	assert.Zero(t, store.calls)
}

// Enabled with a participant list that intersects to zero active
// attendants is handled explicitly: no assignment, no error, no
// rotation advance.
func TestAutoAssign_US020_EnabledWithoutEligibleParticipants(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)
	settingsRepo := new(MockAutoAssignSettingsRepo)
	settingsRepo.On("GetAutoAssignSettings", ctx, "acc-1").
		Return(&domain.AutoAssignSettings{AccountID: "acc-1", Enabled: true, ParticipantUserIDs: []string{"ghost-user"}}, nil)
	convRepo.On("GetActiveAttendants", ctx, "acc-1").Return([]string{"user-1", "user-2"}, nil)

	store := newFakeRotationStore()
	uc := NewAutoAssignUseCase(convRepo, store)
	uc.SetSettingsRepo(settingsRepo)

	assignee, err := uc.Assign(ctx, "conv-1", "acc-1")

	assert.NoError(t, err)
	assert.Nil(t, assignee)
	assert.Zero(t, store.calls)
}

// Disabled row on the PLATFORM account wins over the platform default —
// an explicit off is off everywhere.
func TestAutoAssign_US020_PlatformExplicitOff(t *testing.T) {
	ctx := context.Background()
	convRepo := new(MockConversationRepo)
	settingsRepo := new(MockAutoAssignSettingsRepo)
	settingsRepo.On("GetAutoAssignSettings", ctx, "acc-platform").
		Return(&domain.AutoAssignSettings{AccountID: "acc-platform", Enabled: false}, nil)

	uc := NewAutoAssignUseCase(convRepo, newFakeRotationStore())
	uc.SetSettingsRepo(settingsRepo)
	uc.SetPlatformAccountID("acc-platform")

	assignee, err := uc.Assign(ctx, "conv-1", "acc-platform")

	assert.NoError(t, err)
	assert.Nil(t, assignee)
	convRepo.AssertNotCalled(t, "GetActiveAttendants", mock.Anything, mock.Anything)
}
