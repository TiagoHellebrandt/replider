package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// The overnight worker (K2).
//
// Two things matter and neither is the drafting itself: the night is CLAIMED
// before any allowance is spent (two pods tick in the same minute), and a run
// that fails halfway is still recorded — a morning where the salesperson cannot
// tell "nothing to draft" from "it broke" is the failure worth preventing.

type fakeOvernightRepo struct {
	targets []domain.OvernightDraftTarget
	listErr error

	claimed    map[string]bool
	claimErr   error
	claimCalls int

	saved     []*domain.OvernightDraftRun
	saveErr   error
	unseenRun *domain.OvernightDraftRun
}

func (f *fakeOvernightRepo) Settings(context.Context, string) (*domain.OvernightDraftSettings, error) {
	return nil, nil
}
func (f *fakeOvernightRepo) SaveSettings(context.Context, *domain.OvernightDraftSettings) error {
	return nil
}
func (f *fakeOvernightRepo) ListEnabled(context.Context) ([]domain.OvernightDraftTarget, error) {
	return f.targets, f.listErr
}

func (f *fakeOvernightRepo) ClaimNight(_ context.Context, accountID string, _ time.Time) (bool, error) {
	f.claimCalls++
	if f.claimErr != nil {
		return false, f.claimErr
	}
	if f.claimed == nil {
		f.claimed = map[string]bool{}
	}
	if f.claimed[accountID] {
		return false, nil
	}
	f.claimed[accountID] = true
	return true, nil
}

func (f *fakeOvernightRepo) SaveRun(_ context.Context, run *domain.OvernightDraftRun) error {
	f.saved = append(f.saved, run)
	return f.saveErr
}

func (f *fakeOvernightRepo) LatestUnseen(context.Context, string) (*domain.OvernightDraftRun, error) {
	return f.unseenRun, nil
}
func (f *fakeOvernightRepo) MarkSeen(context.Context, string, string, time.Time) error { return nil }

type fakeNightlyDrafter struct {
	drafted, optedOut, budget, failed int
	stoppedReason                     string
	err                               error
	calls                             int
	gotLimit                          int
	gotFunnel                         string
}

func (f *fakeNightlyDrafter) DraftBatchForAccount(_ context.Context, _, funnelID string, limit int) (*domain.BatchDraftOutcome, error) {
	f.calls++
	f.gotLimit, f.gotFunnel = limit, funnelID
	// The real one returns the partial counters ALONGSIDE the error — a
	// funnel that blew up does not erase the work the others did. A fake
	// that returned nil here would let the worker regress to losing them.
	return &domain.BatchDraftOutcome{
		Drafted:         f.drafted,
		SkippedOptedOut: f.optedOut,
		SkippedBudget:   f.budget,
		SkippedError:    f.failed,
		StoppedReason:   f.stoppedReason,
	}, f.err
}

func overnightTarget(accountID string, hour int, cap int) domain.OvernightDraftTarget {
	return domain.OvernightDraftTarget{
		Settings: domain.OvernightDraftSettings{
			AccountID: accountID, Enabled: true, RunHour: hour, NightlyCap: cap,
		},
		Timezone: "UTC",
	}
}

func newOvernightWorker(repo *fakeOvernightRepo, drafter *fakeNightlyDrafter, at time.Time) *OvernightDraftWorker {
	w := NewOvernightDraftWorker(repo, drafter)
	w.now = func() time.Time { return at }
	return w
}

func TestOvernightWorker_DraftsAtTheAccountsHour(t *testing.T) {
	repo := &fakeOvernightRepo{targets: []domain.OvernightDraftTarget{
		overnightTarget("acc", 3, 40),
	}}
	drafter := &fakeNightlyDrafter{drafted: 40}
	w := newOvernightWorker(repo, drafter, time.Date(2026, 7, 30, 3, 10, 0, 0, time.UTC))

	w.RunOnce(context.Background())

	if drafter.calls != 1 {
		t.Fatalf("drafter calls = %d, want 1", drafter.calls)
	}
	if drafter.gotLimit != 40 {
		t.Fatalf("limit = %d, want the nightly cap", drafter.gotLimit)
	}
	if len(repo.saved) != 1 || repo.saved[0].Drafted != 40 {
		t.Fatalf("saved = %+v, want one run of 40", repo.saved)
	}
}

func TestOvernightWorker_DoesNothingOutsideTheHour(t *testing.T) {
	repo := &fakeOvernightRepo{targets: []domain.OvernightDraftTarget{
		overnightTarget("acc", 3, 40),
	}}
	drafter := &fakeNightlyDrafter{}
	w := newOvernightWorker(repo, drafter, time.Date(2026, 7, 30, 14, 0, 0, 0, time.UTC))

	w.RunOnce(context.Background())

	// Two in the afternoon is not overnight, and drafting then spends the
	// allowance while the salesperson is at their desk and could have asked.
	if drafter.calls != 0 {
		t.Fatalf("drafted %d times outside the window", drafter.calls)
	}
	if repo.claimCalls != 0 {
		t.Fatal("must not even claim outside the window")
	}
}

func TestOvernightWorker_ClaimsBeforeSpendingAnything(t *testing.T) {
	repo := &fakeOvernightRepo{
		targets: []domain.OvernightDraftTarget{overnightTarget("acc", 3, 40)},
		// Another pod already claimed this night.
		claimed: map[string]bool{"acc": true},
	}
	drafter := &fakeNightlyDrafter{drafted: 40}
	w := newOvernightWorker(repo, drafter, time.Date(2026, 7, 30, 3, 10, 0, 0, time.UTC))

	w.RunOnce(context.Background())

	// The loser of the claim spends NOTHING. Claiming after drafting would have
	// both pods spend a night's allowance.
	if drafter.calls != 0 {
		t.Fatalf("drafted %d times after losing the claim", drafter.calls)
	}
	if len(repo.saved) != 0 {
		t.Fatal("a lost claim must not record a run")
	}
}

func TestOvernightWorker_AFailedRunIsStillRecorded(t *testing.T) {
	repo := &fakeOvernightRepo{targets: []domain.OvernightDraftTarget{
		overnightTarget("acc", 3, 40),
	}}
	// Half the work done, then a failure.
	drafter := &fakeNightlyDrafter{drafted: 12, failed: 3, err: errors.New("provider down")}
	w := newOvernightWorker(repo, drafter, time.Date(2026, 7, 30, 3, 10, 0, 0, time.UTC))

	w.RunOnce(context.Background())

	// A run that failed halfway and left no trace is a morning where the
	// salesperson cannot tell "nothing to draft" from "it broke".
	if len(repo.saved) != 1 {
		t.Fatalf("saved %d runs, want the partial one recorded", len(repo.saved))
	}
	if repo.saved[0].Drafted != 12 || repo.saved[0].SkippedError != 3 {
		t.Fatalf("run = %+v, want the partial counters", repo.saved[0])
	}
	if repo.saved[0].FinishedAt == nil {
		t.Fatal("a recorded run must be stamped finished, even a failed one")
	}
}

func TestOvernightWorker_ABudgetCutIsCarriedIntoTheRun(t *testing.T) {
	repo := &fakeOvernightRepo{targets: []domain.OvernightDraftTarget{
		overnightTarget("acc", 3, 100),
	}}
	drafter := &fakeNightlyDrafter{drafted: 40, budget: 60}
	w := newOvernightWorker(repo, drafter, time.Date(2026, 7, 30, 3, 10, 0, 0, time.UTC))

	w.RunOnce(context.Background())

	// The briefing has to be able to explain why 100 became 40.
	if repo.saved[0].SkippedBudget != 60 {
		t.Fatalf("run = %+v, want the budget cut carried through", repo.saved[0])
	}
}

func TestOvernightWorker_OneAccountsFailureDoesNotStopTheNext(t *testing.T) {
	repo := &fakeOvernightRepo{targets: []domain.OvernightDraftTarget{
		overnightTarget("acc-1", 3, 40),
		overnightTarget("acc-2", 3, 40),
	}}
	drafter := &fakeNightlyDrafter{drafted: 5}
	w := newOvernightWorker(repo, drafter, time.Date(2026, 7, 30, 3, 10, 0, 0, time.UTC))

	w.RunOnce(context.Background())

	if drafter.calls != 2 {
		t.Fatalf("drafter calls = %d, want one per account", drafter.calls)
	}
	if len(repo.saved) != 2 {
		t.Fatalf("saved %d runs, want 2", len(repo.saved))
	}
}

func TestOvernightWorker_UnwiredDoesNotPanic(t *testing.T) {
	// A deployment that has not enabled the feature must boot, not crash on the
	// first tick.
	NewOvernightDraftWorker(nil, nil).RunOnce(context.Background())

	var nilWorker *OvernightDraftWorker
	nilWorker.Start(context.Background())
}

func TestOvernightWorker_AListFailureIsLoggedNotFatal(t *testing.T) {
	repo := &fakeOvernightRepo{listErr: errors.New("timeout")}
	drafter := &fakeNightlyDrafter{}
	w := newOvernightWorker(repo, drafter, time.Date(2026, 7, 30, 3, 10, 0, 0, time.UTC))

	w.RunOnce(context.Background())

	if drafter.calls != 0 {
		t.Fatal("nothing to draft when the list could not be read")
	}
}

// Nine nights in a row, production drafted nothing because the account had
// no catalogue — and the morning card kept showing the last SUCCESSFUL night
// as if it were today's, because a run with drafted=0 was filtered out of
// the briefing entirely. A blocked night has to be recorded AND worth
// showing, or silence reads as success.
func TestOvernightWorker_ABlockedNightIsRecordedAndWorthShowing(t *testing.T) {
	repo := &fakeOvernightRepo{targets: []domain.OvernightDraftTarget{
		overnightTarget("acc", 3, 40),
	}}
	drafter := &fakeNightlyDrafter{
		failed:        63,
		stoppedReason: "conta sem catálogo",
	}
	w := newOvernightWorker(repo, drafter, time.Date(2026, 9, 9, 3, 10, 0, 0, time.UTC))

	w.RunOnce(context.Background())

	if len(repo.saved) != 1 {
		t.Fatalf("saved %d runs, want the blocked one recorded", len(repo.saved))
	}
	run := repo.saved[0]
	if run.StoppedReason != "conta sem catálogo" {
		t.Fatalf("reason = %q, want the cause carried onto the run", run.StoppedReason)
	}
	if !run.Blocked() {
		t.Fatal("a run with a stopped reason must report itself blocked")
	}
	if !run.Worth() {
		t.Fatal("a blocked night must be worth telling somebody about")
	}
}

// The other direction: a night that drafted nothing because there was nobody
// left to write to has NOT failed, and must not grow a scary card.
func TestOvernightWorker_AQuietNightIsNotABlockedOne(t *testing.T) {
	repo := &fakeOvernightRepo{targets: []domain.OvernightDraftTarget{
		overnightTarget("acc", 3, 40),
	}}
	w := newOvernightWorker(repo, &fakeNightlyDrafter{}, time.Date(2026, 9, 9, 3, 10, 0, 0, time.UTC))

	w.RunOnce(context.Background())

	if len(repo.saved) != 1 {
		t.Fatalf("saved %d runs", len(repo.saved))
	}
	if repo.saved[0].Blocked() || repo.saved[0].Worth() {
		t.Fatalf("run = %+v, want a quiet night to stay quiet", repo.saved[0])
	}
}
