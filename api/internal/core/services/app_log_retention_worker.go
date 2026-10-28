package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// AppLogRetentionWorker deletes log lines past their validity.
//
// The table exists to investigate what broke yesterday, not to become a
// historical archive: unpruned it grows forever, and a table nobody reads and
// everybody pays for is the definition of useless cost. domain.AppLogRetention is
// the deadline; here it is only carried out.
type AppLogRetentionWorker struct {
	repo         domain.AppLogRepository
	retention    time.Duration
	pollInterval time.Duration
	running      atomic.Bool
}

func NewAppLogRetentionWorker(repo domain.AppLogRepository) *AppLogRetentionWorker {
	return &AppLogRetentionWorker{
		repo:         repo,
		retention:    domain.AppLogRetention,
		pollInterval: time.Hour,
	}
}

// WithPollInterval and WithRetention exist so a test runs in milliseconds instead
// of waiting an hour.
func (w *AppLogRetentionWorker) WithPollInterval(d time.Duration) *AppLogRetentionWorker {
	w.pollInterval = d
	return w
}

func (w *AppLogRetentionWorker) WithRetention(d time.Duration) *AppLogRetentionWorker {
	w.retention = d
	return w
}

// Start runs until the context ends. Once per process.
func (w *AppLogRetentionWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("app log retention worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *AppLogRetentionWorker) tick(ctx context.Context) {
	deleted, err := w.repo.DeleteOlderThan(ctx, time.Now().Add(-w.retention))
	if err != nil {
		slog.Warn("app log retention: sweep failed", "error", err)
		return
	}
	if deleted > 0 {
		slog.Info("app log retention: pruned", "deleted", deleted)
	}
}
