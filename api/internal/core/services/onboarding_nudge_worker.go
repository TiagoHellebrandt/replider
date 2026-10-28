package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// OnboardingNudgeSender is the minimal contract the worker drives — one
// method keeps the test fake tiny (mirrors ReactivationSender). Satisfied
// by usecases.SendOnboardingNudgesUseCase.
type OnboardingNudgeSender interface {
	Execute(ctx context.Context) (int, error)
}

// OnboardingNudgeWorker periodically fires the onboarding activation-drip
// emails (US-182). It ticks daily; the use case decides per-account which
// milestone (D1/D3/D7 after signup) is due and is idempotent per milestone
// (DB stage stamp), so a same-day restart re-running the tick never
// double-sends.
type OnboardingNudgeWorker struct {
	sender       OnboardingNudgeSender
	pollInterval time.Duration
	running      atomic.Bool
}

func NewOnboardingNudgeWorker(sender OnboardingNudgeSender) *OnboardingNudgeWorker {
	return &OnboardingNudgeWorker{
		sender:       sender,
		pollInterval: 24 * time.Hour,
	}
}

// WithPollInterval overrides the tick cadence (tests use ms-scale).
func (w *OnboardingNudgeWorker) WithPollInterval(d time.Duration) *OnboardingNudgeWorker {
	w.pollInterval = d
	return w
}

// Start runs the onboarding-nudge loop until ctx is cancelled. Once-per-process.
func (w *OnboardingNudgeWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("onboarding nudge worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("onboarding nudge worker started", "poll_interval", w.pollInterval)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("onboarding nudge worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *OnboardingNudgeWorker) tick(ctx context.Context) {
	if _, err := w.sender.Execute(ctx); err != nil {
		slog.Warn("onboarding nudge worker: tick failed", "error", err)
	}
}
