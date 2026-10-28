package services

import (
	"context"
	"log/slog"
	"time"
)

// ApproachScheduleWorker drains approved-but-capped approach sends when the
// mailbox's daily budget renews (Ofertas → Hoje, phase 1b).
//
// The worker is only a ticker: claiming, gating and sending live in the
// approach use case (DueApproachProcessor), so the scheduled path can never
// drift from the live-click path.
type DueApproachProcessor interface {
	ProcessDueScheduledApproaches(ctx context.Context, limit int32) (int, error)
}

type ApproachScheduleWorker struct {
	processor DueApproachProcessor
	interval  time.Duration
	batch     int32
}

func NewApproachScheduleWorker(p DueApproachProcessor) *ApproachScheduleWorker {
	return &ApproachScheduleWorker{processor: p, interval: 10 * time.Minute, batch: 20}
}

func (w *ApproachScheduleWorker) WithPollInterval(d time.Duration) *ApproachScheduleWorker {
	w.interval = d
	return w
}

func (w *ApproachScheduleWorker) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sent, err := w.processor.ProcessDueScheduledApproaches(ctx, w.batch)
				if err != nil {
					slog.Warn("approach schedule: drain failed", "error", err)
				} else if sent > 0 {
					slog.Info("approach schedule: sent scheduled approaches", "count", sent)
				}
			}
		}
	}()
}
