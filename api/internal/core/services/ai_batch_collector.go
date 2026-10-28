package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// BatchCollector lands finished provider batch jobs (overnight drafts).
// Polls every 15 minutes: the provider targets 24h but usually answers in
// well under an hour, and the whole design only needs the answers before
// the morning.
type BatchCollector struct {
	collector interface {
		CollectBatchDrafts(ctx context.Context) int
	}
	tick    time.Duration
	running atomic.Bool
}

func NewBatchCollector(c interface {
	CollectBatchDrafts(ctx context.Context) int
}) *BatchCollector {
	return &BatchCollector{collector: c, tick: 15 * time.Minute}
}

// WithTick overrides the cadence (tests).
func (w *BatchCollector) WithTick(d time.Duration) *BatchCollector {
	w.tick = d
	return w
}

// Start runs until ctx is done. Once-per-process.
func (w *BatchCollector) Start(ctx context.Context) {
	if w == nil || w.collector == nil {
		return
	}
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("batch collector already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("ai batch collector started", "tick", w.tick)

	ticker := time.NewTicker(w.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("ai batch collector stopped")
			return
		case <-ticker.C:
			if landed := w.collector.CollectBatchDrafts(ctx); landed > 0 {
				slog.Info("ai batch collector landed drafts", "count", landed)
			}
		}
	}
}
