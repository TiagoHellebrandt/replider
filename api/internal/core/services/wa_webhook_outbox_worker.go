package services

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// WAWebhookOutboxProcessor is the function the worker invokes to replay
// a row that landed in `pending`/failed state. The handler exposes its
// inline processing logic via this single-method contract so the worker
// doesn't need to know about gin/HTTP. Returns:
//
//	(processed=true, nil) — every side effect committed; mark Processed.
//	(processed=false, nil) — recognised but intentionally not actionable
//	    (Meta test webhook, system event, deduped retry); mark Skipped.
//	(_, err) — transient failure; bump attempts, leave row pending so
//	    the next tick retries.
type WAWebhookOutboxProcessor func(ctx context.Context, payload []byte) (processed bool, err error)

// WAWebhookOutboxWorker drains pending rows from `wa_webhook_events`.
// Goal: a transient parser bug, AI quota exhaustion, or DB blip
// during fanout doesn't lose the inbound — the worker re-runs the
// inline processing on the next tick, capped by maxAttempts to avoid
// infinite retry of broken payloads.
type WAWebhookOutboxWorker struct {
	outbox       domain.WAWebhookOutbox
	process      WAWebhookOutboxProcessor
	alerter      FailureReporter
	pendingGauge func(int)
	pollInterval time.Duration
	batchSize    int
	maxAttempts  int
	pruneAge     time.Duration
	running      atomic.Bool
}

func NewWAWebhookOutboxWorker(outbox domain.WAWebhookOutbox, process WAWebhookOutboxProcessor) *WAWebhookOutboxWorker {
	return &WAWebhookOutboxWorker{
		outbox:       outbox,
		process:      process,
		pollInterval: 30 * time.Second,
		batchSize:    50,
		maxAttempts:  6, // ~3 minutes of retries at 30s spacing.
		pruneAge:     14 * 24 * time.Hour,
	}
}

func (w *WAWebhookOutboxWorker) WithPollInterval(d time.Duration) *WAWebhookOutboxWorker {
	w.pollInterval = d
	return w
}

func (w *WAWebhookOutboxWorker) WithMaxAttempts(n int) *WAWebhookOutboxWorker {
	w.maxAttempts = n
	return w
}

func (w *WAWebhookOutboxWorker) SetAlerter(a FailureReporter) {
	w.alerter = a
}

// SetPendingGauge wires a metric callback that receives the size of
// the pending batch on every tick. Kept as a function so the worker
// stays free of a Prometheus dependency — main.go injects the gauge.
func (w *WAWebhookOutboxWorker) SetPendingGauge(set func(int)) {
	w.pendingGauge = set
}

// Start runs the drain loop until ctx is cancelled. Once-per-process
// guarantee — repeat calls log + return.
func (w *WAWebhookOutboxWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("wa webhook outbox worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)

	slog.Info("wa webhook outbox worker started",
		"poll_interval", w.pollInterval,
		"batch_size", w.batchSize,
		"max_attempts", w.maxAttempts)

	w.tick(ctx)
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	prune := time.NewTicker(1 * time.Hour)
	defer prune.Stop()
	// Stale-recovery sweeper runs every 5 min so a crashed replica's
	// `processing` claims roll back to `pending` quickly enough that
	// the customer doesn't notice. 10min staleness threshold is well
	// past any legitimate processing time (max 90s per the inbound
	// AI pipeline).
	recover := time.NewTicker(5 * time.Minute)
	defer recover.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("wa webhook outbox worker stopping")
			return
		case <-t.C:
			w.tick(ctx)
		case <-prune.C:
			w.prune(ctx)
		case <-recover.C:
			w.recoverStale(ctx)
		}
	}
}

// recoverStale rolls back rows stuck in 'processing' for >10min.
// Best-effort; a failure here just means the next tick retries
// the recovery.
func (w *WAWebhookOutboxWorker) recoverStale(ctx context.Context) {
	rec, ok := w.outbox.(domain.WAWebhookOutboxStaleRecover)
	if !ok {
		return
	}
	recovered, err := rec.RecoverStale(ctx, 10)
	if err != nil {
		slog.Warn("wa outbox: stale recovery failed", "error", err)
		return
	}
	if recovered > 0 {
		slog.Info("wa outbox: recovered stale processing rows", "count", recovered)
	}
}

func (w *WAWebhookOutboxWorker) tick(ctx context.Context) {
	rows, err := w.outbox.ListPending(ctx, w.batchSize)
	if err != nil {
		slog.Warn("wa outbox: list pending failed", "error", err)
		return
	}
	if w.pendingGauge != nil {
		// Reports the size of THIS tick's batch — capped at batchSize,
		// so a sustained gauge reading at the cap means the worker is
		// falling behind and ListPending keeps maxing out.
		w.pendingGauge(len(rows))
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return
		}
		w.processRow(ctx, row)
	}
}

func (w *WAWebhookOutboxWorker) processRow(ctx context.Context, row *domain.WAWebhookEvent) {
	processed, err := w.process(ctx, row.Payload)
	if err != nil {
		// Transient failure. Mark + leave pending; next tick retries
		// after the 30s cooldown enforced by ListPending.
		_ = w.outbox.MarkFailed(ctx, row.ID, err.Error())
		// `attempts` came from BEFORE we incremented; +1 here mirrors
		// the SQL update and decides whether we promote to dead.
		if row.Attempts+1 >= w.maxAttempts {
			_ = w.outbox.MarkDead(ctx, row.ID)
			w.alert("wa outbox row reached max_attempts (" +
				row.ID + "): " + err.Error())
		}
		return
	}
	if processed {
		_ = w.outbox.MarkProcessed(ctx, row.ID)
		return
	}
	_ = w.outbox.MarkSkipped(ctx, row.ID)
}

func (w *WAWebhookOutboxWorker) prune(ctx context.Context) {
	cutoff := time.Now().Add(-w.pruneAge)
	deleted, err := w.outbox.PruneProcessed(ctx, cutoff)
	if err != nil {
		// Non-fatal — failed prune just means the table grows a bit
		// larger this hour. Don't escalate to alerter.
		if !errors.Is(err, context.Canceled) {
			slog.Warn("wa outbox: prune failed", "error", err)
		}
		return
	}
	if deleted > 0 {
		slog.Info("wa outbox: pruned old rows", "deleted", deleted, "cutoff", cutoff)
	}
}

func (w *WAWebhookOutboxWorker) alert(msg string) {
	if w.alerter == nil {
		return
	}
	w.alerter.Record("whatsapp_webhook_outbox", msg)
}
