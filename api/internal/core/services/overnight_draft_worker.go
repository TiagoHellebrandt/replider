package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// Overnight drafting worker (§6.17, K2).
//
// Fills the approval queue while nobody is watching, so the salesperson opens the
// app to work already done rather than to an empty screen.

// OvernightDrafter is the narrow slice of the approach use case this needs.
//
// Deliberately the SAME entry point the human-triggered batch uses: the opt-out
// filtering, the strategy attribution and the per-plan spend cap all live behind
// it, and a second path into drafting would be a second place for those to be
// forgotten.
type OvernightDrafter interface {
	DraftBatchForAccount(ctx context.Context, accountID, funnelID string, limit int) (*domain.BatchDraftOutcome, error)
}

// OvernightBatchSubmitter is the 50%-off path: the same night's prompts go
// out as one provider batch job and a collector lands the answers before
// morning. Optional — accounts whose provider cannot batch fall back to the
// synchronous drafter above.
type OvernightBatchSubmitter interface {
	SubmitOvernightBatch(ctx context.Context, accountID, funnelID string, limit int) (submitted, optedOut, budget int, err error)
}

// OvernightDraftWorker runs the nightly pass.
type OvernightDraftWorker struct {
	repo    domain.OvernightDraftRepository
	drafter OvernightDrafter

	// batch, when set (AI_BATCH_DRAFTS=true), is tried FIRST each night;
	// any submit failure falls back to the synchronous drafter, so batch
	// being down never costs a night of drafts — only the discount.
	batch OvernightBatchSubmitter

	// tick is how often the schedule is evaluated. Ten minutes: the schedule is
	// stated in whole hours, so anything finer only costs queries — and anything
	// coarser than an hour would skip a run entirely.
	tick time.Duration
	now  func() time.Time
}

func NewOvernightDraftWorker(
	repo domain.OvernightDraftRepository, drafter OvernightDrafter,
) *OvernightDraftWorker {
	return &OvernightDraftWorker{
		repo:    repo,
		drafter: drafter,
		tick:    10 * time.Minute,
		now:     time.Now,
	}
}

// SetBatchSubmitter turns the batch-first path on.
func (w *OvernightDraftWorker) SetBatchSubmitter(b OvernightBatchSubmitter) {
	w.batch = b
}

// Start runs the loop until ctx is done.
func (w *OvernightDraftWorker) Start(ctx context.Context) {
	if w == nil || w.repo == nil || w.drafter == nil {
		slog.Info("overnight draft worker not started (unwired)")
		return
	}
	slog.Info("overnight draft worker started", "tick", w.tick)

	ticker := time.NewTicker(w.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("overnight draft worker stopped")
			return
		case <-ticker.C:
			w.RunOnce(ctx)
		}
	}
}

// RunOnce evaluates every opted-in account and drafts for the ones whose local
// hour has come. Exported so the tests drive it without a ticker.
func (w *OvernightDraftWorker) RunOnce(ctx context.Context) {
	// Guarded here as well as in Start: RunOnce is the exported entry point, and
	// a deployment that has not wired the feature must be a no-op rather than a
	// panic on the first tick.
	if w == nil || w.repo == nil || w.drafter == nil {
		return
	}
	targets, err := w.repo.ListEnabled(ctx)
	if err != nil {
		slog.Error("overnight draft: could not list enabled accounts", "error", err)
		return
	}

	now := w.now()
	for _, target := range targets {
		if !target.Settings.DueTonight(now, target.Timezone) {
			continue
		}
		w.runForAccount(ctx, target, now)
	}
}

// runForAccount claims the night and drafts.
func (w *OvernightDraftWorker) runForAccount(
	ctx context.Context, target domain.OvernightDraftTarget, now time.Time,
) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("overnight draft: panic while drafting",
				"account_id", target.Settings.AccountID, "recover", r)
		}
	}()

	// The account's local date, which is what the night is named after and what
	// the claim is keyed on. Midnight-local so the stored DATE is unambiguous.
	local := now.In(domain.LoadLocation(target.Timezone))
	localDate := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)

	// Claim BEFORE drafting. Two pods ticking in the same minute both reach
	// here; exactly one wins the conditional update, and the loser spends
	// nothing. Claiming afterwards would have both of them draft first.
	claimed, err := w.repo.ClaimNight(ctx, target.Settings.AccountID, localDate)
	if err != nil {
		slog.Error("overnight draft: could not claim the night",
			"account_id", target.Settings.AccountID, "error", err)
		return
	}
	if !claimed {
		return
	}

	run := &domain.OvernightDraftRun{
		AccountID: target.Settings.AccountID,
		RanOn:     localDate,
		StartedAt: now,
	}

	// Its own deadline, detached from the tick: drafting forty approaches is
	// forty provider calls, and the next tick must not cancel a run in progress.
	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Minute)
	defer cancel()

	// Batch first when wired: one submit call instead of forty generations,
	// at half the token price. The submitted count is recorded as the
	// night's Drafted — the collector lands the texts before morning, and a
	// double-count cannot happen because the collector refuses any lead the
	// synchronous path (or a human) already drafted. Any submit error —
	// unsupported provider, job already in flight, provider down — falls
	// back to the synchronous pass, so batch never costs a night.
	if w.batch != nil {
		submitted, optedOut, budget, err := w.batch.SubmitOvernightBatch(
			runCtx, target.Settings.AccountID, target.Settings.FunnelID, target.Settings.NightlyCap)
		if err == nil {
			run.Drafted = submitted
			run.SkippedOptedOut = optedOut
			run.SkippedBudget = budget
			finished := w.now()
			run.FinishedAt = &finished
			if err := w.repo.SaveRun(runCtx, run); err != nil {
				slog.Error("overnight draft: could not record the batch run",
					"account_id", target.Settings.AccountID, "error", err)
				return
			}
			slog.Info("overnight draft: batch submitted",
				"account_id", target.Settings.AccountID, "submitted", submitted,
				"skipped_budget", budget)
			return
		}
		slog.Warn("overnight draft: batch submit failed, running synchronously",
			"account_id", target.Settings.AccountID, "error", err)
	}

	res, err := w.drafter.DraftBatchForAccount(
		runCtx, target.Settings.AccountID, target.Settings.FunnelID, target.Settings.NightlyCap)
	if err != nil {
		// The night is still recorded, with whatever it managed. A run that
		// failed halfway and left no trace is a morning where the salesperson
		// cannot tell "nothing to draft" from "it broke".
		slog.Error("overnight draft: run failed",
			"account_id", target.Settings.AccountID, "error", err)
	}
	if res == nil {
		// Nothing came back at all — the night is still recorded, and the
		// reason has to be SOMETHING the morning card can render.
		res = &domain.BatchDraftOutcome{}
		if err != nil {
			res.StoppedReason = err.Error()
		}
	}

	run.Drafted = res.Drafted
	run.SkippedOptedOut = res.SkippedOptedOut
	run.SkippedBudget = res.SkippedBudget
	run.SkippedError = res.SkippedError
	run.StoppedReason = res.StoppedReason
	finished := w.now()
	run.FinishedAt = &finished

	if err := w.repo.SaveRun(runCtx, run); err != nil {
		slog.Error("overnight draft: could not record the run",
			"account_id", target.Settings.AccountID, "error", err)
		return
	}
	if run.Blocked() {
		slog.Warn("overnight draft: night blocked",
			"account_id", target.Settings.AccountID,
			"skipped_error", res.SkippedError, "reason", run.StoppedReason)
		return
	}
	slog.Info("overnight draft: night complete",
		"account_id", target.Settings.AccountID,
		"drafted", res.Drafted, "skipped_budget", res.SkippedBudget,
		"skipped_error", res.SkippedError)
}
