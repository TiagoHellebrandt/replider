package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// OverageMeterWorker periodically bills conversation overage for every
// account that has passed its plan franchise. The per-account billing
// logic lives in OverageSettler, which is also invoked synchronously by
// the invoice.paid handler so overage accrued since the last tick is
// settled before the cycle counters are reset.
type OverageMeterWorker struct {
	usageRepo    domain.UsageRepository
	settler      *OverageSettler
	pollInterval time.Duration
	running      atomic.Bool
}

func NewOverageMeterWorker(usageRepo domain.UsageRepository, settler *OverageSettler) *OverageMeterWorker {
	return &OverageMeterWorker{
		usageRepo:    usageRepo,
		settler:      settler,
		pollInterval: 6 * time.Hour, // 4 ticks/day — granular enough for monthly billing
	}
}

// WithPollInterval overrides the default cadence (used by tests).
func (w *OverageMeterWorker) WithPollInterval(d time.Duration) *OverageMeterWorker {
	if d > 0 {
		w.pollInterval = d
	}
	return w
}

// Start runs the worker loop until ctx is cancelled. Safe to call once;
// subsequent calls are no-ops.
func (w *OverageMeterWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("overage meter worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("overage meter worker started", "poll_interval", w.pollInterval)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("overage meter worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *OverageMeterWorker) tick(ctx context.Context) {
	candidates, err := w.usageRepo.ListOverageCandidates(ctx)
	if err != nil {
		slog.Warn("overage meter: list candidates failed", "error", err)
		return
	}
	for _, c := range candidates {
		if ctx.Err() != nil {
			return
		}
		if err := w.settler.Settle(ctx, c); err != nil {
			// Counter not advanced; the idempotency key makes the next
			// tick's retry safe.
			slog.Error("overage meter: settle failed — will retry next tick",
				"account_id", c.AccountID, "error", err)
		}
	}
}
