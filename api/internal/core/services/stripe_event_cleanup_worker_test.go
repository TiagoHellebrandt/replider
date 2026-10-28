package services

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeStripeEventCleaner struct {
	calls   atomic.Int32
	deleted int64
	err     error
}

func (f *fakeStripeEventCleaner) CleanupOldEvents(ctx context.Context) (int64, error) {
	f.calls.Add(1)
	return f.deleted, f.err
}

// TestStripeEventCleanupWorker_TicksAndDeletes verifies US-020: the worker
// invokes CleanupOldEvents on its cadence. We tighten the poll interval to
// ms-scale and assert it fires at least the immediate tick + one periodic.
func TestStripeEventCleanupWorker_TicksAndDeletes(t *testing.T) {
	cleaner := &fakeStripeEventCleaner{deleted: 3}
	w := NewStripeEventCleanupWorker(cleaner).WithPollInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	assert.Eventually(t, func() bool {
		return cleaner.calls.Load() >= 2 // immediate tick + at least one periodic
	}, 2*time.Second, 5*time.Millisecond)

	cancel()
}

// TestStripeEventCleanupWorker_SurvivesError confirms a failing cleanup
// doesn't kill the loop — it logs and keeps ticking.
func TestStripeEventCleanupWorker_SurvivesError(t *testing.T) {
	cleaner := &fakeStripeEventCleaner{err: assert.AnError}
	w := NewStripeEventCleanupWorker(cleaner).WithPollInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	assert.Eventually(t, func() bool {
		return cleaner.calls.Load() >= 2 // kept ticking despite the error
	}, 2*time.Second, 5*time.Millisecond)

	cancel()
}
