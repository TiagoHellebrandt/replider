package usecases

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// --- fakes -----------------------------------------------------------------

type fakeWinBackStore struct {
	due []domain.WinBackAccount

	mu       sync.Mutex
	started  map[string]string // accountID -> couponCode
	marked   map[string]int    // accountID -> stage
	startErr error
}

func (f *fakeWinBackStore) StartWinBack(_ context.Context, accountID, couponCode string, _ time.Time) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.started == nil {
		f.started = map[string]string{}
	}
	f.started[accountID] = couponCode
	return nil
}

func (f *fakeWinBackStore) ListWinBackDue(_ context.Context) ([]domain.WinBackAccount, error) {
	return f.due, nil
}

func (f *fakeWinBackStore) MarkWinBackSent(_ context.Context, accountID string, stage int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.marked == nil {
		f.marked = map[string]int{}
	}
	f.marked[accountID] = stage
	return nil
}

type fakeWinBackIssuer struct {
	code      string
	err       error
	customers []string
}

func (f *fakeWinBackIssuer) CreateWinBackCoupon(customerID string) (string, error) {
	f.customers = append(f.customers, customerID)
	return f.code, f.err
}

type fakeWinBackNotifier struct {
	mu     sync.Mutex
	sent   map[string]int // recipient -> daysSince passed
	codes  map[string]string
	err    error
	called int
}

func (f *fakeWinBackNotifier) SendWinBackEmail(to, _ string, daysSince, _, _ int, couponCode, _, _ string) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.called++
	if f.sent == nil {
		f.sent = map[string]int{}
		f.codes = map[string]string{}
	}
	f.sent[to] = daysSince
	f.codes[to] = couponCode
	return nil
}

// fakeAdminResolver + fixedClock are reused from the reactivation/digest
// tests (same package).

// --- tests -----------------------------------------------------------------

func TestWinBack_Start_CreatesCouponPersistsAndEmails(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeWinBackStore{}
	issuer := &fakeWinBackIssuer{code: "WINBACKABC"}
	admins := &fakeAdminResolver{admins: map[string]*domain.User{
		"acc1": {Email: "a@x.com", Name: "Ana"},
	}}
	notifier := &fakeWinBackNotifier{}

	uc := NewWinBackUseCase(store, issuer, admins, notifier, "https://app.replider.com").WithClock(fixedClock(now))
	uc.Start(context.Background(), "acc1", "cus_123")

	if got := issuer.customers; len(got) != 1 || got[0] != "cus_123" {
		t.Fatalf("expected coupon minted for cus_123, got %v", got)
	}
	if store.started["acc1"] != "WINBACKABC" {
		t.Errorf("expected start persisted with coupon code, got %q", store.started["acc1"])
	}
	if notifier.sent["a@x.com"] != 0 {
		t.Errorf("expected day-0 offer email, got daysSince %d", notifier.sent["a@x.com"])
	}
	if notifier.codes["a@x.com"] != "WINBACKABC" {
		t.Errorf("expected coupon code in email, got %q", notifier.codes["a@x.com"])
	}
}

func TestWinBack_Start_NoCustomerIsNoop(t *testing.T) {
	store := &fakeWinBackStore{}
	issuer := &fakeWinBackIssuer{code: "X"}
	notifier := &fakeWinBackNotifier{}
	uc := NewWinBackUseCase(store, issuer, &fakeAdminResolver{}, notifier, "https://app.replider.com")

	uc.Start(context.Background(), "acc1", "")

	if len(issuer.customers) != 0 || notifier.called != 0 || len(store.started) != 0 {
		t.Fatalf("expected no-op when customer id is empty")
	}
}

func TestWinBack_Start_CouponFailureSkipsEmail(t *testing.T) {
	store := &fakeWinBackStore{}
	issuer := &fakeWinBackIssuer{err: context.DeadlineExceeded}
	notifier := &fakeWinBackNotifier{}
	uc := NewWinBackUseCase(store, issuer, &fakeAdminResolver{}, notifier, "https://app.replider.com")

	uc.Start(context.Background(), "acc1", "cus_123")

	if len(store.started) != 0 {
		t.Errorf("expected no start persisted when coupon mint fails")
	}
	if notifier.called != 0 {
		t.Errorf("expected no email when coupon mint fails")
	}
}

func TestWinBack_Execute_SendsDueFollowUpsAndMarks(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeWinBackStore{
		due: []domain.WinBackAccount{
			// 8 days since cancel, nothing followed up -> day-7 reminder due.
			{AccountID: "acc1", Name: "A", StartedAt: now.Add(-8 * 24 * time.Hour), CouponCode: "C1", StageSent: 0},
			// 31 days since cancel, day-7 already sent -> day-30 reminder due.
			{AccountID: "acc2", Name: "B", StartedAt: now.Add(-31 * 24 * time.Hour), CouponCode: "C2", StageSent: 7},
			// 3 days since cancel -> nothing due yet.
			{AccountID: "acc3", Name: "C", StartedAt: now.Add(-3 * 24 * time.Hour), CouponCode: "C3", StageSent: 0},
		},
	}
	admins := &fakeAdminResolver{admins: map[string]*domain.User{
		"acc1": {Email: "a@x.com", Name: "Ana"},
		"acc2": {Email: "b@x.com", Name: "Bob"},
		"acc3": {Email: "c@x.com", Name: "Cid"},
	}}
	notifier := &fakeWinBackNotifier{}

	uc := NewWinBackUseCase(store, &fakeWinBackIssuer{}, admins, notifier, "https://app.replider.com").WithClock(fixedClock(now))
	sent, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 2 {
		t.Fatalf("expected 2 follow-ups sent, got %d", sent)
	}
	if store.marked["acc1"] != 7 {
		t.Errorf("acc1 should be marked at stage 7, got %d", store.marked["acc1"])
	}
	if store.marked["acc2"] != 30 {
		t.Errorf("acc2 should be marked at stage 30, got %d", store.marked["acc2"])
	}
	if _, ok := store.marked["acc3"]; ok {
		t.Errorf("acc3 should not be marked (not yet due)")
	}
	if notifier.codes["b@x.com"] != "C2" {
		t.Errorf("acc2 follow-up should carry its coupon code C2, got %q", notifier.codes["b@x.com"])
	}
}

func TestWinBack_Execute_MissingAdminNotMarked(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeWinBackStore{
		due: []domain.WinBackAccount{
			{AccountID: "acc1", Name: "A", StartedAt: now.Add(-8 * 24 * time.Hour), CouponCode: "C1", StageSent: 0},
		},
	}
	// No admin resolvable for acc1.
	uc := NewWinBackUseCase(store, &fakeWinBackIssuer{}, &fakeAdminResolver{}, &fakeWinBackNotifier{}, "https://app.replider.com").WithClock(fixedClock(now))

	sent, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 0 {
		t.Fatalf("expected 0 sent when admin missing, got %d", sent)
	}
	if _, ok := store.marked["acc1"]; ok {
		t.Errorf("acc1 must not be marked when email could not be sent (retried next tick)")
	}
}
