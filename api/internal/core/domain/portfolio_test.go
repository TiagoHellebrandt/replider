package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Consumables are recurring revenue: a customer whose repurchase works does
// not churn, and one who has to remember to reorder eventually remembers a
// competitor instead. The whole feature is one number — when they run out.

var portfolioNow = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

func customer(dueIn time.Duration, silentFor time.Duration) *PortfolioCustomer {
	due := portfolioNow.Add(dueIn)
	contact := portfolioNow.Add(-silentFor)
	return &PortfolioCustomer{
		ContactID: "c1", ContactName: "Clínica Bem Viver",
		Purchases: 4, NextDueAt: &due, LastContactAt: &contact,
	}
}

// Negative is the point: "acabou há 12 dias" is the most actionable thing on
// the screen, and clamping it to zero would hide it.
func TestPortfolio_OverdueCountsNegative(t *testing.T) {
	days := customer(-12*24*time.Hour, time.Hour).DaysUntilDue(portfolioNow)

	require.NotNil(t, days)
	assert.Equal(t, -12, *days)
}

func TestPortfolio_DaysUntilDue(t *testing.T) {
	days := customer(5*24*time.Hour, time.Hour).DaysUntilDue(portfolioNow)

	require.NotNil(t, days)
	assert.Equal(t, 5, *days)
}

// A guessed date puts a real customer on a follow-up list on a day nobody
// chose.
func TestPortfolio_NoPredictionMeansNoCountdown(t *testing.T) {
	c := customer(0, time.Hour)
	c.NextDueAt = nil

	assert.Nil(t, c.DaysUntilDue(portfolioNow))
}

// "Never spoke to them" is a different thing from "spoke long ago".
func TestPortfolio_NeverContactedIsNotZeroDays(t *testing.T) {
	c := customer(5*24*time.Hour, 0)
	c.LastContactAt = nil

	assert.Nil(t, c.DaysSinceContact(portfolioNow))
}

// ── Urgency ───────────────────────────────────────────────────────────────

// The person opening a portfolio has an hour, and the question is who to call
// FIRST. Alphabetical or by revenue is a list nobody works through.
func TestPortfolio_OverdueOutranksEverything(t *testing.T) {
	overdue := customer(-3*24*time.Hour, time.Hour)
	dueSoon := customer(2*24*time.Hour, time.Hour)
	dueLater := customer(45*24*time.Hour, time.Hour)

	assert.Greater(t, overdue.PortfolioUrgency(portfolioNow),
		dueSoon.PortfolioUrgency(portfolioNow))
	assert.Greater(t, dueSoon.PortfolioUrgency(portfolioNow),
		dueLater.PortfolioUrgency(portfolioNow))
}

// Inside the lead-time window, sooner is more urgent.
func TestPortfolio_SoonerInsideTheWindowRanksHigher(t *testing.T) {
	twoDays := customer(2*24*time.Hour, time.Hour)
	sixDays := customer(6*24*time.Hour, time.Hour)

	assert.Greater(t, twoDays.PortfolioUrgency(portfolioNow),
		sixDays.PortfolioUrgency(portfolioNow))
}

// Longer overdue is worse, but the bonus is capped: a customer who ran out
// two years ago is not four times more urgent than one who ran out last
// month — both mean the same thing, which is "we lost them".
func TestPortfolio_OverdueBonusIsBounded(t *testing.T) {
	recent := customer(-30*24*time.Hour, time.Hour)
	ancient := customer(-900*24*time.Hour, time.Hour)

	assert.Greater(t, ancient.PortfolioUrgency(portfolioNow),
		recent.PortfolioUrgency(portfolioNow))
	assert.LessOrEqual(t, ancient.PortfolioUrgency(portfolioNow), 160)
}

// Silence is its own risk, independent of any reorder date: a customer
// nobody has spoken to in four months is one a competitor is speaking to.
func TestPortfolio_SilenceAddsUrgency(t *testing.T) {
	fresh := customer(45*24*time.Hour, 3*24*time.Hour)
	silent := customer(45*24*time.Hour, 90*24*time.Hour)

	assert.Greater(t, silent.PortfolioUrgency(portfolioNow),
		fresh.PortfolioUrgency(portfolioNow))
}

// Sixty days, not thirty: shorter would flag every customer on a quarterly
// cycle as neglected, which trains people to ignore the flag.
func TestPortfolio_AQuarterlyCustomerIsNotFlaggedTooEarly(t *testing.T) {
	c := customer(45*24*time.Hour, 40*24*time.Hour)
	fresh := customer(45*24*time.Hour, time.Hour)

	assert.Equal(t, fresh.PortfolioUrgency(portfolioNow),
		c.PortfolioUrgency(portfolioNow))
}

// ── Prediction ────────────────────────────────────────────────────────────

// Version one on purpose: last quantity over the cycle. Wrong for a customer
// whose usage changed, right for the overwhelming majority — and it needs no
// history, so it works on the SECOND purchase rather than the fifth.
func TestConsumptionRate(t *testing.T) {
	rate := ConsumptionRate(200, 30)

	require.NotNil(t, rate)
	assert.InDelta(t, 6.67, *rate, 0.01)
}

// A rate derived from a guess is worse than no rate.
func TestConsumptionRate_NeedsBothNumbers(t *testing.T) {
	assert.Nil(t, ConsumptionRate(0, 30))
	assert.Nil(t, ConsumptionRate(200, 0))
}

// The sale's own cycle wins: somebody typed it about THIS customer, and the
// product default is an average across all of them.
func TestPredictRunOut_SaleCycleBeatsTheProductDefault(t *testing.T) {
	sold := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	out := PredictRunOut(sold, 15, 30)

	require.NotNil(t, out)
	assert.Equal(t, time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), *out)
}

func TestPredictRunOut_FallsBackToTheProduct(t *testing.T) {
	sold := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	out := PredictRunOut(sold, 0, 30)

	require.NotNil(t, out)
	assert.Equal(t, time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), *out)
}

// A prediction with no basis is worse than none, because somebody acts on it.
func TestPredictRunOut_NoCycleMeansNoPrediction(t *testing.T) {
	assert.Nil(t, PredictRunOut(time.Now(), 0, 0))
}
