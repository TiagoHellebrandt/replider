package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This record feeds a commission payout. Every test here is about the money
// being right, or about the ways it quietly is not.

func distributionSale() *Sale {
	return &Sale{
		ContactID:  "clinica-1",
		GrossCents: 100_000_00, // R$ 100.000
		CostCents:  72_000_00,  // most of the invoice, as in distribution
		TaxCents:   14_000_00,
		SoldAt:     time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
	}
}

// THE reason the three amounts are stored apart. A commission on gross is a
// commission on money the company never had: 5% of R$ 100.000 is R$ 5.000,
// and the whole margin here is R$ 14.000.
func TestSale_NetIsGrossMinusCostAndTax(t *testing.T) {
	assert.Equal(t, int64(14_000_00), distributionSale().NetCents())
}

func TestSale_MarginRate(t *testing.T) {
	rate := distributionSale().MarginRate()

	require.NotNil(t, rate)
	assert.InDelta(t, 0.14, *rate, 0.0001)
}

// Zero over zero is unknown, not zero — rendering 0% would condemn a sale
// that has not been priced yet.
func TestSale_MarginRateIsUnknownWithNoGross(t *testing.T) {
	assert.Nil(t, (&Sale{}).MarginRate())
}

// Surfaced, never refused: discounts, loss leaders and mis-entered costs all
// look like this, and only a person can tell which. But paying commission on
// one without anybody noticing is how a good month becomes a bad quarter.
func TestSale_UnderwaterIsFlagged(t *testing.T) {
	s := distributionSale()
	s.CostCents = 95_000_00

	assert.True(t, s.Underwater())
	assert.Negative(t, s.NetCents())
}

func TestSale_AHealthySaleIsNotFlagged(t *testing.T) {
	assert.False(t, distributionSale().Underwater())
}

// A service with no marginal cost is a legitimate zero, not an error.
func TestSale_ZeroCostIsValid(t *testing.T) {
	s := &Sale{
		ContactID: "c1", GrossCents: 5_000_00, SoldAt: time.Now(),
	}

	assert.Empty(t, s.Validate())
	assert.Equal(t, int64(5_000_00), s.NetCents())
}

// ── Validation ────────────────────────────────────────────────────────────

func TestSale_RefusesASaleWithNoCustomer(t *testing.T) {
	s := distributionSale()
	s.ContactID = ""

	assert.Contains(t, s.Validate(), "informe o cliente")
}

// Zero would sit in every report as a sale that happened and earned nothing.
func TestSale_RefusesAZeroValue(t *testing.T) {
	s := distributionSale()
	s.GrossCents = 0

	assert.Contains(t, s.Validate(), "informe o valor da venda")
}

func TestSale_RefusesNegativeCostOrTax(t *testing.T) {
	s := distributionSale()
	s.CostCents = -1

	assert.Contains(t, s.Validate(), "custo e imposto não podem ser negativos")
}

// A form reports everything at once rather than one problem per submit.
func TestSale_ReportsEveryProblemTogether(t *testing.T) {
	problems := (&Sale{}).Validate()

	assert.Len(t, problems, 3)
}

// ── Repurchase ────────────────────────────────────────────────────────────

// The single field the repurchase engine runs on.
func TestSale_NextPurchaseDueFollowsTheCycle(t *testing.T) {
	s := distributionSale()
	s.CycleDays = 30

	due := s.NextPurchaseDue()

	require.NotNil(t, due)
	assert.Equal(t, time.Date(2026, 4, 9, 0, 0, 0, 0, time.UTC), *due)
}

// A guess would put a real customer on a follow-up list on a date nobody
// chose.
func TestSale_NoCycleMeansNoPrediction(t *testing.T) {
	assert.Nil(t, distributionSale().NextPurchaseDue())
}

// ── Period totals ─────────────────────────────────────────────────────────

// Never a single "revenue" number: a report that shows only gross is the one
// that makes a discounted month look like a good one.
func TestSaleTotals_NetIsWhatTheCompanyKept(t *testing.T) {
	totals := SaleTotals{
		Count: 3, GrossCents: 300_000_00, CostCents: 216_000_00, TaxCents: 42_000_00,
	}

	assert.Equal(t, int64(42_000_00), totals.NetCents())
}
