package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// PixRenewalSender is the minimal contract the worker drives. Satisfied by
// usecases.SendPixRenewalCycleUseCase.
type PixRenewalSender interface {
	Execute(ctx context.Context) (int, error)
}

// PixRenewalWorker walks the PIX renewal ladder (US-M014).
//
// Daily, not hourly: every rung is a message to a paying customer or a
// suspension, and the ladder is defined in whole days. Running more often
// could not deliver anything new — the use case is idempotent per day —
// but it would multiply the chance of a partial run mid-incident.
type PixRenewalWorker struct {
	sender       PixRenewalSender
	pollInterval time.Duration
	running      atomic.Bool
}

func NewPixRenewalWorker(sender PixRenewalSender) *PixRenewalWorker {
	return &PixRenewalWorker{sender: sender, pollInterval: 24 * time.Hour}
}

// WithPollInterval overrides the tick cadence (tests use ms-scale).
func (w *PixRenewalWorker) WithPollInterval(d time.Duration) *PixRenewalWorker {
	w.pollInterval = d
	return w
}

// Start runs the loop until ctx is cancelled. Once-per-process.
func (w *PixRenewalWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("pix renewal worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("pix renewal worker started", "poll_interval", w.pollInterval)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("pix renewal worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *PixRenewalWorker) tick(ctx context.Context) {
	delivered, err := w.sender.Execute(ctx)
	if err != nil {
		slog.Error("pix renewal run failed", "error", err)
		return
	}
	if delivered > 0 {
		slog.Info("pix renewal ladder advanced", "accounts", delivered)
	}
}
