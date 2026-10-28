package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

type fakeBatchSubmitter struct {
	submitted, optedOut, budget int
	err                         error
	calls                       int
	gotFunnel                   string
	gotLimit                    int
}

func (f *fakeBatchSubmitter) SubmitOvernightBatch(_ context.Context, _, funnelID string, limit int) (int, int, int, error) {
	f.calls++
	f.gotFunnel, f.gotLimit = funnelID, limit
	return f.submitted, f.optedOut, f.budget, f.err
}

func TestOvernightWorker_BatchWinsWhenItSubmits(t *testing.T) {
	repo := &fakeOvernightRepo{targets: []domain.OvernightDraftTarget{
		overnightTarget("acc", 3, 40),
	}}
	drafter := &fakeNightlyDrafter{drafted: 40}
	batch := &fakeBatchSubmitter{submitted: 40, budget: 2}
	w := newOvernightWorker(repo, drafter, time.Date(2026, 7, 30, 3, 10, 0, 0, time.UTC))
	w.SetBatchSubmitter(batch)

	w.RunOnce(context.Background())

	if batch.calls != 1 || batch.gotLimit != 40 {
		t.Fatalf("batch calls=%d limit=%d, want 1/40", batch.calls, batch.gotLimit)
	}
	if drafter.calls != 0 {
		t.Fatalf("sync drafter ran %d times, batch should have covered the night", drafter.calls)
	}
	if len(repo.saved) != 1 || repo.saved[0].Drafted != 40 || repo.saved[0].SkippedBudget != 2 {
		t.Fatalf("saved = %+v, want one run of 40 with 2 budget skips", repo.saved)
	}
}

func TestOvernightWorker_BatchFailureFallsBackToSync(t *testing.T) {
	repo := &fakeOvernightRepo{targets: []domain.OvernightDraftTarget{
		overnightTarget("acc", 3, 40),
	}}
	drafter := &fakeNightlyDrafter{drafted: 40}
	batch := &fakeBatchSubmitter{err: domain.ErrBatchUnsupported}
	w := newOvernightWorker(repo, drafter, time.Date(2026, 7, 30, 3, 10, 0, 0, time.UTC))
	w.SetBatchSubmitter(batch)

	w.RunOnce(context.Background())

	if batch.calls != 1 {
		t.Fatalf("batch calls = %d, want 1", batch.calls)
	}
	if drafter.calls != 1 {
		t.Fatalf("sync fallback did not run (calls=%d)", drafter.calls)
	}
	if len(repo.saved) != 1 || repo.saved[0].Drafted != 40 {
		t.Fatalf("saved = %+v, want the sync run recorded", repo.saved)
	}
}

func TestOvernightWorker_BatchProviderDownStillDraftsTheNight(t *testing.T) {
	repo := &fakeOvernightRepo{targets: []domain.OvernightDraftTarget{
		overnightTarget("acc", 3, 10),
	}}
	drafter := &fakeNightlyDrafter{drafted: 10}
	batch := &fakeBatchSubmitter{err: errors.New("batch submit: status 500")}
	w := newOvernightWorker(repo, drafter, time.Date(2026, 7, 30, 3, 10, 0, 0, time.UTC))
	w.SetBatchSubmitter(batch)

	w.RunOnce(context.Background())

	if drafter.calls != 1 || len(repo.saved) != 1 || repo.saved[0].Drafted != 10 {
		t.Fatalf("provider outage must not cost the night: drafter=%d saved=%+v", drafter.calls, repo.saved)
	}
}

func TestOvernightWorker_NoBatchSubmitterKeepsTheOldPath(t *testing.T) {
	repo := &fakeOvernightRepo{targets: []domain.OvernightDraftTarget{
		overnightTarget("acc", 3, 40),
	}}
	drafter := &fakeNightlyDrafter{drafted: 40}
	w := newOvernightWorker(repo, drafter, time.Date(2026, 7, 30, 3, 10, 0, 0, time.UTC))

	w.RunOnce(context.Background())

	if drafter.calls != 1 {
		t.Fatalf("sync path should run when batch is unwired (calls=%d)", drafter.calls)
	}
}
