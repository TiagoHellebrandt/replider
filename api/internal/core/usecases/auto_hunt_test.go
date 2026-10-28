package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
)

// An account held ninety products, a whole state's worth of matching hospitals
// in the database, and ONE lead. The first version of this sweep did not fix
// that: it asked the CNES dictionary about the six LONGEST words in the
// catalogue, and on a real one the longest words are "antitransbordamento",
// "multiparamétricos", "bilirrubinômetro" — none of which the registry knows.
// It reported, truthfully and uselessly, that it had found nothing, while
// "incubadora" and "ventilador" sat further down the list unasked.
//
// It now works from the same list the home screen's tile is built from, which
// resolves the WHOLE catalogue in one query. These pin that, and the two lines
// the night must not cross: it never drafts, and it never floods the funnel.

type fakeTargets struct {
	holders      []domain.CNESEquipmentHolder
	total        int
	askedLimit   int
	askedOffsets []int
	known        map[string]bool
	err          error
}

// Page-aware on purpose: a fake that answers the same rows at every offset
// cannot tell a sweep that walks its territory from one that reads the top
// of the list over and over, which is exactly the bug this paging exists to
// fix.
func (f *fakeTargets) ListPage(_ context.Context, _ string, offset, limit int) (UntouchedPage, error) {
	f.askedLimit = limit
	f.askedOffsets = append(f.askedOffsets, offset)
	if f.err != nil {
		return UntouchedPage{}, f.err
	}
	if offset >= len(f.holders) {
		return UntouchedPage{Total: f.total}, nil
	}
	end := offset + limit
	if end > len(f.holders) {
		end = len(f.holders)
	}
	window := f.holders[offset:end]
	// Scanned counts the window the park returned; Holders is what is left
	// after the known ones drop out. The `known` set is what makes the two
	// differ, which is the case the walk has to survive.
	out := UntouchedPage{Total: f.total, Scanned: len(window)}
	for _, h := range window {
		if f.known[h.CNES] {
			continue
		}
		out.Holders = append(out.Holders, h)
	}
	return out, nil
}

type fakeCursors struct {
	start int
	saved int
	err   error
}

func (f *fakeCursors) HuntCursor(context.Context, string) (int, error) {
	return f.start, f.err
}

func (f *fakeCursors) SetHuntCursor(_ context.Context, _ string, offset int) error {
	f.saved = offset
	return nil
}

type fakeEnsurer struct {
	ensured  []string
	existing map[string]bool
	err      error
}

func (f *fakeEnsurer) EnsureLead(_ context.Context, _, cnes string) (*domain.Lead, bool, error) {
	f.ensured = append(f.ensured, cnes)
	if f.err != nil {
		return nil, false, f.err
	}
	return &domain.Lead{ID: "lead-" + cnes}, f.existing[cnes], nil
}

type fakeCandidates struct {
	list []domain.SignalWatchCandidate
	err  error
}

func (f *fakeCandidates) AccountsForAutoHunt(context.Context) ([]domain.SignalWatchCandidate, error) {
	return f.list, f.err
}

func holders(n int) []domain.CNESEquipmentHolder {
	out := make([]domain.CNESEquipmentHolder, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, domain.CNESEquipmentHolder{
			CNES: string(rune('a'+i%26)) + string(rune('0'+i/26)), Total: n - i,
		})
	}
	return out
}

func TestAutoHunt_TurnsUntouchedTerritoryIntoLeads(t *testing.T) {
	targets := &fakeTargets{holders: holders(3), total: 132}
	ensurer := &fakeEnsurer{}
	uc := NewAutoHuntUseCase(&fakeCandidates{}, targets, ensurer)

	res, err := uc.RunForAccount(context.Background(), "acc1")
	if err != nil {
		t.Fatalf("RunForAccount: %v", err)
	}
	if res.Created != 3 || res.Found != 3 {
		t.Fatalf("created=%d found=%d", res.Created, res.Found)
	}
	if len(ensurer.ensured) != 3 {
		t.Fatalf("ensured %v", ensurer.ensured)
	}
}

// The regression that made the first version useless: the words came from a
// longest-first sample of the catalogue. It now asks for the resolved list,
// and it asks for MORE than it will create — the biggest parks are often
// customers already, and a run that stopped at fifteen candidates would end
// the night early.
func TestAutoHunt_AsksForMoreCandidatesThanItWillCreate(t *testing.T) {
	targets := &fakeTargets{holders: holders(1)}
	uc := NewAutoHuntUseCase(&fakeCandidates{}, targets, &fakeEnsurer{})

	if _, err := uc.RunForAccount(context.Background(), "acc1"); err != nil {
		t.Fatal(err)
	}
	if targets.askedLimit <= maxNewLeadsPerRun {
		t.Fatalf("asked for %d candidates but may create %d",
			targets.askedLimit, maxNewLeadsPerRun)
	}
}

// A funnel nobody can work through in a week is a funnel that gets abandoned.
func TestAutoHunt_CapsOneNightsLeads(t *testing.T) {
	targets := &fakeTargets{holders: holders(40)}
	ensurer := &fakeEnsurer{}
	uc := NewAutoHuntUseCase(&fakeCandidates{}, targets, ensurer)

	res, _ := uc.RunForAccount(context.Background(), "acc1")
	if res.Created != maxNewLeadsPerRun {
		t.Fatalf("created %d, want the cap of %d", res.Created, maxNewLeadsPerRun)
	}
	if len(ensurer.ensured) != maxNewLeadsPerRun {
		t.Fatalf("kept working past the cap: %d calls", len(ensurer.ensured))
	}
}

// A hospital already in the funnel is not new ground. Counting it would make
// every night look productive.
func TestAutoHunt_DoesNotCountLeadsThatAlreadyExisted(t *testing.T) {
	hs := holders(2)
	targets := &fakeTargets{holders: hs}
	ensurer := &fakeEnsurer{existing: map[string]bool{hs[0].CNES: true}}
	uc := NewAutoHuntUseCase(&fakeCandidates{}, targets, ensurer)

	res, _ := uc.RunForAccount(context.Background(), "acc1")
	if res.Created != 1 {
		t.Fatalf("created=%d, want 1", res.Created)
	}
}

// The night never drafts: the only lead-side port it has is the one that
// stops at the lead. A drafter would need a new field, and this is where that
// shows up.
func TestAutoHunt_NeverDrafts(t *testing.T) {
	var _ leadEnsurer = (*fakeEnsurer)(nil)
	uc := NewAutoHuntUseCase(&fakeCandidates{}, &fakeTargets{}, &fakeEnsurer{})
	if uc.leads == nil {
		t.Fatal("unexpected shape")
	}
}

// One account's failure must never stop the sweep.
func TestAutoHunt_RunContinuesPastAFailingAccount(t *testing.T) {
	uc := NewAutoHuntUseCase(
		&fakeCandidates{list: []domain.SignalWatchCandidate{
			{AccountID: "bad", UF: "MT"}, {AccountID: "good", UF: "MT"},
		}},
		&failingThenOK{},
		&fakeEnsurer{},
	)
	got := uc.Run(context.Background())
	if len(got) != 1 || got[0].AccountID != "good" {
		t.Fatalf("sweep stopped at the first failure: %+v", got)
	}
}

type failingThenOK struct{}

func (f *failingThenOK) ListPage(_ context.Context, accountID string, offset, _ int) (UntouchedPage, error) {
	if accountID == "bad" {
		return UntouchedPage{}, errors.New("cnes down")
	}
	if offset > 0 {
		return UntouchedPage{Total: 1}, nil
	}
	return UntouchedPage{Holders: holders(1), Total: 1, Scanned: 1}, nil
}

// The ceiling.
//
// The sweep read one page from the top of the ranking and stopped, every
// night, for the life of the account. Production on 2026-09-17: 60 hospitals
// in the funnel, 2.818 matching establishments in the territory, and nine
// consecutive nights of `found=52 created=0` — the other 97,9% of the ground
// was unreachable by construction, not by priority.
func TestAutoHunt_WalksPastTheFirstPage(t *testing.T) {
	// Two pages' worth, with the WHOLE first page already in the funnel — so
	// page one comes back scanned-but-empty, which is exactly the state this
	// account is in the moment paging ships. An empty work list read as "end
	// of territory" stops the walk on page one and rebuilds the ceiling.
	hs := holders(maxHuntCandidates + 5)
	known := map[string]bool{}
	for _, h := range hs[:maxHuntCandidates] {
		known[h.CNES] = true
	}
	targets := &fakeTargets{holders: hs, known: known}
	ensurer := &fakeEnsurer{}
	uc := NewAutoHuntUseCase(&fakeCandidates{}, targets, ensurer)

	res, err := uc.RunForAccount(context.Background(), "acc1")
	if err != nil {
		t.Fatalf("RunForAccount: %v", err)
	}
	if res.Created != 5 {
		t.Fatalf("created=%d, want the 5 hospitals past the first page", res.Created)
	}
	if len(targets.askedOffsets) < 2 || targets.askedOffsets[1] != maxHuntCandidates {
		t.Fatalf("never read past the top: offsets %v", targets.askedOffsets)
	}
}

// A page full of hospitals we already have offers no work and is NOT the end
// of the territory.
//
// The distinction is the difference between a sweep and the ceiling it was
// built to remove: on the day paging ships, page one is entirely the ground
// the old sweep already consumed, so "no work here" and "nothing left
// anywhere" arrive as the same empty list on the very first read.
func TestAutoHunt_AnEmptyPageIsNotAnEmptyTerritory(t *testing.T) {
	hs := holders(maxHuntCandidates + 1)
	known := map[string]bool{}
	for _, h := range hs[:maxHuntCandidates] {
		known[h.CNES] = true
	}
	targets := &fakeTargets{holders: hs, known: known}
	uc := NewAutoHuntUseCase(&fakeCandidates{}, targets, &fakeEnsurer{})

	res, err := uc.RunForAccount(context.Background(), "acc1")
	if err != nil {
		t.Fatalf("RunForAccount: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("created=%d; the walk stopped on a page that was merely worked out",
			res.Created)
	}
}

// Tonight's reading position is where tomorrow starts. Without it every night
// re-decides the same hospitals and the sweep never leaves the doorstep.
func TestAutoHunt_ResumesWhereTheLastNightStopped(t *testing.T) {
	hs := holders(10)
	cursors := &fakeCursors{start: 7}
	targets := &fakeTargets{holders: hs}
	ensurer := &fakeEnsurer{}
	uc := NewAutoHuntUseCase(&fakeCandidates{}, targets, ensurer)
	uc.SetCursorStore(cursors)

	res, _ := uc.RunForAccount(context.Background(), "acc1")
	if res.Created != 3 {
		t.Fatalf("created=%d, want the 3 left after offset 7", res.Created)
	}
	if targets.askedOffsets[0] != 7 {
		t.Fatalf("started at %d, not at the stored cursor", targets.askedOffsets[0])
	}
	if cursors.saved != 10 {
		t.Fatalf("saved cursor %d, want the end of what it read", cursors.saved)
	}
}

// The end of the ranking is not the end of the product: the park is
// republished monthly, and a hospital a name collision swallowed deserves
// another pass. A cursor past the end restarts at the top.
func TestAutoHunt_WrapsWhenTheTerritoryRunsOut(t *testing.T) {
	targets := &fakeTargets{holders: holders(2)}
	cursors := &fakeCursors{start: 999}
	uc := NewAutoHuntUseCase(&fakeCandidates{}, targets, &fakeEnsurer{})
	uc.SetCursorStore(cursors)

	res, _ := uc.RunForAccount(context.Background(), "acc1")
	if res.Created != 2 {
		t.Fatalf("created=%d, want the territory read again from the top", res.Created)
	}
	if len(targets.askedOffsets) < 2 || targets.askedOffsets[1] != 0 {
		t.Fatalf("did not wrap: offsets %v", targets.askedOffsets)
	}
}

// A page that failed to read has not been worked. Advancing over it would
// lose that ground until the next full lap.
func TestAutoHunt_AFailedPageDoesNotAdvanceTheCursor(t *testing.T) {
	cursors := &fakeCursors{start: 60, saved: -1}
	uc := NewAutoHuntUseCase(&fakeCandidates{},
		&fakeTargets{err: errors.New("cnes down")}, &fakeEnsurer{})
	uc.SetCursorStore(cursors)

	if _, err := uc.RunForAccount(context.Background(), "acc1"); err == nil {
		t.Fatal("want the read failure surfaced")
	}
	if cursors.saved != -1 {
		t.Fatalf("saved %d; a failed page must leave the cursor alone", cursors.saved)
	}
}

// Unwired seams do nothing rather than panic.
func TestAutoHunt_UnwiredIsANoOp(t *testing.T) {
	if got := (&AutoHuntUseCase{}).Run(context.Background()); got != nil {
		t.Fatalf("got %v", got)
	}
}
