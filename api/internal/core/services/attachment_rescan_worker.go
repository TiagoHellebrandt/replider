package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// AttachmentRescanner is the minimal contract the worker needs.
// *usecases.MessageAttachmentUseCase satisfies it.
type AttachmentRescanner interface {
	Rescan(ctx context.Context, limit int32) (int, error)
}

// AttachmentRescanWorker re-checks files that were stored while the scanner
// was unavailable.
//
// Without it, a five-minute clamd outage leaves every attachment received in
// that window permanently unopenable: the verdict is 'failed', 'failed' is
// not downloadable, and nothing would ever revisit it. A real contract would
// sit in the inbox forever behind a message the user cannot act on — which is
// a quieter version of the bug this whole feature replaced.
//
// The sweep is small and frequent rather than large and rare: a batch big
// enough to matter would also be a batch that hammers clamd, and the daemon
// is single-threaded per connection.
type AttachmentRescanWorker struct {
	uc           AttachmentRescanner
	pollInterval time.Duration
	batchSize    int32
	running      atomic.Bool
}

func NewAttachmentRescanWorker(uc AttachmentRescanner) *AttachmentRescanWorker {
	return &AttachmentRescanWorker{
		uc:           uc,
		pollInterval: 30 * time.Minute,
		batchSize:    50,
	}
}

// WithPollInterval overrides the cadence (tests use ms-scale).
func (w *AttachmentRescanWorker) WithPollInterval(d time.Duration) *AttachmentRescanWorker {
	w.pollInterval = d
	return w
}

// WithBatchSize overrides how many rows one sweep settles.
func (w *AttachmentRescanWorker) WithBatchSize(n int32) *AttachmentRescanWorker {
	w.batchSize = n
	return w
}

// Start runs until ctx is cancelled. Once per process.
//
// Deliberately does NOT tick immediately on start: a deploy restarts the API
// and clamd at the same time, and the daemon takes minutes to load its
// signature database. An immediate sweep would re-record everything as
// 'failed' against a scanner that is merely still booting.
func (w *AttachmentRescanWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("attachment rescan worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("attachment rescan worker started",
		"poll_interval", w.pollInterval, "batch_size", w.batchSize)

	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("attachment rescan worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *AttachmentRescanWorker) tick(ctx context.Context) {
	settled, err := w.uc.Rescan(ctx, w.batchSize)
	if err != nil {
		slog.Warn("attachment rescan worker: sweep failed", "error", err)
		return
	}
	if settled > 0 {
		slog.Info("attachment rescan worker: settled pending files", "settled", settled)
	}
}
