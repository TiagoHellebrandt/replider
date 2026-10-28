package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// OAuthCodePruner and OAuthTokenPruner are the minimal contracts the
// cleanup worker needs; domain.OAuthCodeRepository and
// domain.APITokenRepository satisfy them.
type OAuthCodePruner interface {
	DeleteExpired(ctx context.Context) error
}

type OAuthTokenPruner interface {
	DeleteExpired(ctx context.Context) error
}

// OAuthTokenCleanupWorker prunes expired OAuth authorization codes and
// aged-out access/refresh tokens (30-day grace after expiry/revocation
// for auditability; PATs are never auto-deleted). Pure hygiene — the
// auth queries already filter on expires_at/revoked_at.
type OAuthTokenCleanupWorker struct {
	codes        OAuthCodePruner
	tokens       OAuthTokenPruner
	pollInterval time.Duration
	running      atomic.Bool
}

func NewOAuthTokenCleanupWorker(codes OAuthCodePruner, tokens OAuthTokenPruner) *OAuthTokenCleanupWorker {
	return &OAuthTokenCleanupWorker{
		codes:        codes,
		tokens:       tokens,
		pollInterval: time.Hour,
	}
}

// WithPollInterval overrides the tick cadence (tests use ms-scale).
func (w *OAuthTokenCleanupWorker) WithPollInterval(d time.Duration) *OAuthTokenCleanupWorker {
	w.pollInterval = d
	return w
}

// Start runs the cleanup loop until ctx is cancelled. Once-per-process.
func (w *OAuthTokenCleanupWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("oauth token cleanup worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("oauth token cleanup worker started", "poll_interval", w.pollInterval)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("oauth token cleanup worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *OAuthTokenCleanupWorker) tick(ctx context.Context) {
	if err := w.codes.DeleteExpired(ctx); err != nil {
		slog.Error("oauth cleanup: pruning authorization codes failed", "error", err)
	}
	if err := w.tokens.DeleteExpired(ctx); err != nil {
		slog.Error("oauth cleanup: pruning api tokens failed", "error", err)
	}
}
