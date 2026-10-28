package services

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeUsageSummaryRefresher struct {
	calls atomic.Int32
	err   error
}

func (f *fakeUsageSummaryRefresher) RefreshUsageSummary(ctx context.Context) error {
	f.calls.Add(1)
	return f.err
}

// TestUsageSummaryRefreshWorker_TicksAndRefreshes verifies US-065: the worker
// invokes RefreshUsageSummary on its cadence. We tighten the poll interval to
// ms-scale and assert it fires at least the immediate tick + one periodic.
func TestUsageSummaryRefreshWorker_TicksAndRefreshes(t *testing.T) {
	refresher := &fakeUsageSummaryRefresher{}
	w := NewUsageSummaryRefreshWorker(refresher).WithPollInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	assert.Eventually(t, func() bool {
		return refresher.calls.Load() >= 2 // immediate tick + at least one periodic
	}, 2*time.Second, 5*time.Millisecond)

	cancel()
}

// TestUsageSummaryRefreshWorker_SurvivesError confirms a failing refresh
// doesn't kill the loop — it logs and keeps ticking.
func TestUsageSummaryRefreshWorker_SurvivesError(t *testing.T) {
	refresher := &fakeUsageSummaryRefresher{err: assert.AnError}
	w := NewUsageSummaryRefreshWorker(refresher).WithPollInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	assert.Eventually(t, func() bool {
		return refresher.calls.Load() >= 2 // kept ticking despite the error
	}, 2*time.Second, 5*time.Millisecond)

	cancel()
}
