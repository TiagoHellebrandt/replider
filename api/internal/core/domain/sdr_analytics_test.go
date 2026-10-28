package domain

import (
	"testing"
	"time"
)

// The two properties this file exists to pin: a period is bounded in the
// account's timezone, and a rate over nothing is unknown rather than zero.

func TestResolveSDRPeriod_TodayIsAccountMidnightNotUTC(t *testing.T) {
	// 02:30 UTC on the 10th is still 22:30 on the 9th in São Paulo. An
	// account there asking for "hoje" must get the 9th — a report that
	// silently shifts by hours is one somebody screenshots and argues with.
	now := time.Date(2026, 7, 10, 2, 30, 0, 0, time.UTC)

	p := ResolveSDRPeriod(SDRPeriodToday, "America/Sao_Paulo", now)

	if got := p.From.Day(); got != 9 {
		t.Fatalf("From day = %d, want 9 (local midnight of the 9th)", got)
	}
	if p.From.Hour() != 0 {
		t.Fatalf("From hour = %d, want local midnight", p.From.Hour())
	}
	if p.Timezone != "America/Sao_Paulo" {
		t.Fatalf("Timezone = %q, want the account's", p.Timezone)
	}
}

func TestResolveSDRPeriod_UnknownKeyFallsBackWithoutLying(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

	p := ResolveSDRPeriod("last-fortnight", "UTC", now)

	// A typo in a saved deep link renders the default view, and the payload
	// reports which period was ACTUALLY used so the right chip highlights.
	if p.Key != DefaultSDRPeriod {
		t.Fatalf("Key = %q, want the default %q", p.Key, DefaultSDRPeriod)
	}
}

func TestResolveSDRPeriod_SevenDaysCoversSevenNotEight(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

	p := ResolveSDRPeriod(SDRPeriod7d, "UTC", now)

	if got := p.From.Day(); got != 4 {
		t.Fatalf("From day = %d, want 4 (today plus 6 back = 7 days)", got)
	}
}

func TestResolveSDRPeriod_ToIsNowNotEndOfDay(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 15, 0, 0, time.UTC)

	p := ResolveSDRPeriod(SDRPeriodToday, "UTC", now)

	// Counting the rest of today as elapsed would divide by leads that have
	// not had their chance yet.
	if !p.To.Equal(now) {
		t.Fatalf("To = %v, want now (%v)", p.To, now)
	}
}

func TestSDRPeriod_PreviousIsSameLengthImmediatelyBefore(t *testing.T) {
	p := SDRPeriod{
		From: time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
	}

	prev := p.Previous()

	if !prev.To.Equal(p.From) {
		t.Fatalf("previous To = %v, want the current From", prev.To)
	}
	if prev.To.Sub(prev.From) != p.To.Sub(p.From) {
		t.Fatalf("previous span %v != current span %v", prev.To.Sub(prev.From), p.To.Sub(p.From))
	}
}

func TestBuildFunnel_RatesAreNilWhenThereIsNothingToDivideBy(t *testing.T) {
	steps := BuildFunnel(SDRFunnelCounts{})

	if len(steps) != len(SDRFunnelLadder()) {
		t.Fatalf("got %d steps, want the full ladder", len(steps))
	}
	for _, s := range steps {
		if s.ConversionFromPrevious != nil {
			t.Fatalf("%s: conversion = %v, want nil (0 of 0 is unknown, not 0%%)",
				s.Metric, *s.ConversionFromPrevious)
		}
		if s.ConversionFromEntry != nil {
			t.Fatalf("%s: overall = %v, want nil", s.Metric, *s.ConversionFromEntry)
		}
	}
}

func TestBuildFunnel_ConversionIsAgainstThePreviousRungAndTheEntry(t *testing.T) {
	steps := BuildFunnel(SDRFunnelCounts{
		SDRMetricEntered: 100,
		// Researched is deliberately NOT a rung; it must not land in the
		// ladder or shift the rungs below it.
		SDRMetricResearched: 80,
		SDRMetricApproached: 40,
		SDRMetricReplied:    10,
		SDRMetricQualified:  4,
		SDRMetricWon:        2,
	})

	byMetric := map[string]SDRFunnelStep{}
	for _, s := range steps {
		byMetric[s.Metric] = s
	}
	if _, present := byMetric[SDRMetricResearched]; present {
		t.Fatal("researched must not appear as a funnel rung")
	}

	replied := byMetric[SDRMetricReplied]
	if replied.ConversionFromPrevious == nil || *replied.ConversionFromPrevious != 0.25 {
		t.Fatalf("replied from previous = %v, want 0.25 (10/40)",
			rateValue(replied.ConversionFromPrevious))
	}
	if replied.ConversionFromEntry == nil || *replied.ConversionFromEntry != 0.1 {
		t.Fatalf("replied from entry = %v, want 0.1 (10/100)",
			rateValue(replied.ConversionFromEntry))
	}
	// The first rung has nothing above it.
	if steps[0].ConversionFromPrevious != nil {
		t.Fatal("entry step should have no conversion-from-previous")
	}
}

// rateValue dereferences for error messages — %v on a *float64 prints a pointer,
// which tells you nothing about why the assertion failed.
func rateValue(v *float64) any {
	if v == nil {
		return "nil"
	}
	return *v
}

func TestBuildFunnel_AStepBiggerThanTheOneAboveIsFlaggedNotClamped(t *testing.T) {
	// Real case: a sale typed in by hand for a lead nobody ever recorded an
	// approach for. Clamping would hide a data-quality problem the operator
	// can actually fix.
	steps := BuildFunnel(SDRFunnelCounts{
		SDRMetricEntered:    10,
		SDRMetricApproached: 2,
		SDRMetricReplied:    5,
	})

	replied := steps[2]
	if replied.Metric != SDRMetricReplied {
		t.Fatalf("steps[2] = %q, want the replied rung", replied.Metric)
	}
	if !replied.Incomplete {
		t.Fatal("replied (5) > approached (2) must be flagged incomplete")
	}
	if replied.Count != 5 {
		t.Fatalf("count = %d, want the real 5 — never clamped to the rung above", replied.Count)
	}
	if steps[1].Incomplete {
		t.Fatal("a rung smaller than the one above it is a normal funnel, not incomplete")
	}
}

func TestSDRRate_ZeroDenominatorIsUnknown(t *testing.T) {
	if v := (SDRRate{Numerator: 0, Denominator: 0}).Value(); v != nil {
		t.Fatalf("0/0 = %v, want nil", *v)
	}
	// A real zero out of a real denominator IS zero and must be reported.
	v := (SDRRate{Numerator: 0, Denominator: 7}).Value()
	if v == nil || *v != 0 {
		t.Fatalf("0/7 = %v, want 0", v)
	}
}

func TestValidSDRMetric(t *testing.T) {
	for _, rung := range SDRFunnelLadder() {
		if !ValidSDRMetric(rung.Metric) {
			t.Fatalf("%q is on the ladder but rejected by the guard", rung.Metric)
		}
	}
	// Researched is a valid DRILL metric even though it is not a ladder rung —
	// "who did we research?" is a real question with a real list behind it.
	if !ValidSDRMetric(SDRMetricResearched) {
		t.Fatal("researched must remain drillable")
	}
	// The guard is what keeps a metric name out of SQL. It validates the
	// EXACT string, untrimmed — accepting "entered " while the query runs
	// with the space would pass a value nothing matches.
	for _, bad := range []string{"", "drop table", "Won", "entered "} {
		if ValidSDRMetric(bad) {
			t.Fatalf("%q accepted", bad)
		}
	}
}
