package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SDR analytics against a real database (§6.12, I1).
//
// The property worth an integration test is that the FUNNEL and the DRILL-DOWN
// read the same facts: "40 abordados" has to open onto exactly 40 rows. They are
// separate SQL statements, so only a test against real rows can keep them from
// drifting — and the day they drift, every number on the screen stops being
// trusted.

// sdrFixture seeds one funnel with leads at known rungs and returns the ids.
type sdrFixture struct {
	accountID string
	funnelID  string
	// leads maps a readable name to its id, so assertions read like the story.
	leads map[string]string
}

func seedSDRFixture(t *testing.T, accountID string) sdrFixture {
	t.Helper()
	ctx := context.Background()

	funnelID := ulid.Make().String()
	_, err := testPool.Exec(ctx,
		`INSERT INTO sales_funnels (id, account_id, name, created_at, updated_at)
		 VALUES ($1, $2, 'Clínicas SP', NOW(), NOW())`,
		funnelID, accountID)
	require.NoError(t, err)

	strategyID := ulid.Make().String()
	_, err = testPool.Exec(ctx,
		`INSERT INTO strategies (id, account_id, name, version, status)
		 VALUES ($1, $2, 'Comodato primeiro', 2, 'testing')`,
		strategyID, accountID)
	require.NoError(t, err)

	fx := sdrFixture{accountID: accountID, funnelID: funnelID, leads: map[string]string{}}

	// Six leads, each at a different depth. The names say where each one got
	// to, so a failing assertion reads as a sentence.
	seed := []struct {
		name       string
		status     string
		qualified  bool
		approached bool
		replied    bool
		enriched   bool
		strategy   bool
	}{
		{name: "só entrou", status: "new"},
		{name: "pesquisado", status: "enriched", enriched: true},
		{name: "abordado", status: "contacted", approached: true, strategy: true},
		{name: "respondeu", status: "responded", approached: true, replied: true, strategy: true},
		{name: "qualificado", status: "qualified", approached: true, replied: true, qualified: true, strategy: true},
		{name: "ganhou", status: "won", approached: true, replied: true, qualified: true, strategy: true},
	}

	for i, s := range seed {
		contactID := seedSaleContact(t, accountID)
		leadID := ulid.Make().String()
		fx.leads[s.name] = leadID

		var qualifiedAt any
		if s.qualified {
			qualifiedAt = time.Now().Add(-time.Hour)
		}
		var stratID any
		var stratVersion any
		if s.strategy {
			stratID = strategyID
			stratVersion = 2
		}
		// Created a few hours ago so every preset window contains them, and
		// staggered so the newest-first ordering is deterministic.
		_, err := testPool.Exec(ctx,
			`INSERT INTO leads (id, account_id, funnel_id, contact_id, status, source,
			                    qualified_at, strategy_id, strategy_version, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, 'pncp', $6, $7, $8, NOW() - ($9 || ' minutes')::interval, NOW())`,
			leadID, accountID, funnelID, contactID, s.status, qualifiedAt,
			stratID, stratVersion, fmt.Sprint(10+i))
		require.NoError(t, err)

		if s.enriched {
			seedSDREvent(t, accountID, leadID, "enrichment_run", "")
		}
		if s.approached {
			seedSDREvent(t, accountID, leadID, "approach_sent", "whatsapp")
		}
		if s.replied {
			seedSDREvent(t, accountID, leadID, "reply_received", "whatsapp")
		}
	}
	return fx
}

func seedSDREvent(t *testing.T, accountID, leadID, kind, channel string) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO sales_events (id, account_id, kind, lead_id, source, channel_type, occurred_at)
		 VALUES ($1, $2, $3, $4, 'pncp', $5, NOW() - interval '1 hour')`,
		ulid.Make().String(), accountID, kind, leadID, channel)
	require.NoError(t, err)
}

func sdrReport(t *testing.T, token, period string) map[string]any {
	t.Helper()
	w := doRequest("GET", "/v1/analytics/sdr?period="+period, nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return parseJSON(t, w)
}

// funnelStep pulls one rung out of the report by its metric name.
func funnelStep(t *testing.T, report map[string]any, metric string) map[string]any {
	t.Helper()
	steps, _ := report["funnel"].([]any)
	for _, raw := range steps {
		step, _ := raw.(map[string]any)
		if step["metric"] == metric {
			return step
		}
	}
	t.Fatalf("rung %q missing from funnel %v", metric, report["funnel"])
	return nil
}

func TestSDRAnalytics_EveryNumberOpensOntoAListOfTheSameSize(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	seedSDRFixture(t, accountID)

	report := sdrReport(t, token, "30d")

	// The funnel tapers exactly as seeded: 6 in, 4 approached, 3 replied,
	// 2 qualified, 1 won.
	for metric, want := range map[string]float64{
		"entered":    6,
		"approached": 4,
		"replied":    3,
		"qualified":  2,
		"won":        1,
	} {
		step := funnelStep(t, report, metric)
		assert.Equal(t, want, step["count"], "rung %s", metric)

		// And the drill-down agrees. This is the assertion the whole test
		// exists for: the count and the list are separate SQL statements,
		// and a number that opens onto a differently-sized list makes the
		// entire screen untrustworthy.
		w := doRequest("GET", "/v1/analytics/sdr/leads?period=30d&metric="+metric, nil, token)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		leads, _ := parseJSON(t, w)["leads"].([]any)
		assert.Len(t, leads, int(want), "drill-down for %s must match its count", metric)
	}

	// Researching is NOT a rung — it is optional work, and putting it in the
	// ladder would flag every account that skips it as having broken data.
	assert.Nil(t, funnelStepIfPresent(report, "researched"),
		"researched must not be a funnel rung")
	totals, _ := report["totals"].(map[string]any)
	assert.Equal(t, float64(1), totals["researched"], "but it is still reported")
}

func funnelStepIfPresent(report map[string]any, metric string) map[string]any {
	steps, _ := report["funnel"].([]any)
	for _, raw := range steps {
		if step, _ := raw.(map[string]any); step["metric"] == metric {
			return step
		}
	}
	return nil
}

func TestSDRAnalytics_ConversionRatesAreAbsentWhenNothingHappened(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	// No leads at all: a brand-new account.

	report := sdrReport(t, token, "7d")

	entered := funnelStep(t, report, "entered")
	assert.Equal(t, float64(0), entered["count"])
	// Zero out of zero is UNKNOWN. A 0% conversion on an empty account reads
	// as "the product does not work" to somebody who has not used it yet.
	for _, metric := range []string{"approached", "replied", "qualified", "won"} {
		step := funnelStep(t, report, metric)
		assert.NotContains(t, step, "conversion_from_previous",
			"rung %s must omit its rate, not report 0%%", metric)
	}
	totals, _ := report["totals"].(map[string]any)
	assert.NotContains(t, totals, "reply_rate")
	assert.NotContains(t, totals, "avg_reply_seconds")
}

func TestSDRAnalytics_DrillNarrowsByStrategySellerAndChannel(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	fx := seedSDRFixture(t, accountID)

	// Four of the six leads carry the strategy, and all four of those were
	// approached.
	byStrategy := doRequest("GET",
		"/v1/analytics/sdr/leads?period=30d&metric=approached&channel=whatsapp", nil, token)
	require.Equal(t, http.StatusOK, byStrategy.Code, byStrategy.Body.String())
	leads, _ := parseJSON(t, byStrategy)["leads"].([]any)
	assert.Len(t, leads, 4, "the channel cut must keep the four WhatsApp approaches")

	// Nobody owns any of them, so the unowned bucket is everything and a
	// named seller is nothing. The sentinel exists because "" already means
	// "any seller".
	unowned := doRequest("GET",
		"/v1/analytics/sdr/leads?period=30d&metric=entered&seller_id=none", nil, token)
	require.Equal(t, http.StatusOK, unowned.Code)
	unownedLeads, _ := parseJSON(t, unowned)["leads"].([]any)
	assert.Len(t, unownedLeads, 6)

	named := doRequest("GET",
		"/v1/analytics/sdr/leads?period=30d&metric=entered&seller_id="+ulid.Make().String(), nil, token)
	require.Equal(t, http.StatusOK, named.Code)
	namedLeads, _ := parseJSON(t, named)["leads"].([]any)
	assert.Empty(t, namedLeads)

	// A signal source nothing came from narrows to nothing rather than
	// silently ignoring the filter.
	assert.NotEmpty(t, fx.leads)
	bogus := doRequest("GET",
		"/v1/analytics/sdr/leads?period=30d&metric=entered&signal_source=nowhere", nil, token)
	require.Equal(t, http.StatusOK, bogus.Code)
	bogusLeads, _ := parseJSON(t, bogus)["leads"].([]any)
	assert.Empty(t, bogusLeads)
}

func TestSDRAnalytics_StrategyRowsAreKeyedByVersion(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	seedSDRFixture(t, accountID)

	report := sdrReport(t, token, "30d")
	strategies, _ := report["strategies"].([]any)
	require.Len(t, strategies, 1)
	row, _ := strategies[0].(map[string]any)

	// The version travels with the row: an approach sent under version 2 must
	// never be credited to version 3's rewritten arguments.
	assert.Equal(t, float64(2), row["version"])
	assert.Equal(t, float64(4), row["approaches"])
	assert.Equal(t, float64(3), row["replied"])
	assert.Equal(t, float64(1), row["won"])
	// 3/4 replied.
	assert.InDelta(t, 0.75, row["reply_rate"], 0.001)
}

func TestSDRAnalytics_RefusesAnUnknownRungAndIsAdminOnly(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	adminToken := tokenForSeededAccount(t, accountID, "admin")

	// An unknown rung is a 400, not an empty list: an empty list reads as
	// "there are none" and hides the caller's mistake.
	bad := doRequest("GET", "/v1/analytics/sdr/leads?metric=everything", nil, adminToken)
	assert.Equal(t, http.StatusBadRequest, bad.Code, bad.Body.String())

	// The report carries per-seller revenue and commission — an attendant
	// reading it is an attendant reading a colleague's pay.
	attendantToken := tokenForSeededAccount(t, accountID, "attendant")
	forbidden := doRequest("GET", "/v1/analytics/sdr", nil, attendantToken)
	assert.Equal(t, http.StatusForbidden, forbidden.Code, forbidden.Body.String())
}

func TestSDRAnalytics_LeaksNothingAcrossAccounts(t *testing.T) {
	cleanDB(t)
	mine := seedIdentityAccount(t)
	myToken := tokenForSeededAccount(t, mine, "admin")
	seedSDRFixture(t, mine)

	// A second tenant with an identical fixture.
	theirs := seedIdentityAccount(t)
	seedSDRFixture(t, theirs)

	report := sdrReport(t, myToken, "30d")

	// Six, not twelve. Every query is account-scoped in its own WHERE clause,
	// never only through a join.
	assert.Equal(t, float64(6), funnelStep(t, report, "entered")["count"])

	w := doRequest("GET", "/v1/analytics/sdr/leads?period=30d&metric=entered", nil, myToken)
	require.Equal(t, http.StatusOK, w.Code)
	leads, _ := parseJSON(t, w)["leads"].([]any)
	assert.Len(t, leads, 6)
}
