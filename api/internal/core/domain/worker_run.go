package domain

import (
	"context"
	"time"
)

// WorkerRunRepository persists "when did this worker last complete a pass"
// (table worker_runs, migration 000223).
//
// Why it exists: the 6h/24h workers deliberately have no boot pass (a
// deploy loop must not become three rounds of messages), but with a deploy
// most days a 24h ticker NEVER fires — the container is replaced before the
// first tick. Stamping each completed pass lets Start() decide honestly: run
// on boot when the last pass is older than the interval, otherwise wait for
// the ticker. The stamp is also what stops the signal poller's boot pass
// from drafting up to ten offers per account on every restart.
type WorkerRunRepository interface {
	// LastRun returns the stamp for a worker name, nil when it never ran.
	LastRun(ctx context.Context, name string) (*time.Time, error)
	// Stamp records that a pass completed at `at` (upsert).
	Stamp(ctx context.Context, name string, at time.Time) error
}
