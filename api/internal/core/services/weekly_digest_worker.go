package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// WeeklyDigestSender is the minimal contract the worker drives — one
// method keeps the test fake tiny (mirrors PrebillingNotifier). Satisfied
// by usecases.SendWeeklyDigestUseCase.
type WeeklyDigestSender interface {
	Execute(ctx context.Context) (int, error)
}

// WeeklyDigestWorker fires the weekly retention digest (US-176) on
// Mondays. It ticks daily and only runs the send on the configured send
// day, so the digest goes out once a week regardless of when the process
// last restarted. The use case is idempotent per week (DB last-sent
// stamp), so a same-day restart re-running the tick never double-sends.
type WeeklyDigestWorker struct {
	sender       WeeklyDigestSender
	pollInterval time.Duration
	sendDay      time.Weekday
	running      atomic.Bool
	// now is injectable so tests can pin the weekday without waiting for
	// a real Monday.
	now func() time.Time
}

func NewWeeklyDigestWorker(sender WeeklyDigestSender) *WeeklyDigestWorker {
	return &WeeklyDigestWorker{
		sender:       sender,
		pollInterval: 24 * time.Hour,
		sendDay:      time.Monday,
		now:          time.Now,
	}
}

// WithPollInterval overrides the tick cadence (tests use ms-scale).
func (w *WeeklyDigestWorker) WithPollInterval(d time.Duration) *WeeklyDigestWorker {
	w.pollInterval = d
	return w
}

// WithClock overrides the time source (tests only).
func (w *WeeklyDigestWorker) WithClock(now func() time.Time) *WeeklyDigestWorker {
	w.now = now
	return w
}

// WithSendDay overrides the day the digest is sent (tests only).
func (w *WeeklyDigestWorker) WithSendDay(d time.Weekday) *WeeklyDigestWorker {
	w.sendDay = d
	return w
}

// Start runs the digest loop until ctx is cancelled. Once-per-process.
func (w *WeeklyDigestWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("weekly digest worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("weekly digest worker started", "poll_interval", w.pollInterval, "send_day", w.sendDay.String())

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("weekly digest worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *WeeklyDigestWorker) tick(ctx context.Context) {
	if w.now().Weekday() != w.sendDay {
		return
	}
	if _, err := w.sender.Execute(ctx); err != nil {
		slog.Warn("weekly digest worker: tick failed", "error", err)
	}
}
