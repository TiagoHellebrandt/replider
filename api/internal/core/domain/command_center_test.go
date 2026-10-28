package domain

import (
	"testing"
	"time"
)

// Command centre (I2).
//
// The ranking IS the feature: a "needs you" list of eleven equally-weighted rows
// is a list nobody reads. These tests pin the order of precedence, and pin that
// nothing on the list can be marked done without doing it.

var ccNow = time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)

func ccDay(offset int) *time.Time {
	d := ccNow.AddDate(0, 0, offset)
	return &d
}

func TestRankPendingActions_MoneyOutranksNoMoney(t *testing.T) {
	actions := []PendingAction{
		{Kind: PendingApproveDraft, TargetID: "draft", SinceAt: ccDay(-4)},
		{Kind: PendingSignatureViewed, TargetID: "sig", MoneyAtStakeCents: 90000},
	}

	RankPendingActions(actions, ccNow)

	// R$900 with a name on it beats a draft that has waited four days: the
	// screen promises impact, and staleness is only the tiebreak among rows
	// that carry no amount.
	if actions[0].Kind != PendingSignatureViewed {
		t.Fatalf("first = %s, want the one carrying money", actions[0].Kind)
	}
}

func TestRankPendingActions_BiggerMoneyFirst(t *testing.T) {
	actions := []PendingAction{
		{Kind: PendingConfirmSale, TargetID: "small", MoneyAtStakeCents: 90000},
		{Kind: PendingSignatureViewed, TargetID: "big", MoneyAtStakeCents: 10000000},
	}

	RankPendingActions(actions, ccNow)

	if actions[0].TargetID != "big" {
		t.Fatalf("first = %q, want the R$100.000 one", actions[0].TargetID)
	}
}

func TestRankPendingActions_OverdueBeatsABiggerOpportunity(t *testing.T) {
	actions := []PendingAction{
		{Kind: PendingSignatureViewed, TargetID: "big-opportunity",
			MoneyAtStakeCents: 10000000},
		{Kind: PendingContractAlert, TargetID: "renewal-fired",
			MoneyAtStakeCents: 500000, DueOn: ccDay(-1)},
	}

	RankPendingActions(actions, ccNow)

	// A renewal that fired yesterday cannot be un-fired by a bigger deal
	// somewhere else. Deadlines that have passed come first, whatever they
	// are worth.
	if actions[0].TargetID != "renewal-fired" {
		t.Fatalf("first = %q, want the overdue deadline", actions[0].TargetID)
	}
}

func TestRankPendingActions_AFutureDeadlineIsNotOverdue(t *testing.T) {
	actions := []PendingAction{
		{Kind: PendingContractAlert, TargetID: "next-month",
			MoneyAtStakeCents: 500000, DueOn: ccDay(20)},
		{Kind: PendingSignatureViewed, TargetID: "big",
			MoneyAtStakeCents: 10000000},
	}

	RankPendingActions(actions, ccNow)

	// Not yet due means it ranks on its amount like everything else —
	// otherwise every future deadline would permanently outrank live money.
	if actions[0].TargetID != "big" {
		t.Fatalf("first = %q, want the bigger amount", actions[0].TargetID)
	}
}

func TestRankPendingActions_AmongMoneylessTheOldestOfTheSameKindFirst(t *testing.T) {
	actions := []PendingAction{
		{Kind: PendingApproveDraft, TargetID: "today", SinceAt: ccDay(0)},
		{Kind: PendingApproveDraft, TargetID: "four-days", SinceAt: ccDay(-4)},
		{Kind: PendingUnownedLead, TargetID: "yesterday", SinceAt: ccDay(-1)},
	}

	RankPendingActions(actions, ccNow)

	// Kind decides across kinds; staleness decides within one. An offer
	// written this morning is still work that pays, and a lead nobody owns
	// is still a chore, however long it has sat.
	if actions[0].TargetID != "four-days" {
		t.Fatalf("first = %q, want the oldest offer", actions[0].TargetID)
	}
	if actions[1].TargetID != "today" {
		t.Fatalf("second = %q, want the newer offer, still ahead of the chore", actions[1].TargetID)
	}
	if actions[2].TargetID != "yesterday" {
		t.Fatalf("last = %q, want the unowned lead", actions[2].TargetID)
	}
}

// The exact shape production had on 2026-09-09: 39 offers waiting, and the
// six rows the screen showed were all "assign this lead". Both kinds carry
// no amount, and the unowned leads were older, so age handed the top of the
// screen to the work that pays nothing.
func TestRankPendingActions_OffersOutrankOlderChores(t *testing.T) {
	actions := []PendingAction{
		{Kind: PendingUnownedLead, TargetID: "chore-1", SinceAt: ccDay(-39)},
		{Kind: PendingUnownedLead, TargetID: "chore-2", SinceAt: ccDay(-39)},
		{Kind: PendingApproveDraft, TargetID: "offer", SinceAt: ccDay(-17)},
	}

	RankPendingActions(actions, ccNow)

	if actions[0].TargetID != "offer" {
		t.Fatalf("first = %q, want the offer even though the chores are older", actions[0].TargetID)
	}
}

// A customer already talking outranks an offer nobody has read yet: the
// reply window is the one that closes by itself. Pins the intent that
// pendingKindRank encodes, now that it actually decides the order.
func TestRankPendingActions_MoneyStillBeatsKind(t *testing.T) {
	actions := []PendingAction{
		{Kind: PendingApproveDraft, TargetID: "offer", SinceAt: ccDay(-30)},
		{Kind: PendingReorderDue, TargetID: "reorder", MoneyAtStakeCents: 90000},
	}

	RankPendingActions(actions, ccNow)

	if actions[0].TargetID != "reorder" {
		t.Fatalf("first = %q, want the row with an amount", actions[0].TargetID)
	}
}

func TestRankPendingActions_NoAgeSortsLastNotFirst(t *testing.T) {
	actions := []PendingAction{
		{Kind: PendingUnownedLead, TargetID: "unmeasured"},
		{Kind: PendingApproveDraft, TargetID: "aged", SinceAt: ccDay(-1)},
	}

	RankPendingActions(actions, ccNow)

	// An action with no recorded age is UNMEASURED, not infinitely old.
	// Sorting it first would put the least-known row at the top of the screen.
	if actions[0].TargetID != "aged" {
		t.Fatalf("first = %q, want the one with a known age", actions[0].TargetID)
	}
}

func TestRankPendingActions_IsDeterministic(t *testing.T) {
	build := func() []PendingAction {
		return []PendingAction{
			{Kind: PendingUnownedLead, TargetID: "b"},
			{Kind: PendingApproveDraft, TargetID: "a"},
			{Kind: PendingReorderDue, TargetID: "c"},
		}
	}

	first, second := build(), build()
	RankPendingActions(first, ccNow)
	RankPendingActions(second, ccNow)

	// Identical data must render an identical order — a list that reshuffles
	// between two loads looks made up.
	for i := range first {
		if first[i].TargetID != second[i].TargetID {
			t.Fatalf("position %d: %q vs %q", i, first[i].TargetID, second[i].TargetID)
		}
	}
}

func TestFilterDismissed_DropsOnlyTheSnoozedPair(t *testing.T) {
	actions := []PendingAction{
		{Kind: PendingApproveDraft, TargetID: "l1"},
		{Kind: PendingUnownedLead, TargetID: "l1"},
		{Kind: PendingApproveDraft, TargetID: "l2"},
	}
	dismissed := map[DismissalKey]struct{}{
		{Kind: PendingApproveDraft, TargetID: "l1"}: {},
	}

	out := FilterDismissed(actions, dismissed)

	// The key is (kind, target): the SAME lead can legitimately need two
	// different things, and snoozing one must not hide the other.
	if len(out) != 2 {
		t.Fatalf("got %d actions, want 2: %+v", len(out), out)
	}
	for _, a := range out {
		if a.Kind == PendingApproveDraft && a.TargetID == "l1" {
			t.Fatal("the snoozed pair survived")
		}
	}
}

func TestPendingActionDismissal_ASnoozeAlwaysExpires(t *testing.T) {
	d := &PendingActionDismissal{Kind: PendingApproveDraft, TargetID: "l1"}

	if err := d.Validate(ccNow); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !d.SnoozedUntil.After(ccNow) {
		t.Fatal("an empty snooze must default to a real future deadline")
	}

	// A caller asking for a decade gets the cap. Past that, a "snooze" is a
	// deletion with better manners, and the deadline it hides is still real.
	d.SnoozedUntil = ccNow.AddDate(10, 0, 0)
	if err := d.Validate(ccNow); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if d.SnoozedUntil.After(ccNow.Add(MaxSnooze)) {
		t.Fatalf("snooze = %v, want capped at %v", d.SnoozedUntil, MaxSnooze)
	}
}

func TestPendingActionDismissal_RefusesWhatCannotBeHonoured(t *testing.T) {
	cases := map[string]*PendingActionDismissal{
		"unknown kind": {Kind: "whatever", TargetID: "l1"},
		"no target":    {Kind: PendingApproveDraft, TargetID: "  "},
		// A snooze already in the past would read as success and then the row
		// would still be there.
		"past snooze": {Kind: PendingApproveDraft, TargetID: "l1",
			SnoozedUntil: ccNow.Add(-time.Hour)},
	}
	for name, d := range cases {
		if err := d.Validate(ccNow); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}

func TestBuildMiniFunnel_ProjectsOntoTheCompressedLadder(t *testing.T) {
	stages := BuildMiniFunnel(map[LeadStatus]int64{
		LeadStatusNew: 12,
		// A machine state: it lasts seconds and must not become a bar.
		LeadStatusPendingApproval: 7,
		LeadStatusContacted:       5,
		LeadStatusWon:             1,
	})

	if len(stages) != len(MiniFunnelLadder()) {
		t.Fatalf("got %d stages, want the ladder", len(stages))
	}
	for _, s := range stages {
		if s.Status == LeadStatusPendingApproval {
			t.Fatal("pending_approval is a queue, not a funnel stage")
		}
	}
	byStatus := map[LeadStatus]int64{}
	for _, s := range stages {
		byStatus[s.Status] = s.Count
	}
	if byStatus[LeadStatusNew] != 12 || byStatus[LeadStatusContacted] != 5 {
		t.Fatalf("counts = %d, %d; want 12, 5",
			byStatus[LeadStatusNew], byStatus[LeadStatusContacted])
	}
	// A status with no leads is a real zero on the board, not a missing bar:
	// the shape of the funnel is the information.
	if _, ok := byStatus[LeadStatusResponded]; !ok {
		t.Fatal("responded is missing; want an explicit 0")
	}
}

// The home ladder used to be a second, shorter hand-written list. It dropped
// `prospected` and `enriched` — the two places prospecting parks its output —
// so an account that had just imported two hundred clinics saw a funnel of all
// zeros beside a tile reading "200 leads ativos".
func TestMiniFunnelLadder_CountsEveryStageTheBoardHas(t *testing.T) {
	inLadder := map[LeadStatus]bool{}
	for _, rung := range MiniFunnelLadder() {
		inLadder[rung.Status] = true
	}
	for _, stage := range DefaultFunnelStages() {
		if stage.Status == LeadStatusLost {
			continue // a ladder of progress does not end in the loss column
		}
		if stage.Status == LeadStatusPendingApproval {
			// A board column since 2026-09-10 (it holds the bottleneck and
			// deserved one), but NOT a home rung: the tile above this funnel
			// already carries that number, and showing it twice on one screen
			// reads as two different facts.
			continue
		}
		if !inLadder[stage.Status] {
			t.Fatalf("board stage %q (%s) is counted nowhere on the home screen",
				stage.Name, stage.Status)
		}
	}
	// And the transient states stay out: one lasts seconds, the other already
	// has its own tile above the funnel.
	for _, s := range []LeadStatus{LeadStatusEnriching, LeadStatusPendingApproval} {
		if inLadder[s] {
			t.Fatalf("%s is a queue, not a rung", s)
		}
	}
}

// Same account, same second, two screens: the board resolved the 41
// pending_approval leads onto "Pesquisados" while the home funnel read the
// raw status and drew nothing. 19 of 60 leads had a bar; the offers had none.
func TestBuildMiniFunnel_CountsPendingApprovalWhereTheBoardShowsIt(t *testing.T) {
	funnel := BuildMiniFunnel(map[LeadStatus]int64{
		LeadStatusProspected:      19,
		LeadStatusPendingApproval: 41,
	})

	var total int64
	byStatus := map[LeadStatus]int64{}
	for _, s := range funnel {
		total += s.Count
		byStatus[s.Status] = s.Count
	}

	if total != 60 {
		t.Fatalf("funnel totals %d, want all 60 leads represented", total)
	}
	if byStatus[LeadStatusEnriched] != 41 {
		t.Fatalf("Pesquisados = %d, want the 41 waiting offers, same as the board",
			byStatus[LeadStatusEnriched])
	}
	if byStatus[LeadStatusProspected] != 19 {
		t.Fatalf("Prospectados = %d, want 19", byStatus[LeadStatusProspected])
	}
}

// `enriching` folds the other way — onto Prospectados — and must not be
// double-counted into the column it came from.
func TestProjectStatusCounts_FoldsBothTransientStates(t *testing.T) {
	got := ProjectStatusCounts(map[LeadStatus]int64{
		LeadStatusProspected:      2,
		LeadStatusEnriching:       3,
		LeadStatusEnriched:        1,
		LeadStatusPendingApproval: 4,
		LeadStatusWon:             5,
	})

	want := map[LeadStatus]int64{
		LeadStatusProspected: 5,
		LeadStatusEnriched:   5,
		LeadStatusWon:        5,
	}
	for status, n := range want {
		if got[status] != n {
			t.Fatalf("%s = %d, want %d", status, got[status], n)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("got %d buckets, want %d: %v", len(got), len(want), got)
	}
}
