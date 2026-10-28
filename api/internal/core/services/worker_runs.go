package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Boot-pass policy for the slow tickers (see domain.WorkerRunRepository).
//
// bootPassDue answers "should this worker run right now, at boot?": yes when
// it never completed a pass or the last one is older than `interval`. A nil
// log — the worker was wired without one — keeps the historical behaviour
// (no boot pass), because guessing "run now" for a worker that drafts
// messages is the expensive direction to be wrong in. A lookup FAILURE also
// answers no, for the same reason; it logs so the silence is visible.
func bootPassDue(ctx context.Context, log domain.WorkerRunRepository, name string, interval time.Duration, now time.Time) bool {
	if log == nil {
		return false
	}
	last, err := log.LastRun(ctx, name)
	if err != nil {
		slog.WarnContext(ctx, "worker run log: lookup failed; skipping the boot pass",
			"worker", name, "error", err)
		return false
	}
	if last == nil {
		return true
	}
	return now.Sub(*last) >= interval
}

// stampRun records a completed pass. Failures are logged, never fatal: the
// worker did its job, the stamp only shapes the NEXT boot.
func stampRun(ctx context.Context, log domain.WorkerRunRepository, name string, now time.Time) {
	if log == nil {
		return
	}
	if err := log.Stamp(ctx, name, now); err != nil {
		slog.WarnContext(ctx, "worker run log: stamp failed", "worker", name, "error", err)
	}
}
