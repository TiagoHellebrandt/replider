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

type fakeDigestStore struct {
	due      []domain.WeeklyDigestAccount
	stats    domain.WeeklyDigestStats
	statsErr error
	listErr  error

	mu          sync.Mutex
	marked      []string
	computedFor []string
}

func (f *fakeDigestStore) ListAccountsDueForDigest(_ context.Context, _ time.Time) ([]domain.WeeklyDigestAccount, error) {
	return f.due, f.listErr
}

func (f *fakeDigestStore) ComputeWeeklyStats(_ context.Context, accountID string, _ time.Time) (domain.WeeklyDigestStats, error) {
	f.mu.Lock()
	f.computedFor = append(f.computedFor, accountID)
	f.mu.Unlock()
	return f.stats, f.statsErr
}

func (f *fakeDigestStore) MarkDigestSent(_ context.Context, accountID string, _ time.Time) error {
	f.mu.Lock()
	f.marked = append(f.marked, accountID)
	f.mu.Unlock()
	return nil
}

type fakeAdminResolver struct {
	admins map[string]*domain.User
}

func (f *fakeAdminResolver) GetAdminByAccountID(_ context.Context, accountID string) (*domain.User, error) {
	return f.admins[accountID], nil
}

type fakeDigestNotifier struct {
	mu   sync.Mutex
	sent []string // recipient emails
	err  error
}

func (f *fakeDigestNotifier) SendWeeklyDigestEmail(to, _ string, _ domain.WeeklyDigest, _, _, _ string) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	f.sent = append(f.sent, to)
	f.mu.Unlock()
	return nil
}

// --- tests -----------------------------------------------------------------

func TestSendWeeklyDigest_SendsToDueAccountsAndMarks(t *testing.T) {
	store := &fakeDigestStore{
		due: []domain.WeeklyDigestAccount{
			{AccountID: "acc1", Name: "A"},
			{AccountID: "acc2", Name: "B"},
		},
		stats: domain.WeeklyDigestStats{MessagesHandled: 10, TimeSavedMinutes: 20, ConversionPct: 50},
	}
	admins := &fakeAdminResolver{admins: map[string]*domain.User{
		"acc1": {Email: "a@x.com", Name: "Ana"},
		"acc2": {Email: "b@x.com", Name: "Bob"},
	}}
	notifier := &fakeDigestNotifier{}

	uc := NewSendWeeklyDigestUseCase(store, admins, notifier, "https://app.test")
	sent, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 2 {
		t.Fatalf("expected 2 sent, got %d", sent)
	}
	if len(notifier.sent) != 2 {
		t.Fatalf("expected 2 emails, got %d", len(notifier.sent))
	}
	if len(store.marked) != 2 {
		t.Fatalf("expected 2 marks, got %d", len(store.marked))
	}
}

func TestSendWeeklyDigest_SkipsAccountWithoutAdmin(t *testing.T) {
	store := &fakeDigestStore{
		due: []domain.WeeklyDigestAccount{{AccountID: "acc1", Name: "A"}},
	}
	admins := &fakeAdminResolver{admins: map[string]*domain.User{}} // no admin
	notifier := &fakeDigestNotifier{}

	uc := NewSendWeeklyDigestUseCase(store, admins, notifier, "https://app.test")
	sent, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent != 0 {
		t.Fatalf("expected 0 sent, got %d", sent)
	}
	// Not stamped, so it retries next tick.
	if len(store.marked) != 0 {
		t.Fatalf("account without admin must not be marked sent")
	}
	// Stats must not even be computed for an unresolvable admin.
	if len(store.computedFor) != 0 {
		t.Fatalf("stats should not be computed when admin is missing")
	}
}

func TestSendWeeklyDigest_SendFailureNotMarked(t *testing.T) {
	store := &fakeDigestStore{
		due: []domain.WeeklyDigestAccount{{AccountID: "acc1", Name: "A"}},
	}
	admins := &fakeAdminResolver{admins: map[string]*domain.User{"acc1": {Email: "a@x.com"}}}
	notifier := &fakeDigestNotifier{err: errors.New("smtp down")}

	uc := NewSendWeeklyDigestUseCase(store, admins, notifier, "https://app.test")
	sent, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("batch should not abort on one send failure: %v", err)
	}
	if sent != 0 {
		t.Fatalf("expected 0 sent on send failure, got %d", sent)
	}
	if len(store.marked) != 0 {
		t.Fatalf("failed send must not be marked sent")
	}
}

func TestSendWeeklyDigest_ListErrorPropagates(t *testing.T) {
	store := &fakeDigestStore{listErr: errors.New("db down")}
	uc := NewSendWeeklyDigestUseCase(store, &fakeAdminResolver{}, &fakeDigestNotifier{}, "https://app.test")
	if _, err := uc.Execute(context.Background()); err == nil {
		t.Fatal("expected list error to propagate")
	}
}
