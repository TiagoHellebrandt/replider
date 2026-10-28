package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// IndexRefresher is the minimal contract the worker needs.
// *usecases.ContractAdjustmentUseCase satisfies it.
type IndexRefresher interface {
	RefreshIndices(ctx context.Context, codes []domain.IndexCode) error
}

// EconomicIndexWorker keeps the published index series current.
//
// The series is what turns "reajuste IPCA em março" into "you may raise 4.83%",
// and a stale cache does not produce a wrong number — the accumulation reports
// itself incomplete — but it does produce a useless alert. So this runs, and it
// runs on a cadence matched to how often the data actually changes: the indices
// are published ONCE A MONTH.
//
// Twice a day rather than monthly: cheap, and it means a deployment whose first
// tick landed the day before publication is only hours behind instead of a month.
type EconomicIndexWorker struct {
	uc     IndexRefresher
	codes  []domain.IndexCode
	period time.Duration
}

func NewEconomicIndexWorker(uc IndexRefresher, codes []domain.IndexCode) *EconomicIndexWorker {
	return &EconomicIndexWorker{uc: uc, codes: codes, period: 12 * time.Hour}
}

// WithPeriod overrides the cadence (tests use ms-scale).
func (w *EconomicIndexWorker) WithPeriod(d time.Duration) *EconomicIndexWorker {
	w.period = d
	return w
}

// Start runs until ctx is cancelled.
//
// Ticks IMMEDIATELY on start, unlike the scanner worker: a cold deployment has
// an empty series, and every contract alert would carry no percentage until the
// first tick. There is nothing to warm up on the other side — it is a read of a
// public endpoint.
func (w *EconomicIndexWorker) Start(ctx context.Context) {
	if w.uc == nil || len(w.codes) == 0 {
		slog.InfoContext(ctx, "economic index worker: not configured, skipping")
		return
	}

	w.sweep(ctx)

	ticker := time.NewTicker(w.period)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.sweep(ctx)
		}
	}
}

func (w *EconomicIndexWorker) sweep(ctx context.Context) {
	// Bounded per sweep: a government endpoint that hangs must not hold this
	// goroutine until the next tick.
	sweepCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err := w.uc.RefreshIndices(sweepCtx, w.codes); err != nil {
		slog.WarnContext(ctx, "economic index worker: refresh failed", "error", err)
	}
}
