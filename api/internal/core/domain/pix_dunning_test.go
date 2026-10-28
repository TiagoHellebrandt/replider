package domain

import (
	"testing"
	"time"
)

// US-M014. The ladder decides when a paying customer stops being served,
// so the cases that matter are the ones where being wrong costs money or
// trust: skipping a rung, repeating one, or walking someone through four
// messages because a worker was down.

func TestPixDunning_LadderTimings(t *testing.T) {
	cases := []struct {
		name string
		days int
		sent PixDunningStage
		want PixDunningStage
	}{
		{"nothing due a week out", -7, PixStageNone, PixStageNone},
		{"charge goes out 3 days early", -3, PixStageNone, PixStageCharge},
		{"still just the charge 1 day out", -1, PixStageCharge, PixStageNone},
		{"reminder on the renewal date", 0, PixStageCharge, PixStageDueReminder},
		{"second reminder 2 days late", 2, PixStageDueReminder, PixStageOverdueReminder},
		{"nothing new on day 3", 3, PixStageOverdueReminder, PixStageNone},
		{"suspended at 7 days late", 7, PixStageOverdueReminder, PixStageSuspended},
		{"nothing after suspension", 30, PixStageSuspended, PixStageNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NextPixDunningStage(tc.days, tc.sent); got != tc.want {
				t.Errorf("days=%d sent=%d: got %d, want %d", tc.days, tc.sent, got, tc.want)
			}
		})
	}
}

// A worker that was down for a week must not deliver the whole ladder on
// its way back up: the customer gets the message that is true TODAY.
func TestPixDunning_BacklogCollapsesToTheTruestRung(t *testing.T) {
	if got := NextPixDunningStage(9, PixStageNone); got != PixStageSuspended {
		t.Errorf("a nine-day backlog must jump straight to suspension, got %d", got)
	}
	if got := NextPixDunningStage(2, PixStageNone); got != PixStageOverdueReminder {
		t.Errorf("a two-day backlog must skip to the overdue reminder, got %d", got)
	}
}

// Renewal resets the ladder. Without this an account that paid would
// inherit last month's "suspended" stamp and never be chased again — or
// worse, be treated as already-suspended.
func TestPixDunning_StageResetsWhenThePeriodMoves(t *testing.T) {
	lastMonth := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	thisMonth := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)

	stale := PixRenewalAccount{
		PeriodEnd:      thisMonth,
		StageSent:      PixStageSuspended,
		StagePeriodEnd: &lastMonth,
	}
	if got := stale.StageForPeriod(); got != PixStageNone {
		t.Errorf("a stamp from a previous period must not carry over, got %d", got)
	}

	current := PixRenewalAccount{
		PeriodEnd:      thisMonth,
		StageSent:      PixStageCharge,
		StagePeriodEnd: &thisMonth,
	}
	if got := current.StageForPeriod(); got != PixStageCharge {
		t.Errorf("this period's stamp must count, got %d", got)
	}

	// No stamp at all is a fresh ladder, not an error.
	if got := (PixRenewalAccount{PeriodEnd: thisMonth}).StageForPeriod(); got != PixStageNone {
		t.Errorf("an unstamped account starts at zero, got %d", got)
	}
}

// Whole days, both sides truncated: an 08:00 run and a 23:00 run must
// agree on the rung, or an account gets suspended a run early because of
// the clock.
func TestPixDunning_DaysFromRenewalIgnoresTimeOfDay(t *testing.T) {
	renewal := time.Date(2026, 7, 31, 14, 30, 0, 0, time.UTC)

	if got := DaysFromRenewal(time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC), renewal); got != 0 {
		t.Errorf("morning of the renewal day is day 0, got %d", got)
	}
	if got := DaysFromRenewal(time.Date(2026, 7, 31, 23, 59, 0, 0, time.UTC), renewal); got != 0 {
		t.Errorf("late on the renewal day is still day 0, got %d", got)
	}
	if got := DaysFromRenewal(time.Date(2026, 8, 7, 0, 1, 0, 0, time.UTC), renewal); got != 7 {
		t.Errorf("seven days later is day 7, got %d", got)
	}
	if got := DaysFromRenewal(time.Date(2026, 7, 28, 23, 0, 0, 0, time.UTC), renewal); got != -3 {
		t.Errorf("three days out is day -3, got %d", got)
	}
}
