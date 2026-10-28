package domain

import (
	"math"
	"testing"
	"time"
)

// The index accumulation (§6.11, G5/G6).
//
// This arithmetic is quoted to a customer in a renegotiation. Being wrong is
// worse than being absent, so the tests are about the two ways it could lie:
// summing instead of compounding, and reporting a partial series as a whole one.

func month(y int, m time.Month) time.Time {
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}

func series(code IndexCode, from time.Time, rates ...float64) []IndexValue {
	out := make([]IndexValue, 0, len(rates))
	for i, rate := range rates {
		out = append(out, IndexValue{
			Code:           code,
			ReferenceMonth: from.AddDate(0, i, 0),
			MonthlyRate:    rate,
		})
	}
	return out
}

func TestAccumulateIndex_CompoundsNeverSums(t *testing.T) {
	// Twelve months of exactly 1% is 12.68%, not 12%. Over a multi-year contract
	// the difference between the two IS the argument.
	values := series(IndexIPCA, month(2026, time.January),
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1)

	acc := AccumulateIndex(IndexIPCA, values, month(2026, time.January), month(2026, time.December))
	if !acc.Complete {
		t.Fatalf("a full series must be complete: %+v", acc.MissingMonths)
	}
	if acc.Months != 12 {
		t.Fatalf("months = %d, want 12", acc.Months)
	}
	if math.Abs(acc.Percent-12.68) > 0.01 {
		t.Fatalf("percent = %.4f, want 12.68 (compounded, not 12.00)", acc.Percent)
	}
	if p := acc.UsablePercent(); p == nil || math.Abs(*p-12.68) > 0.01 {
		t.Fatalf("a complete accumulation is usable: %v", p)
	}
}

func TestAccumulateIndex_HandlesDeflation(t *testing.T) {
	// A negative month is real (IPCA has printed them) and must reduce the
	// accumulation rather than being treated as missing data.
	values := series(IndexIGPM, month(2026, time.March), 1.0, -0.5, 0.5)

	acc := AccumulateIndex(IndexIGPM, values, month(2026, time.March), month(2026, time.May))
	if !acc.Complete || acc.Months != 3 {
		t.Fatalf("deflation is data, not a gap: %+v", acc)
	}
	// 1.01 × 0.995 × 1.005 = 1.00998...
	if math.Abs(acc.Percent-1.0) > 0.02 {
		t.Fatalf("percent = %.4f, want ~1.00", acc.Percent)
	}
}

// The rule the whole feature rests on: a partial series is never a percentage
// somebody negotiates with.
func TestAccumulateIndex_IncompleteSeriesIsNotUsable(t *testing.T) {
	values := []IndexValue{
		{Code: IndexIPCA, ReferenceMonth: month(2026, time.January), MonthlyRate: 1},
		// February missing.
		{Code: IndexIPCA, ReferenceMonth: month(2026, time.March), MonthlyRate: 1},
	}

	acc := AccumulateIndex(IndexIPCA, values, month(2026, time.January), month(2026, time.March))
	if acc.Complete {
		t.Fatal("a missing month must make the accumulation incomplete")
	}
	if acc.UsablePercent() != nil {
		t.Fatal("an incomplete accumulation must not produce a quotable number — " +
			"a figure that is quietly low gets taken into a negotiation")
	}
	// It still reports what it has, so the screen can say "parcial, faltam meses".
	if acc.Months != 2 || len(acc.MissingMonths) != 1 || acc.MissingMonths[0] != "2026-02" {
		t.Fatalf("the gap must be named: %+v", acc)
	}
}

func TestAccumulateIndex_IgnoresOtherIndicesAndNothingElapsed(t *testing.T) {
	// A row from a different series must not be counted just because it lands in
	// the range: adjusting an IPCA contract by IGP-M months is the silent error
	// this guard exists for.
	mixed := []IndexValue{
		{Code: IndexIGPM, ReferenceMonth: month(2026, time.January), MonthlyRate: 5},
	}
	acc := AccumulateIndex(IndexIPCA, mixed, month(2026, time.January), month(2026, time.January))
	if acc.Complete || acc.Months != 0 {
		t.Fatalf("another index's month is not this index's data: %+v", acc)
	}

	// A contract adjusted last month has nothing to claim yet — that is complete
	// and zero, not incomplete.
	empty := AccumulateIndex(IndexIPCA, nil, month(2026, time.June), month(2026, time.May))
	if !empty.Complete || empty.Months != 0 || empty.Percent != 0 {
		t.Fatalf("no elapsed months is zero, not a gap: %+v", empty)
	}
	if empty.UsablePercent() != nil {
		t.Fatal("zero months carries no percentage to apply")
	}
}

func TestNormalizeIndexCode(t *testing.T) {
	cases := map[string]IndexCode{
		"IPCA":             IndexIPCA,
		"ipca":             IndexIPCA,
		"IPCA/IBGE":        IndexIPCA,
		"IGP-M":            IndexIGPM,
		"igpm":             IndexIGPM,
		"IGP-M (FGV)":      IndexIGPM,
		"IGP M":            IndexIGPM,
		"IGP-DI":           IndexIGPDI,
		"INPC":             IndexINPC,
		"índice INPC/IBGE": IndexINPC,
		// A bare "IGP" is ambiguous between IGP-M and IGP-DI, and guessing which
		// index somebody's price rises by is not this code's call.
		"IGP":            "",
		"":               "",
		"tabela própria": "",
		"reajuste anual": "",
	}
	for raw, want := range cases {
		if got := NormalizeIndexCode(raw); got != want {
			t.Errorf("NormalizeIndexCode(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestNextAdjustmentDate_KeepsAnOverdueDateVisible(t *testing.T) {
	march := 3
	now := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)

	// Started March 2025, adjusts every March, never adjusted: March 2026 is
	// overdue and must be REPORTED as March 2026 — rolling it to 2027 is how a
	// year of a frozen price disappears.
	c := &Contract{
		StartsOn:        ptrTime(time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC)),
		AdjustmentMonth: &march,
	}
	due := c.NextAdjustmentDate(now)
	if due == nil || due.Format("2006-01") != "2026-03" {
		t.Fatalf("due = %v, want 2026-03", due)
	}

	// Once applied, the next one is a year later.
	c.LastAdjustedOn = ptrTime(time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC))
	due = c.NextAdjustmentDate(now)
	if due == nil || due.Format("2006-01") != "2027-03" {
		t.Fatalf("after adjusting, due = %v, want 2027-03", due)
	}

	// No stated month, no invented date.
	if (&Contract{StartsOn: c.StartsOn}).NextAdjustmentDate(now) != nil {
		t.Fatal("a contract with no adjustment month gets none invented")
	}

	// Annuality (Lei 10.192/2001): a contract starting in April that states
	// March is first adjusted in March of the year AFTER next — March 2026 would
	// be eleven months and the counterpart could refuse the whole increase.
	april := &Contract{
		StartsOn:        ptrTime(time.Date(2025, time.April, 1, 0, 0, 0, 0, time.UTC)),
		AdjustmentMonth: &march,
	}
	if due := april.NextAdjustmentDate(now); due == nil || due.Format("2006-01") != "2027-03" {
		t.Fatalf("due = %v, want 2027-03 (twelve-month floor)", due)
	}
}

func TestAdjustmentBaseDate_PrefersTheLastRaiseOverTheStart(t *testing.T) {
	start := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	raised := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)

	c := &Contract{StartsOn: &start}
	if got := c.AdjustmentBaseDate(); !got.Equal(start) {
		t.Fatalf("base = %v, want the start date", got)
	}
	// Otherwise an accumulation would run from 2024 forever and double-count
	// every adjustment already applied.
	c.LastAdjustedOn = &raised
	if got := c.AdjustmentBaseDate(); !got.Equal(raised) {
		t.Fatalf("base = %v, want the last raise", got)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
