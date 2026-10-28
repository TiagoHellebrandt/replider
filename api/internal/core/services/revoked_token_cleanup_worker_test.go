package services

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeRevokedTokenPruner struct {
	calls   atomic.Int32
	deleted int64
	err     error
}

func (f *fakeRevokedTokenPruner) PruneExpired(_ context.Context, _ time.Time) (int64, error) {
	f.calls.Add(1)
	return f.deleted, f.err
}

// TestRevokedTokenCleanupWorker_TicksAndPrunes verifies US-082: the worker
// invokes PruneExpired on its cadence (immediate tick + at least one
// periodic).
func TestRevokedTokenCleanupWorker_TicksAndPrunes(t *testing.T) {
	pruner := &fakeRevokedTokenPruner{deleted: 2}
	w := NewRevokedTokenCleanupWorker(pruner).WithPollInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	assert.Eventually(t, func() bool {
		return pruner.calls.Load() >= 2
	}, 2*time.Second, 5*time.Millisecond)

	cancel()
}

// TestRevokedTokenCleanupWorker_SurvivesError confirms a failing prune
// doesn't kill the loop.
func TestRevokedTokenCleanupWorker_SurvivesError(t *testing.T) {
	pruner := &fakeRevokedTokenPruner{err: assert.AnError}
	w := NewRevokedTokenCleanupWorker(pruner).WithPollInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	assert.Eventually(t, func() bool {
		return pruner.calls.Load() >= 2
	}, 2*time.Second, 5*time.Millisecond)

	cancel()
}
