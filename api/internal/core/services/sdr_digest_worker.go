package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// SDRDigestSender is the minimal contract the worker drives. Satisfied by
// usecases.SendSDRDigestUseCase.
type SDRDigestSender interface {
	Execute(ctx context.Context) (int, error)
}

// SDRDigestWorker fires the actionable daily digest (US-M010).
//
// It ticks hourly and does NOT decide the hour. The send window is per
// ACCOUNT and lives in the use case's due-query, in each account's own
// timezone — this worker used to gate on `time.Now().Hour()`, which is the
// container's clock in UTC, so every Brazilian account got its "start of
// your day" digest at 05:28. A worker cannot know whose morning it is.
//
// The use case is idempotent per account-day (DB stamp), so extra ticks
// inside an account's window never double-send — which is what makes an
// hourly tick safe.
type SDRDigestWorker struct {
	sender       SDRDigestSender
	pollInterval time.Duration
	running      atomic.Bool
}

func NewSDRDigestWorker(sender SDRDigestSender) *SDRDigestWorker {
	return &SDRDigestWorker{
		sender:       sender,
		pollInterval: time.Hour,
	}
}

// WithPollInterval overrides the tick cadence (tests use ms-scale).
func (w *SDRDigestWorker) WithPollInterval(d time.Duration) *SDRDigestWorker {
	w.pollInterval = d
	return w
}

// Start runs the loop until ctx is cancelled. Once-per-process.
func (w *SDRDigestWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("sdr digest worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("sdr digest worker started", "poll_interval", w.pollInterval)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("sdr digest worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *SDRDigestWorker) tick(ctx context.Context) {
	// No hour gate here on purpose: the window is per account, in the
	// account's timezone, and the due-query applies it. An account whose
	// local morning has not arrived is simply not returned.
	sent, err := w.sender.Execute(ctx)
	if err != nil {
		slog.Error("sdr digest run failed", "error", err)
		return
	}
	if sent > 0 {
		slog.Info("sdr digest sent", "accounts", sent)
	}
}
