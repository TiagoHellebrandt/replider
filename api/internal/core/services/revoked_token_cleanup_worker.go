package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// RevokedTokenPruner is the minimal contract the cleanup worker needs.
// domain.RevokedTokenStore satisfies it.
type RevokedTokenPruner interface {
	PruneExpired(ctx context.Context, now time.Time) (int64, error)
}

// RevokedTokenCleanupWorker periodically prunes aged-out rows from the
// revoked_refresh_tokens durable revocation list (US-082). Entries are
// only useful until the token they reference expires (<=24h), so an
// hourly sweep keeps the table bounded. Stale rows are already harmless
// for correctness (RevokedTokenStore.Exists filters on expires_at) — this
// is pure hygiene.
type RevokedTokenCleanupWorker struct {
	pruner       RevokedTokenPruner
	pollInterval time.Duration
	running      atomic.Bool
}

func NewRevokedTokenCleanupWorker(pruner RevokedTokenPruner) *RevokedTokenCleanupWorker {
	return &RevokedTokenCleanupWorker{
		pruner:       pruner,
		pollInterval: time.Hour,
	}
}

// WithPollInterval overrides the tick cadence (tests use ms-scale).
func (w *RevokedTokenCleanupWorker) WithPollInterval(d time.Duration) *RevokedTokenCleanupWorker {
	w.pollInterval = d
	return w
}

// Start runs the cleanup loop until ctx is cancelled. Once-per-process.
func (w *RevokedTokenCleanupWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("revoked token cleanup worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("revoked token cleanup worker started", "poll_interval", w.pollInterval)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("revoked token cleanup worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *RevokedTokenCleanupWorker) tick(ctx context.Context) {
	deleted, err := w.pruner.PruneExpired(ctx, time.Now())
	if err != nil {
		slog.Warn("revoked token cleanup worker: tick failed", "error", err)
		return
	}
	if deleted > 0 {
		slog.Info("revoked token cleanup worker: pruned expired tokens", "deleted", deleted)
	}
}
