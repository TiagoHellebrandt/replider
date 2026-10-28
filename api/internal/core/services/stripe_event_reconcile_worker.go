package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// StripeEventReconciler is the minimal contract the reconcile worker
// needs. A one-method interface keeps the test fake tiny and avoids
// dragging the whole webhook use case into this package.
type StripeEventReconciler interface {
	ReconcileStuck(ctx context.Context) (int, error)
}

// StripeEventReconcileWorker periodically re-runs Stripe webhook events
// that were claimed but never finished — handlers that crashed mid-flight
// (US-011). Without it a process that died between claiming an event and
// committing its side-effects would drop the event forever (no usage
// reset, no subscription activation). The reconciler only touches rows
// whose claim is older than 5 minutes (enforced in the repo query), so it
// never races a live, in-flight handler.
type StripeEventReconcileWorker struct {
	reconciler   StripeEventReconciler
	pollInterval time.Duration
	running      atomic.Bool
}

func NewStripeEventReconcileWorker(reconciler StripeEventReconciler) *StripeEventReconcileWorker {
	return &StripeEventReconcileWorker{
		reconciler: reconciler,
		// Stuck rows aren't eligible until their claim is >5m old, so a
		// minute cadence recovers them promptly without busy-looping.
		pollInterval: time.Minute,
	}
}

// WithPollInterval overrides the tick cadence (tests use ms-scale).
func (w *StripeEventReconcileWorker) WithPollInterval(d time.Duration) *StripeEventReconcileWorker {
	w.pollInterval = d
	return w
}

// Start runs the reconcile loop until ctx is cancelled. Once-per-process.
func (w *StripeEventReconcileWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("stripe event reconcile worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("stripe event reconcile worker started", "poll_interval", w.pollInterval)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("stripe event reconcile worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *StripeEventReconcileWorker) tick(ctx context.Context) {
	recovered, err := w.reconciler.ReconcileStuck(ctx)
	if err != nil {
		slog.Warn("stripe event reconcile worker: tick failed", "error", err)
		return
	}
	if recovered > 0 {
		slog.Info("stripe event reconcile worker: recovered stuck events", "recovered", recovered)
	}
}
