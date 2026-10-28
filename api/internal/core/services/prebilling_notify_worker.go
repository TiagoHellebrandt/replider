package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// PrebillingNotifier is the minimal contract the worker drives — kept to
// one method so the test fake stays tiny (mirrors StripeEventCleaner).
type PrebillingNotifier interface {
	Execute(ctx context.Context) (int, error)
}

// PrebillingNotifyWorker periodically fires the pre-billing reminder
// use case (US-124), which emails admins ~72h before their subscription
// renews. The use case is idempotent per billing cycle (DB guard), so
// the only thing the cadence controls is worst-case notification
// latency. Hourly keeps reminders prompt while the 72h window is wide
// enough that an account never slips through between ticks.
type PrebillingNotifyWorker struct {
	notifier     PrebillingNotifier
	pollInterval time.Duration
	running      atomic.Bool
}

func NewPrebillingNotifyWorker(notifier PrebillingNotifier) *PrebillingNotifyWorker {
	return &PrebillingNotifyWorker{
		notifier:     notifier,
		pollInterval: time.Hour,
	}
}

// WithPollInterval overrides the tick cadence (tests use ms-scale).
func (w *PrebillingNotifyWorker) WithPollInterval(d time.Duration) *PrebillingNotifyWorker {
	w.pollInterval = d
	return w
}

// Start runs the reminder loop until ctx is cancelled. Once-per-process.
func (w *PrebillingNotifyWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("pre-billing notify worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("pre-billing notify worker started", "poll_interval", w.pollInterval)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("pre-billing notify worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *PrebillingNotifyWorker) tick(ctx context.Context) {
	if _, err := w.notifier.Execute(ctx); err != nil {
		slog.Warn("pre-billing notify worker: tick failed", "error", err)
	}
}
