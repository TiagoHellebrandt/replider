package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// ConversationResumer is the minimal contract this worker needs from
// the conversation repo. Defining a single-method interface keeps
// the test fakes small (one method to satisfy) and avoids leaking
// unrelated CRUD into the worker package.
type ConversationResumer interface {
	AutoResumeStuckConversations(ctx context.Context, idleMinutes int) (int64, error)
}

// ConversationResumeWorker periodically flips is_bot_paused back to
// FALSE on conversations whose human takeover went stale. The asker
// scenario: an attendant clicked "take over", got distracted, never
// came back. Without this worker the conversation stays paused
// forever and the bot can't help the customer when they message
// again.
//
// Default idle threshold is 7 days — long enough that genuine
// follow-up doesn't get cut short, short enough that the dashboard
// queue doesn't accumulate forgotten threads. Tunable via
// WithIdleHours for tests / per-tenant overrides.
type ConversationResumeWorker struct {
	convRepo     ConversationResumer
	pollInterval time.Duration
	idleHours    int
	running      atomic.Bool
}

func NewConversationResumeWorker(convRepo ConversationResumer) *ConversationResumeWorker {
	return &ConversationResumeWorker{
		convRepo:     convRepo,
		pollInterval: 1 * time.Hour,
		idleHours:    24 * 7, // 7 days
	}
}

// WithIdleHours overrides the staleness threshold. Tests use a
// minute-scale value to verify the auto-resume in seconds.
func (w *ConversationResumeWorker) WithIdleHours(h int) *ConversationResumeWorker {
	w.idleHours = h
	return w
}

// WithPollInterval overrides the tick cadence. Production runs hourly;
// tests can tighten to ms-scale.
func (w *ConversationResumeWorker) WithPollInterval(d time.Duration) *ConversationResumeWorker {
	w.pollInterval = d
	return w
}

// Start runs the resume loop until ctx is cancelled. Once-per-process
// guarantee — repeat calls log + return.
func (w *ConversationResumeWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("conversation resume worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)
	slog.Info("conversation resume worker started",
		"poll_interval", w.pollInterval,
		"idle_hours", w.idleHours)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("conversation resume worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *ConversationResumeWorker) tick(ctx context.Context) {
	idleMinutes := w.idleHours * 60
	resumed, err := w.convRepo.AutoResumeStuckConversations(ctx, idleMinutes)
	if err != nil {
		slog.Warn("conversation resume worker: tick failed", "error", err)
		return
	}
	if resumed > 0 {
		slog.Info("conversation resume worker: resumed stale takeovers",
			"resumed", resumed, "idle_hours", w.idleHours)
	}
}
