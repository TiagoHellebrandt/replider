package domain

import (
	"testing"
	"time"
)

// Overnight drafting (K2).
//
// The schedule is stated in the ACCOUNT's local hours, which is the whole reason
// this has its own tests: a UTC comparison fires in the middle of the afternoon
// for half of Brazil, and runs the same "night" twice at the date boundary.

func TestDueTonight_FiresAtTheAccountsLocalHour(t *testing.T) {
	s := &OvernightDraftSettings{Enabled: true, RunHour: 3}

	// 06:10 UTC is 03:10 in São Paulo.
	due := time.Date(2026, 7, 30, 6, 10, 0, 0, time.UTC)
	if !s.DueTonight(due, "America/Sao_Paulo") {
		t.Fatal("3am local must be due")
	}

	// The same instant is 03:10 UTC for a UTC account — also its 3am — but
	// 02:10 in Manaus, which is not.
	if s.DueTonight(due, "America/Manaus") {
		t.Fatal("02:10 local must not be due")
	}
}

func TestDueTonight_DisabledNeverRuns(t *testing.T) {
	s := &OvernightDraftSettings{Enabled: false, RunHour: 3}
	at3am := time.Date(2026, 7, 30, 3, 0, 0, 0, time.UTC)

	if s.DueTonight(at3am, "UTC") {
		t.Fatal("an account that never opted in must not be drafted for")
	}
	// A nil settings row is the same answer, and must not panic on a worker
	// path that iterates whatever the query returned.
	var missing *OvernightDraftSettings
	if missing.DueTonight(at3am, "UTC") {
		t.Fatal("nil settings must not be due")
	}
}

func TestDueTonight_OnlyOncePerLocalNight(t *testing.T) {
	ran := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	s := &OvernightDraftSettings{Enabled: true, RunHour: 3, LastRunOn: &ran}

	// A second tick inside the same hour of the same local day.
	same := time.Date(2026, 7, 30, 3, 50, 0, 0, time.UTC)
	if s.DueTonight(same, "UTC") {
		t.Fatal("a night already run must not run again — that is a second night's allowance")
	}

	// The next night is a different date.
	next := time.Date(2026, 7, 31, 3, 10, 0, 0, time.UTC)
	if !s.DueTonight(next, "UTC") {
		t.Fatal("the next night must be due")
	}
}

func TestDueTonight_TheDateIsTheAccountsToo(t *testing.T) {
	// 03:00 UTC on the 31st is still 00:00 on the 31st in São Paulo — but a run
	// stamped for the 30th UTC could look like "today" or "yesterday" depending
	// on whose calendar you read it with.
	ran := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	s := &OvernightDraftSettings{Enabled: true, RunHour: 3, LastRunOn: &ran}

	// 06:10 UTC on the 31st = 03:10 local on the 31st: a new local day.
	next := time.Date(2026, 7, 31, 6, 10, 0, 0, time.UTC)
	if !s.DueTonight(next, "America/Sao_Paulo") {
		t.Fatal("a new local date must be due even though the stored date is UTC-midnight")
	}
}

func TestValidate_ClampsWhatCannotBeHonoured(t *testing.T) {
	s := &OvernightDraftSettings{NightlyCap: 0, RunHour: 99}
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if s.NightlyCap != DefaultNightlyDraftCap {
		t.Fatalf("cap = %d, want the default", s.NightlyCap)
	}
	// An out-of-range hour is a client bug; refusing it would turn a typo into a
	// night with no drafts, which is a worse outcome than the default hour.
	if s.RunHour != DefaultOvernightRunHour {
		t.Fatalf("hour = %d, want the default", s.RunHour)
	}

	big := &OvernightDraftSettings{NightlyCap: 99999, RunHour: 3}
	_ = big.Validate()
	if big.NightlyCap != MaxNightlyDraftCap {
		t.Fatalf("cap = %d, want the ceiling", big.NightlyCap)
	}
}

func TestOvernightRun_ANightThatDreWNothingIsNotNews(t *testing.T) {
	// "0 abordagens prontas" every morning is how a good habit becomes a
	// dismissed banner. The most common reason for an empty night is the honest
	// one: there was nobody left in the funnel to write to.
	empty := &OvernightDraftRun{Drafted: 0, SkippedOptedOut: 12}
	if empty.Worth() {
		t.Fatal("an empty night must not produce a briefing")
	}

	real := &OvernightDraftRun{Drafted: 40}
	if !real.Worth() {
		t.Fatal("40 drafts is exactly the news this feature exists to deliver")
	}

	var missing *OvernightDraftRun
	if missing.Worth() {
		t.Fatal("nil must not be worth showing")
	}
}

func TestOvernightRun_ABudgetCutIsSurfacedAlongsideTheCount(t *testing.T) {
	// "40 prontas" when the customer asked for 100 needs the missing 60
	// explained, or the number simply does not add up.
	cut := &OvernightDraftRun{Drafted: 40, SkippedBudget: 60}
	if !cut.BudgetHit() {
		t.Fatal("a run the plan's allowance cut short must say so")
	}
	if !(&OvernightDraftRun{Drafted: 40}).Worth() {
		t.Fatal("a full night is still worth showing")
	}
	if (&OvernightDraftRun{Drafted: 40}).BudgetHit() {
		t.Fatal("a run that fitted must not claim it was cut")
	}
}
