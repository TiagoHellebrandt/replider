package services

import (
	"context"
	"log/slog"
	"time"
)

// The nightly sweep that makes the product go looking.
//
// Daily rather than hourly: the CNES park moves once a month (DATASUS
// publishes a competência), so hunting more often would re-ask a question
// whose answer has not changed. The tick still runs every day because the
// CATALOGUE moves — a distributor who imports a new brochure should see the
// hospitals for it the next morning, not next month.

// autoHunter is the slice of the use case this worker drives.
type autoHunter interface {
	Run(ctx context.Context) []AutoHuntSummary
}

// AutoHuntSummary mirrors usecases.AutoHuntResult across the layer boundary —
// services must not import usecases.
type AutoHuntSummary struct {
	AccountID string
	Found     int
	Created   int
}

type AutoHuntWorker struct {
	hunter   autoHunter
	interval time.Duration
}

func NewAutoHuntWorker(hunter autoHunter) *AutoHuntWorker {
	return &AutoHuntWorker{hunter: hunter, interval: 24 * time.Hour}
}

// WithInterval overrides the tick. Tests use it; production does not.
func (w *AutoHuntWorker) WithInterval(d time.Duration) *AutoHuntWorker {
	w.interval = d
	return w
}

func (w *AutoHuntWorker) Start(ctx context.Context) {
	if w.hunter == nil {
		slog.Info("auto hunt worker: not wired, skipping")
		return
	}
	// Once on boot, BEFORE the ticker — the same shape every other worker in
	// this package uses, and for a reason that bites hardest here: a 24h
	// ticker with no boot tick means a process restarted daily never hunts at
	// all. Five deploys in one afternoon would have left the sweep looking
	// wired and doing nothing, which is the failure mode this whole feature
	// exists to end.
	w.tick(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *AutoHuntWorker) tick(ctx context.Context) {
	start := time.Now()
	results := w.hunter.Run(ctx)
	created, found := 0, 0
	for _, r := range results {
		created += r.Created
		found += r.Found
	}
	slog.InfoContext(ctx, "auto hunt: tick complete",
		"accounts", len(results), "found", found, "created", created,
		"duration", time.Since(start).String())
}
