package domain

import (
	"context"
	"math"
	"strings"
	"time"
)

// Economic indices and the adjustment a contract is owed (§6.11, G5/G6).
//
// "Reajuste IPCA em março" is a reminder. "You may raise 4.8%, the accumulated
// IPCA since the base date" is a renegotiation — and the difference between the
// two is the whole reason a company pays for this: a price left frozen never
// recovers, because the base it compounds from never resets.
//
// Everything here refuses to guess. A missing month in the series makes the
// accumulation INCOMPLETE rather than smaller, because a number that is quietly
// low is worse than no number: somebody would take it to a negotiation.

// IndexCode is one of the four indices a Brazilian contract actually names.
type IndexCode string

const (
	IndexIPCA  IndexCode = "ipca"
	IndexIGPM  IndexCode = "igpm"
	IndexINPC  IndexCode = "inpc"
	IndexIGPDI IndexCode = "igpdi"
)

// NormalizeIndexCode reads the index out of whatever the contract wrote.
//
// Contracts say "IGP-M", "IGPM (FGV)", "índice IGP-M/FGV" and "igp m" for the
// same thing. Returns empty for anything unrecognised — an unknown index means
// the alert says "renegociar" with no percentage, which is honest, instead of
// silently applying the wrong series.
func NormalizeIndexCode(raw string) IndexCode {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return ""
	}
	// Punctuation and spacing carry no meaning here: "IGP-M" and "IGP M" are
	// the same index, and the alternative is a lookup table of spellings.
	compact := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, s)

	switch {
	// INPC before IPCA: "inpc" contains neither substring of the other, but
	// checking IPCA first on a string like "ipcainpc" would pick the wrong one.
	case strings.Contains(compact, "inpc"):
		return IndexINPC
	case strings.Contains(compact, "igpdi"):
		return IndexIGPDI
	case strings.Contains(compact, "igpm"):
		return IndexIGPM
	case strings.Contains(compact, "ipca"):
		return IndexIPCA
	// A bare "igp" is ambiguous between IGP-M and IGP-DI. In supply contracts
	// it is IGP-M in practice, but guessing which index somebody's price rises
	// by is not a call this code gets to make.
	default:
		return ""
	}
}

// IndexValue is one month of one series.
type IndexValue struct {
	Code IndexCode `json:"code"`
	// Month is always the first day of the reference month.
	Month time.Month `json:"-"`
	Year  int        `json:"-"`
	// ReferenceMonth is the canonical first-of-month date.
	ReferenceMonth time.Time `json:"reference_month"`
	// MonthlyRate is the month's variation in PERCENT: 0.62 means 0.62%.
	MonthlyRate float64 `json:"monthly_rate"`
}

// IndexAccumulation is what a contract may be raised by.
type IndexAccumulation struct {
	Code IndexCode `json:"code"`
	// From is the first month INCLUDED in the accumulation (the month after the
	// base date) and To the last.
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
	// Months is how many months were compounded — the sanity check a person
	// does out loud ("doze meses de IPCA, faz sentido").
	Months int `json:"months"`
	// Percent is the compounded variation, rounded to two decimals: 4.83 means
	// the price may rise 4.83%.
	Percent float64 `json:"percent"`
	// Complete is false when the series was missing at least one month in the
	// range. A partial accumulation is NOT reported as a percentage anybody
	// should negotiate with — see MissingMonths.
	Complete bool `json:"complete"`
	// MissingMonths names what was absent, so the screen can say "faltam dois
	// meses da série" instead of showing a number that is quietly low.
	MissingMonths []string `json:"missing_months,omitempty"`
}

// UsablePercent is the number a person may take into a negotiation, or nil when
// the series was incomplete.
//
// The whole point of the split: the accumulation is still returned for display
// ("parcial: 3,1% de 10 meses"), but the field that drives a suggested new
// price is absent unless every month is there.
func (a *IndexAccumulation) UsablePercent() *float64 {
	if a == nil || !a.Complete || a.Months == 0 {
		return nil
	}
	p := a.Percent
	return &p
}

// AccumulateIndex compounds a monthly series over [from, to], both inclusive
// and both first-of-month.
//
// Compounded, never summed: twelve months of 1% is 12.68%, not 12%, and over a
// multi-year contract the difference is the argument.
func AccumulateIndex(code IndexCode, values []IndexValue, from, to time.Time) *IndexAccumulation {
	from = FirstOfMonth(from)
	to = FirstOfMonth(to)
	acc := &IndexAccumulation{Code: code, From: from, To: to, Complete: true}
	if to.Before(from) {
		// Nothing elapsed yet: zero months, zero percent, and COMPLETE — a
		// contract adjusted last month is not missing data, it simply has no
		// accumulation to claim.
		acc.To = from.AddDate(0, -1, 0)
		return acc
	}

	byMonth := make(map[time.Time]float64, len(values))
	for _, v := range values {
		if v.Code != code {
			continue
		}
		byMonth[FirstOfMonth(v.ReferenceMonth)] = v.MonthlyRate
	}

	factor := 1.0
	for month := from; !month.After(to); month = month.AddDate(0, 1, 0) {
		rate, ok := byMonth[month]
		if !ok {
			acc.Complete = false
			acc.MissingMonths = append(acc.MissingMonths, month.Format("2006-01"))
			continue
		}
		factor *= 1 + rate/100
		acc.Months++
	}

	acc.Percent = math.Round((factor-1)*100*100) / 100
	return acc
}

// FirstOfMonth normalizes a date to the first day of its month, in UTC.
//
// The series is monthly; a day component would split one month into two keys
// and make the same month look missing.
func FirstOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// ContractAdjustment is the renegotiation one contract is owed.
type ContractAdjustment struct {
	// BaseDate is what the accumulation counts from: the last time the price was
	// actually raised, or the contract's start when it never was.
	BaseDate time.Time `json:"base_date"`
	// DueOn is the next adjustment date the contract states.
	DueOn time.Time `json:"due_on"`
	// Overdue says the date has passed with no adjustment recorded — the state
	// that is quietly costing money every month it stays true.
	Overdue bool `json:"overdue"`

	Accumulation *IndexAccumulation `json:"accumulation,omitempty"`

	// CurrentValueCents is the contract's value today and SuggestedValueCents
	// what it becomes with the accumulation applied. Suggested is absent when
	// the accumulation was incomplete or there is no value to raise: a
	// suggested price built on a partial series is a number somebody would
	// quote to a customer.
	CurrentValueCents   int64  `json:"current_value_cents"`
	SuggestedValueCents *int64 `json:"suggested_value_cents,omitempty"`
	// GainCents is the difference — the money on the table, which is the only
	// number that makes anybody open the alert.
	GainCents *int64 `json:"gain_cents,omitempty"`
}

// NextAdjustmentDate is when the contract's price is next due to rise.
//
// Nil when the contract states no base month: a price with no stated adjustment
// month does not get one invented for it.
func (c *Contract) NextAdjustmentDate(now time.Time) *time.Time {
	if c == nil || c.AdjustmentMonth == nil {
		return nil
	}
	month := time.Month(*c.AdjustmentMonth)
	base := c.AdjustmentBaseDate()

	// The first occurrence of the stated month that is at least TWELVE MONTHS
	// after the base date.
	//
	// Annuality is not a preference — Lei 10.192/2001 forbids a price adjustment
	// interval under a year, so a contract that starts in April and states March
	// is first adjusted in March of the year AFTER next. Suggesting the earlier
	// date would hand the counterpart a reason to refuse the whole increase.
	earliest := FirstOfMonth(base).AddDate(1, 0, 0)
	year := earliest.Year()
	if month < earliest.Month() {
		year++
	}
	due := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	for due.Before(earliest) {
		due = due.AddDate(1, 0, 0)
	}
	// A due date already in the past is RETURNED as it is, not rolled forward:
	// the overdue adjustment is the one worth money, and hiding it behind next
	// year's date is how a year of a frozen price disappears.
	return &due
}

// AdjustmentBaseDate is what an accumulation counts from.
func (c *Contract) AdjustmentBaseDate() time.Time {
	if c == nil {
		return time.Time{}
	}
	if c.LastAdjustedOn != nil {
		return *c.LastAdjustedOn
	}
	if c.StartsOn != nil {
		return *c.StartsOn
	}
	return c.CreatedAt
}

// EconomicIndexRepository is the cached series.
type EconomicIndexRepository interface {
	// Range returns the months of one index in [from, to], both first-of-month.
	Range(ctx context.Context, code IndexCode, from, to time.Time) ([]IndexValue, error)
	// Upsert stores fetched months. Idempotent per (code, month).
	Upsert(ctx context.Context, values []IndexValue) error
	// LatestMonth is the newest month held for an index, so the fetcher knows
	// where to resume instead of re-reading the whole series every time.
	LatestMonth(ctx context.Context, code IndexCode) (*time.Time, error)
}
