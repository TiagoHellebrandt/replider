package domain

import (
	"testing"
	"time"
)

func TestRelationshipDate_RecurringIsDueInsideTheLeadWindow(t *testing.T) {
	now := time.Date(2026, 5, 11, 9, 0, 0, 0, time.UTC)
	d := &RelationshipDate{Kind: DateBirthday, Month: 5, Day: 12}

	if !d.DueWithin(now) {
		t.Fatal("a birthday one day out should be picked up")
	}
}

// Two days, not ten: "parabéns pelo aniversário" a week early is worse than
// nothing.
func TestRelationshipDate_RecurringIsNotDueAWeekEarly(t *testing.T) {
	now := time.Date(2026, 5, 5, 9, 0, 0, 0, time.UTC)
	d := &RelationshipDate{Kind: DateBirthday, Month: 5, Day: 12}

	if d.DueWithin(now) {
		t.Fatal("a birthday a week out must not be greeted yet")
	}
}

// The whole reason LastFiredYear exists: a worker that ticks daily would
// otherwise send the same greeting on every day of the lead window.
func TestRelationshipDate_AlreadyFiredThisYearIsNotDueAgain(t *testing.T) {
	now := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	d := &RelationshipDate{Kind: DateBirthday, Month: 5, Day: 12, LastFiredYear: 2026}

	if d.DueWithin(now) {
		t.Fatal("a date already fired this year must not fire twice")
	}
}

func TestRelationshipDate_LastYearsStampDoesNotBlockThisYear(t *testing.T) {
	now := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
	d := &RelationshipDate{Kind: DateBirthday, Month: 5, Day: 12, LastFiredYear: 2025}

	if !d.DueWithin(now) {
		t.Fatal("a birthday comes round; last year's stamp must not silence it")
	}
}

// A missed maintenance does not stop mattering because the day passed.
func TestRelationshipDate_OverdueOneOffStaysDue(t *testing.T) {
	due := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	d := &RelationshipDate{Kind: DateMaintenance, DueAt: &due}

	if !d.DueWithin(due.AddDate(0, 0, 40)) {
		t.Fatal("an overdue maintenance must stay due")
	}
}

func TestRelationshipDate_FutureOneOffIsNotDueYet(t *testing.T) {
	due := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	d := &RelationshipDate{Kind: DateMaintenance, DueAt: &due}

	if d.DueWithin(due.AddDate(0, 0, -30)) {
		t.Fatal("a maintenance a month out must not be raised yet")
	}
}

// The safety rule of F4. A maintenance reminder implies a service the company
// may not be able to schedule, and a check-in is a sales message wearing a
// friendly hat — neither may go out unread.
func TestGreetingIsLowRisk_OnlyGreetings(t *testing.T) {
	for _, kind := range []RelationshipDateKind{DateBirthday, DateFounded, DateSector} {
		if !GreetingIsLowRisk(kind) {
			t.Fatalf("%s is a greeting and should be low risk", kind)
		}
	}
	for _, kind := range []RelationshipDateKind{DateMaintenance, DateCustom} {
		if GreetingIsLowRisk(kind) {
			t.Fatalf("%s must never be sent unattended", kind)
		}
	}
}

func TestSectorDateByKey(t *testing.T) {
	nursing := SectorDateByKey(" Enfermagem ")
	if nursing == nil || nursing.Month != 5 || nursing.Day != 12 {
		t.Fatalf("Dia da Enfermagem is 12 May, got %+v", nursing)
	}
	// A typo must not silently become a 1 January greeting.
	if SectorDateByKey("enfermagem!") != nil {
		t.Fatal("an unknown key must return nil, not a zero value")
	}
}
