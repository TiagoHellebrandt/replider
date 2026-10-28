package usecases

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// --- fakes -----------------------------------------------------------------

type fakeReactivationStore struct {
	due     []domain.ReactivationAccount
	listErr error

	mu     sync.Mutex
	marked map[string]int // accountID -> stage
}

func (f *fakeReactivationStore) ListLapsedTrials(_ context.Context, _ time.Time) ([]domain.ReactivationAccount, error) {
	return f.due, f.listErr
}

func (f *fakeReactivationStore) MarkReactivationSent(_ context.Context, accountID string, stage int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.marked == nil {
		f.marked = map[string]int{}
	}
	f.marked[accountID] = stage
	return nil
}

type fakeReactivationNotifier struct {
	mu   sync.Mutex
	sent map[string]int // recipient -> daysSinceTrial passed
	err  error
}

func (f *fakeReactivationNotifier) SendReactivationEmail(to, _ string, daysSinceTrial int, _, _ string) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sent == nil {
		f.sent = map[string]int{}
	}
	f.sent[to] = daysSinceTrial
	return nil
}

// fakeAdminResolver is reused from send_weekly_digest_test.go (same package).

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// --- tests -----------------------------------------------------------------

func TestSendReactivation_SendsDueStageAndMarks(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeReactivationStore{
		due: []domain.ReactivationAccount{
			// 8 days lapsed, nothing sent -> day-7 nudge due.
			{AccountID: "acc1", Name: "A", TrialExpiresAt: now.Add(-8 * 24 * time.Hour), StageSent: 0},
			// 15 days lapsed, day-7 already sent -> day-14 nudge due.
			{AccountID: "acc2", Name: "B", TrialExpiresAt: now.Add(-15 * 24 * time.Hour), StageSent: 7},
		},
	}
	admins := &fakeAdminResolver{admins: map[string]*domain.User{
		"acc1": {Email: "a@x.com", Name: "Ana"},
		"acc2": {Email: "b@x.com", Name: "Bob"},
	}}
	notifier := &fakeReactivationNotifier{}

	uc := NewSendReactivationEmailsUseCase(store, admins, notifier, "https://app.replider.com").WithClock(fixedClock(now))
	sent, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 2 {
		t.Fatalf("expected 2 sent, got %d", sent)
	}
	if store.marked["acc1"] != 7 {
		t.Errorf("acc1 should be marked at stage 7, got %d", store.marked["acc1"])
	}
	if store.marked["acc2"] != 14 {
		t.Errorf("acc2 should be marked at stage 14, got %d", store.marked["acc2"])
	}
	if notifier.sent["a@x.com"] != 8 {
		t.Errorf("acc1 email should report 8 days since trial, got %d", notifier.sent["a@x.com"])
	}
}

func TestSendReactivation_SkipsWhenNoMilestoneDue(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeReactivationStore{
		due: []domain.ReactivationAccount{
			// Only 3 days lapsed — no milestone elapsed yet.
			{AccountID: "acc1", Name: "A", TrialExpiresAt: now.Add(-3 * 24 * time.Hour), StageSent: 0},
			// 10 days lapsed but day-7 already sent; day-14 not yet reached.
			{AccountID: "acc2", Name: "B", TrialExpiresAt: now.Add(-10 * 24 * time.Hour), StageSent: 7},
		},
	}
	admins := &fakeAdminResolver{admins: map[string]*domain.User{
		"acc1": {Email: "a@x.com", Name: "Ana"},
		"acc2": {Email: "b@x.com", Name: "Bob"},
	}}
	notifier := &fakeReactivationNotifier{}

	uc := NewSendReactivationEmailsUseCase(store, admins, notifier, "https://app.replider.com").WithClock(fixedClock(now))
	sent, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 0 {
		t.Fatalf("expected 0 sent, got %d", sent)
	}
	if len(store.marked) != 0 {
		t.Errorf("nothing should be marked, got %v", store.marked)
	}
}

func TestSendReactivation_NoAdminNotMarked(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeReactivationStore{
		due: []domain.ReactivationAccount{
			{AccountID: "acc1", Name: "A", TrialExpiresAt: now.Add(-8 * 24 * time.Hour), StageSent: 0},
		},
	}
	admins := &fakeAdminResolver{admins: map[string]*domain.User{}} // no admin
	notifier := &fakeReactivationNotifier{}

	uc := NewSendReactivationEmailsUseCase(store, admins, notifier, "https://app.replider.com").WithClock(fixedClock(now))
	sent, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 0 {
		t.Fatalf("expected 0 sent (no admin), got %d", sent)
	}
	if len(store.marked) != 0 {
		t.Errorf("account with no admin must not be marked (retry next tick), got %v", store.marked)
	}
}

func TestSendReactivation_SendErrorNotMarked(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeReactivationStore{
		due: []domain.ReactivationAccount{
			{AccountID: "acc1", Name: "A", TrialExpiresAt: now.Add(-30 * 24 * time.Hour), StageSent: 14},
		},
	}
	admins := &fakeAdminResolver{admins: map[string]*domain.User{
		"acc1": {Email: "a@x.com", Name: "Ana"},
	}}
	notifier := &fakeReactivationNotifier{err: errors.New("smtp down")}

	uc := NewSendReactivationEmailsUseCase(store, admins, notifier, "https://app.replider.com").WithClock(fixedClock(now))
	sent, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 0 {
		t.Fatalf("expected 0 sent (send failed), got %d", sent)
	}
	if len(store.marked) != 0 {
		t.Errorf("failed send must not be marked, got %v", store.marked)
	}
}

func TestSendReactivation_ListErrorPropagates(t *testing.T) {
	store := &fakeReactivationStore{listErr: errors.New("db down")}
	uc := NewSendReactivationEmailsUseCase(store, &fakeAdminResolver{}, &fakeReactivationNotifier{}, "https://app.replider.com")
	if _, err := uc.Execute(context.Background()); err == nil {
		t.Fatal("expected list error to propagate")
	}
}

func TestNextReactivationStage(t *testing.T) {
	cases := []struct {
		daysSince, stageSent, want int
	}{
		{6, 0, 0},   // nothing elapsed
		{7, 0, 7},   // exactly day 7
		{13, 7, 0},  // day-7 sent, day-14 not reached
		{14, 7, 14}, // day 14 due
		{20, 0, 14}, // backlog collapses to latest elapsed (14), not 7
		{31, 14, 30},
		{40, 30, 0}, // all sent
	}
	for _, c := range cases {
		if got := domain.NextReactivationStage(c.daysSince, c.stageSent); got != c.want {
			t.Errorf("NextReactivationStage(%d,%d)=%d want %d", c.daysSince, c.stageSent, got, c.want)
		}
	}
}
