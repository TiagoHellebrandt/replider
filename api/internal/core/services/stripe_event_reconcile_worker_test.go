package services

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeStripeEventReconciler struct {
	calls     atomic.Int32
	recovered int
	err       error
}

func (f *fakeStripeEventReconciler) ReconcileStuck(ctx context.Context) (int, error) {
	f.calls.Add(1)
	return f.recovered, f.err
}

// TestStripeEventReconcileWorker_TicksAndReconciles verifies US-011: the
// worker invokes ReconcileStuck on its cadence (immediate tick + periodic).
func TestStripeEventReconcileWorker_TicksAndReconciles(t *testing.T) {
	rec := &fakeStripeEventReconciler{recovered: 2}
	w := NewStripeEventReconcileWorker(rec).WithPollInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	assert.Eventually(t, func() bool {
		return rec.calls.Load() >= 2
	}, 2*time.Second, 5*time.Millisecond)

	cancel()
}

// TestStripeEventReconcileWorker_SurvivesError confirms a failing tick
// doesn't kill the loop — it logs and keeps ticking.
func TestStripeEventReconcileWorker_SurvivesError(t *testing.T) {
	rec := &fakeStripeEventReconciler{err: assert.AnError}
	w := NewStripeEventReconcileWorker(rec).WithPollInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)

	assert.Eventually(t, func() bool {
		return rec.calls.Load() >= 2
	}, 2*time.Second, 5*time.Millisecond)

	cancel()
}
