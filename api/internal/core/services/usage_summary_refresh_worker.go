package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// UsageSummaryRefresher is the minimal contract the refresh worker needs.
// A one-method interface keeps the test fake tiny and avoids dragging the
// rest of UsageRepository into this package. *postgres.UsageRepository
// satisfies it.
type UsageSummaryRefresher interface {
	RefreshUsageSummary(ctx context.Context) error
}

// UsageSummaryRefreshWorker periodically recomputes the account_usage_summary
// materialized view (US-065) so the read-only usage dashboard reads a
// precomputed latest-cycle row per account instead of running the
// filter+sort aggregation on every request. A 5-minute cadence matches the
// acceptance criteria and is plenty fresh for a usage banner; live plan-limit
// enforcement never reads the view, so the staleness is purely cosmetic.
type UsageSummaryRefreshWorker struct {
	refresher    UsageSummaryRefresher
	pollInterval time.Duration
	running      atomic.Bool
}

func NewUsageSummaryRefreshWorker(refresher UsageSummaryRefresher) *UsageSummaryRefreshWorker {
	return &UsageSummaryRefreshWorker{
		refresher:    refresher,
		pollInterval: 5 * time.Minute,
	}
}

// WithPollInterval overrides the tick cadence (tests use ms-scale).
func (w *UsageSummaryRefreshWorker) WithPollInterval(d time.Duration) *UsageSummaryRefreshWorker {
	w.pollInterval = d
	return w
}

// Start runs the refresh loop until ctx is cancelled. Once-per-process.
func (w *UsageSummaryRefreshWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("usage summary refresh worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("usage summary refresh worker started", "poll_interval", w.pollInterval)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("usage summary refresh worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *UsageSummaryRefreshWorker) tick(ctx context.Context) {
	if err := w.refresher.RefreshUsageSummary(ctx); err != nil {
		slog.Warn("usage summary refresh worker: tick failed", "error", err)
	}
}
