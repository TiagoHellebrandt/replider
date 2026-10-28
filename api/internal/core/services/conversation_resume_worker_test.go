package services

import (
	"context"
	"errors"
	"testing"
)

type fakeResumeRepo struct {
	calls       []int
	returnRows  int64
	returnError error
}

func (f *fakeResumeRepo) AutoResumeStuckConversations(_ context.Context, idleMinutes int) (int64, error) {
	f.calls = append(f.calls, idleMinutes)
	return f.returnRows, f.returnError
}

func TestConversationResumeWorker_TickPassesIdleMinutes(t *testing.T) {
	repo := &fakeResumeRepo{returnRows: 3}
	w := NewConversationResumeWorker(repo).WithIdleHours(48)

	w.tick(context.Background())

	if len(repo.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(repo.calls))
	}
	if repo.calls[0] != 48*60 {
		t.Errorf("expected idleMinutes=2880, got %d", repo.calls[0])
	}
}

func TestConversationResumeWorker_TickToleratesError(t *testing.T) {
	repo := &fakeResumeRepo{returnError: errors.New("db down")}
	w := NewConversationResumeWorker(repo)
	// tick should not panic on error
	w.tick(context.Background())
}
