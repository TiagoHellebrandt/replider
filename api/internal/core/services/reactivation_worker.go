package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// ReactivationSender is the minimal contract the worker drives — one
// method keeps the test fake tiny (mirrors PrebillingNotifier). Satisfied
// by usecases.SendReactivationEmailsUseCase.
type ReactivationSender interface {
	Execute(ctx context.Context) (int, error)
}

// ReactivationWorker periodically fires the lapsed-trial reactivation
// emails (US-181). It ticks daily; the use case decides per-account which
// milestone (7/14/30 days post-expiry) is due and is idempotent per
// milestone (DB stage stamp), so a same-day restart re-running the tick
// never double-sends.
type ReactivationWorker struct {
	sender       ReactivationSender
	pollInterval time.Duration
	running      atomic.Bool
}

func NewReactivationWorker(sender ReactivationSender) *ReactivationWorker {
	return &ReactivationWorker{
		sender:       sender,
		pollInterval: 24 * time.Hour,
	}
}

// WithPollInterval overrides the tick cadence (tests use ms-scale).
func (w *ReactivationWorker) WithPollInterval(d time.Duration) *ReactivationWorker {
	w.pollInterval = d
	return w
}

// Start runs the reactivation loop until ctx is cancelled. Once-per-process.
func (w *ReactivationWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("reactivation worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("reactivation worker started", "poll_interval", w.pollInterval)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("reactivation worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *ReactivationWorker) tick(ctx context.Context) {
	if _, err := w.sender.Execute(ctx); err != nil {
		slog.Warn("reactivation worker: tick failed", "error", err)
	}
}
