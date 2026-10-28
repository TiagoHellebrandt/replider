package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// OverageAccountLister enumerates the accounts the overage reporter
// should push usage for. Kept as a one-method port (satisfied by the
// concrete postgres AccountRepository) so the worker stays decoupled
// from the full repository surface.
type OverageAccountLister interface {
	ListAccountIDsWithActiveSubscription(ctx context.Context) ([]string, error)
}

// OverageReporter reports one account's metered overage to Stripe and
// returns how many meters it reported. Satisfied by
// usecases.ReportOverageUsageUseCase.
type OverageReporter interface {
	Execute(ctx context.Context, accountID string) (int, error)
}

// OverageReportWorker periodically pushes usage-based (metered) overage
// to Stripe for every actively-subscribed account (US-168). Reporting
// uses action=`set` (the running cycle total), so the only thing the
// cadence controls is how fresh the metered quantity is on Stripe's side
// before the invoice finalizes — a missed tick self-heals on the next
// one. Six hours keeps the metered total close to real usage without
// hammering the Stripe API once accounts scale.
type OverageReportWorker struct {
	lister       OverageAccountLister
	reporter     OverageReporter
	pollInterval time.Duration
	running      atomic.Bool
}

func NewOverageReportWorker(lister OverageAccountLister, reporter OverageReporter) *OverageReportWorker {
	return &OverageReportWorker{
		lister:       lister,
		reporter:     reporter,
		pollInterval: 6 * time.Hour,
	}
}

// WithPollInterval overrides the tick cadence (tests use ms-scale).
func (w *OverageReportWorker) WithPollInterval(d time.Duration) *OverageReportWorker {
	w.pollInterval = d
	return w
}

// Start runs the reporting loop until ctx is cancelled. Once-per-process.
func (w *OverageReportWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("overage report worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("overage report worker started", "poll_interval", w.pollInterval)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("overage report worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *OverageReportWorker) tick(ctx context.Context) {
	ids, err := w.lister.ListAccountIDsWithActiveSubscription(ctx)
	if err != nil {
		slog.Warn("overage report worker: failed to list accounts", "error", err)
		return
	}
	var reported, failed int
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		if _, err := w.reporter.Execute(ctx, id); err != nil {
			// One account's failure must not stop the sweep — log and move on.
			failed++
			continue
		}
		reported++
	}
	if len(ids) > 0 {
		slog.Info("overage report worker: tick complete",
			"accounts", len(ids), "reported", reported, "failed", failed)
	}
}
