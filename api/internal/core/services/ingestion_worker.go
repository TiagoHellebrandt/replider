package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// IngestionJobProcessor is the minimal contract the worker needs from
// the use-case layer: claim pending jobs and drive each to terminal
// state. Defined here (inside services) so the worker can live next to
// campaign_worker.go without importing usecases and creating a cycle.
type IngestionJobProcessor interface {
	// Process drives the given job to completed/failed, writing
	// chunks + updating the job row. Implementations must be
	// idempotent w.r.t. the job row (the worker retry contract is
	// "one attempt per claim"; re-running never double-counts).
	Process(ctx context.Context, job *domain.IngestionJob) *domain.IngestionJob
}

// IngestionJobClaimer is the repo port the worker polls. Matches
// domain.IngestionJobRepository's ClaimNext for interface segregation.
type IngestionJobClaimer interface {
	ClaimNext(ctx context.Context) (*domain.IngestionJob, error)
}

// IngestionWorker is a single-goroutine poll loop that drains the
// ingestion_jobs queue one job at a time. Poll interval is 3 s — fine
// for the single-pod MVP; once we scale out or add SLAs a Redis /
// NATS queue will replace the polling.
type IngestionWorker struct {
	claimer      IngestionJobClaimer
	processor    IngestionJobProcessor
	pollInterval time.Duration
	running      atomic.Bool
}

func NewIngestionWorker(claimer IngestionJobClaimer, processor IngestionJobProcessor) *IngestionWorker {
	return &IngestionWorker{
		claimer:      claimer,
		processor:    processor,
		pollInterval: 3 * time.Second,
	}
}

// Start runs the poll loop until ctx is cancelled. Safe to call once;
// subsequent calls are no-ops to prevent double-start in main.go.
func (w *IngestionWorker) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		slog.Warn("ingestion worker already running, ignoring Start()")
		return
	}
	defer w.running.Store(false)

	slog.Info("ingestion worker started", "poll_interval", w.pollInterval)

	// Drain anything left over from a previous deploy before the first
	// tick so a restart immediately resumes pending work.
	w.drain(ctx)

	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("ingestion worker stopping")
			return
		case <-t.C:
			w.drain(ctx)
		}
	}
}

// drain loops claiming jobs until the queue returns empty. One tick
// can process several jobs — the ticker is a liveness heartbeat, not
// a concurrency cap. Each job is processed synchronously inside the
// worker goroutine; a slow embedder doesn't block other jobs at the
// *API* level because the HTTP handler already returned `pending`.
func (w *IngestionWorker) drain(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		job, err := w.claimer.ClaimNext(ctx)
		if err != nil {
			slog.Error("ingestion worker: claim failed", "error", err)
			return
		}
		if job == nil {
			return // queue empty — sleep until next tick
		}
		slog.Info("ingestion: processing job",
			"job_id", job.ID, "source_id", job.KnowledgeSourceID, "type", job.Type)
		result := w.processor.Process(ctx, job)
		slog.Info("ingestion: finished job",
			"job_id", result.ID,
			"status", result.Status,
			"chunks_created", result.ChunksCreated,
			"error_message", result.ErrorMessage,
		)
	}
}
