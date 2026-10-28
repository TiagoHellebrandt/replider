package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// WinBackSender is the minimal contract the worker drives — one method
// keeps the test fake tiny (mirrors ReactivationSender). Satisfied by
// usecases.WinBackUseCase.
type WinBackSender interface {
	Execute(ctx context.Context) (int, error)
}

// WinBackWorker periodically fires the win-back follow-up reminders
// (US-179). It ticks daily; the use case decides per-account which
// milestone (7/30 days post-cancellation) is due and is idempotent per
// milestone (DB stage stamp), so a same-day restart re-running the tick
// never double-sends.
type WinBackWorker struct {
	sender       WinBackSender
	pollInterval time.Duration
	running      atomic.Bool
}

func NewWinBackWorker(sender WinBackSender) *WinBackWorker {
	return &WinBackWorker{
		sender:       sender,
		pollInterval: 24 * time.Hour,
	}
}

// WithPollInterval overrides the tick cadence (tests use ms-scale).
func (w *WinBackWorker) WithPollInterval(d time.Duration) *WinBackWorker {
	w.pollInterval = d
	return w
}

// Start runs the win-back loop until ctx is cancelled. Once-per-process.
func (w *WinBackWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("winback worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("winback worker started", "poll_interval", w.pollInterval)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("winback worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *WinBackWorker) tick(ctx context.Context) {
	if _, err := w.sender.Execute(ctx); err != nil {
		slog.Warn("winback worker: tick failed", "error", err)
	}
}
