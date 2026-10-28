package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// StripeEventCleaner is the minimal contract the cleanup worker needs.
// A one-method interface keeps the test fake tiny and avoids dragging
// the rest of StripeEventRepository into this package.
type StripeEventCleaner interface {
	CleanupOldEvents(ctx context.Context) (int64, error)
}

// StripeEventCleanupWorker periodically prunes the processed_stripe_events
// idempotency table (US-020). Without it the table grows by one row per
// Stripe webhook forever; the rows are dead weight once Stripe's ~72h
// retry window has passed. Daily cadence is plenty — the table is small
// and the cutoff (7 days, in the repo query) leaves a wide safety margin.
type StripeEventCleanupWorker struct {
	cleaner      StripeEventCleaner
	pollInterval time.Duration
	running      atomic.Bool
}

func NewStripeEventCleanupWorker(cleaner StripeEventCleaner) *StripeEventCleanupWorker {
	return &StripeEventCleanupWorker{
		cleaner:      cleaner,
		pollInterval: 24 * time.Hour,
	}
}

// WithPollInterval overrides the tick cadence (tests use ms-scale).
func (w *StripeEventCleanupWorker) WithPollInterval(d time.Duration) *StripeEventCleanupWorker {
	w.pollInterval = d
	return w
}

// Start runs the cleanup loop until ctx is cancelled. Once-per-process.
func (w *StripeEventCleanupWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("stripe event cleanup worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("stripe event cleanup worker started", "poll_interval", w.pollInterval)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("stripe event cleanup worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *StripeEventCleanupWorker) tick(ctx context.Context) {
	deleted, err := w.cleaner.CleanupOldEvents(ctx)
	if err != nil {
		slog.Warn("stripe event cleanup worker: tick failed", "error", err)
		return
	}
	if deleted > 0 {
		slog.Info("stripe event cleanup worker: pruned old events", "deleted", deleted)
	}
}
