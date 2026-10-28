package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Inbound debounce worker (§6.15, K1).
//
// Answers the bursts whose silence window has passed. A worker rather than a
// per-message timer because a timer lives in one process: a deploy mid-burst
// would swallow the reply, and two pods would each answer the same one.

// PendingInboundRunner is what the worker calls when a wait is over. Narrow port
// so the worker does not depend on the whole inbound use case.
type PendingInboundRunner interface {
	RunPending(ctx context.Context, pending *domain.PendingInbound)
}

// InboundDebounceWorker drains the queue.
type InboundDebounceWorker struct {
	repo   domain.InboundDebounceRepository
	runner PendingInboundRunner

	// tick is how often the queue is checked. Five seconds: the window is tens
	// of seconds, so a 5s granularity is invisible to the customer, and a
	// tighter tick would be a query every second per pod for a table that is
	// usually empty.
	tick time.Duration
	// batch bounds one pass. A hundred conversations answered per tick is far
	// more than any real account produces, and the bound is what stops a
	// backlog from turning one pass into a thundering herd of model calls.
	batch int
	// staleClaim is how long a claimed row may sit before it is assumed dead
	// and re-queued. Comfortably longer than the pipeline's own 90s ceiling, so
	// a slow-but-alive run is never duplicated.
	staleClaim time.Duration

	now func() time.Time
}

func NewInboundDebounceWorker(
	repo domain.InboundDebounceRepository, runner PendingInboundRunner,
) *InboundDebounceWorker {
	return &InboundDebounceWorker{
		repo:       repo,
		runner:     runner,
		tick:       5 * time.Second,
		batch:      100,
		staleClaim: 5 * time.Minute,
		now:        time.Now,
	}
}

// Start runs the loop until ctx is done.
func (w *InboundDebounceWorker) Start(ctx context.Context) {
	if w == nil || w.repo == nil || w.runner == nil {
		slog.Info("inbound debounce worker not started (unwired)")
		return
	}
	slog.Info("inbound debounce worker started", "tick", w.tick)

	ticker := time.NewTicker(w.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("inbound debounce worker stopped")
			return
		case <-ticker.C:
			w.RunOnce(ctx)
		}
	}
}

// RunOnce claims and answers one batch. Exported for the tests, which drive it
// directly rather than waiting on a ticker.
func (w *InboundDebounceWorker) RunOnce(ctx context.Context) {
	now := w.now()

	// The stale sweep runs FIRST so a row a dead pod left claimed becomes due in
	// this same pass, instead of waiting a whole extra tick.
	if released, err := w.repo.ReleaseStaleClaims(ctx, now.Add(-w.staleClaim)); err != nil {
		slog.Error("inbound debounce: stale claim sweep failed", "error", err)
	} else if released > 0 {
		// WARN, not Debug: this only happens when a pod died mid-reply, and it
		// means somebody's answer was late. Worth seeing in the logs.
		slog.Warn("inbound debounce: re-queued runs a dead worker left claimed",
			"count", released)
	}

	pending, err := w.repo.ClaimDue(ctx, now, w.batch)
	if err != nil {
		slog.Error("inbound debounce: could not claim due runs", "error", err)
		return
	}
	if len(pending) == 0 {
		return
	}
	slog.Debug("inbound debounce: answering due bursts", "count", len(pending))

	for _, p := range pending {
		// Sequentially, on purpose. Each run is a model call plus a channel
		// send, and the per-conversation serializer would queue them anyway; a
		// hundred goroutines would only move the contention into the AI provider
		// and the connection pool.
		w.runPending(ctx, p)
	}
}

// runPending isolates one run so a panic in the pipeline does not take the
// worker — and the whole queue — down with it.
func (w *InboundDebounceWorker) runPending(ctx context.Context, p *domain.PendingInbound) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("inbound debounce: panic while answering a burst",
				"conversation_id", p.ConversationID, "recover", r)
		}
	}()

	// Detached from the tick's context with its own ceiling: the reply must not
	// be cancelled by the next tick, and it must not hang forever either. 90s
	// matches the webhook handler's own async budget, so a deferred reply gets
	// exactly the same room as an immediate one.
	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 90*time.Second)
	defer cancel()

	w.runner.RunPending(runCtx, p)
}
