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

// Command centre against a real database (§6.13, I2).
//
// The screen's whole claim is that the top row is the thing most worth doing. The
// ranking is domain logic (unit-tested), but the ROWS come from six independent
// queries reaching six different tables for a customer name — so only a test
// against real rows proves that the money, the deadline and the name actually
// arrive attached to the right action.

func commandCenter(t *testing.T, token string) map[string]any {
	t.Helper()
	w := doRequest("GET", "/v1/me/command-center", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return parseJSON(t, w)
}

func ccActions(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()
	raw, _ := payload["actions"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		a, _ := r.(map[string]any)
		out = append(out, a)
	}
	return out
}

// seedUnconfirmedSale is money already earned and not yet counted: an
// AI-extracted sale no human has signed off. (These tests used to drive the
// ranking through signature/contract rows — kinds cut with their screens on
// 2026-08-18, since their deep links landed on NotFound.)
func seedUnconfirmedSale(t *testing.T, accountID, contactID string, grossCents int64) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO sales (id, account_id, contact_id, gross_cents, cost_cents,
		                    tax_cents, currency, source, sold_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 0, 0, 'brl', 'ai_extracted', NOW(), NOW(), NOW())`,
		id, accountID, contactID, grossCents)
	require.NoError(t, err)
	return id
}

// seedOverdueReorder is a deadline that fires whether or not anybody looks:
// a reorder nudge whose due date passed.
func seedOverdueReorder(t *testing.T, accountID, contactID string, grossCents int64, dueOffsetDays int) string {
	t.Helper()
	ctx := context.Background()
	saleID := ulid.Make().String()
	_, err := testPool.Exec(ctx,
		`INSERT INTO sales (id, account_id, contact_id, gross_cents, cost_cents,
		                    tax_cents, currency, sold_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 0, 0, 'brl', NOW() - interval '60 days', NOW(), NOW())`,
		saleID, accountID, contactID, grossCents)
	require.NoError(t, err)

	nudgeID := ulid.Make().String()
	_, err = testPool.Exec(ctx,
		`INSERT INTO reorder_nudges (id, account_id, contact_id, sale_id, due_at, created_at)
		 VALUES ($1, $2, $3, $4, NOW() + ($5 || ' days')::interval, NOW())`,
		nudgeID, accountID, contactID, saleID, fmt.Sprint(dueOffsetDays))
	require.NoError(t, err)
	return nudgeID
}

func seedPendingDraft(t *testing.T, accountID, funnelID, contactID string, ageDays int) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO leads (id, account_id, funnel_id, contact_id, status, source,
		                    approach_draft, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'pending_approval', 'pncp', 'Olá...',
		         NOW() - ($5 || ' days')::interval,
		         NOW() - ($5 || ' days')::interval)`,
		id, accountID, funnelID, contactID, fmt.Sprint(ageDays))
	require.NoError(t, err)
	return id
}

func seedCCFunnel(t *testing.T, accountID string) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO sales_funnels (id, account_id, name, created_at, updated_at)
		 VALUES ($1, $2, 'Clínicas', NOW(), NOW())`,
		id, accountID)
	require.NoError(t, err)
	return id
}

func TestCommandCenter_TheTopRowIsTheThingMostWorthDoing(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	funnelID := seedCCFunnel(t, accountID)

	contact := seedSaleContact(t, accountID)
	// Three things need doing: a stale draft, a R$900 unconfirmed sale, and
	// a R$100.000 one.
	seedPendingDraft(t, accountID, funnelID, seedSaleContact(t, accountID), 5)
	seedUnconfirmedSale(t, accountID, seedSaleContact(t, accountID), 90000)
	big := seedUnconfirmedSale(t, accountID, contact, 10000000)

	payload := commandCenter(t, token)
	actions := ccActions(t, payload)
	require.NotEmpty(t, actions)

	// Impact, not recency: the R$100.000 document somebody opened comes first.
	assert.Equal(t, big, actions[0]["target_id"])
	assert.Equal(t, "confirm_sale", actions[0]["kind"])
	assert.Equal(t, float64(10000000), actions[0]["money_at_stake_cents"])
	// And it names the customer. "3 assinaturas pendentes" is a statistic;
	// "Clínica Bem Viver — R$ 100.000" is a phone call.
	assert.Equal(t, "Clínica Bem Viver", actions[0]["customer_name"])
	// With somewhere to go.
	assert.NotEmpty(t, actions[0]["deep_link"])
}

func TestCommandCenter_AnOverdueDeadlineOutranksABiggerOpportunity(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	seedUnconfirmedSale(t, accountID, seedSaleContact(t, accountID), 10000000)
	overdue := seedOverdueReorder(t, accountID, seedSaleContact(t, accountID), 500000, -2)

	actions := ccActions(t, commandCenter(t, token))
	require.NotEmpty(t, actions)

	// A renewal that fired two days ago cannot be un-fired by a bigger deal
	// elsewhere.
	assert.Equal(t, overdue, actions[0]["target_id"])
	assert.Equal(t, "reorder_due", actions[0]["kind"])
}

func TestCommandCenter_TilesCountAndNameTheirPeriod(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	funnelID := seedCCFunnel(t, accountID)

	seedPendingDraft(t, accountID, funnelID, seedSaleContact(t, accountID), 1)
	contact := seedSaleContact(t, accountID)
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO leads (id, account_id, funnel_id, contact_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'new', NOW(), NOW())`,
		ulid.Make().String(), accountID, funnelID, contact)
	require.NoError(t, err)

	// A sale this month and a meeting this month.
	saleID := ulid.Make().String()
	_, err = testPool.Exec(context.Background(),
		`INSERT INTO sales (id, account_id, contact_id, gross_cents, cost_cents,
		                    tax_cents, currency, sold_at, created_at, updated_at)
		 VALUES ($1, $2, $3, 5000000, 0, 0, 'brl', NOW(), NOW(), NOW())`,
		saleID, accountID, contact)
	require.NoError(t, err)
	_, err = testPool.Exec(context.Background(),
		`INSERT INTO sales_events (id, account_id, kind, occurred_at)
		 VALUES ($1, $2, 'meeting_booked', NOW())`,
		ulid.Make().String(), accountID)
	require.NoError(t, err)

	payload := commandCenter(t, token)
	tiles, _ := payload["tiles"].(map[string]any)
	require.NotNil(t, tiles)

	assert.Equal(t, float64(2), tiles["active_leads"], "new + pending_approval")
	assert.Equal(t, float64(1), tiles["pending_approval"])
	assert.Equal(t, float64(1), tiles["meetings"])
	assert.Equal(t, float64(5000000), tiles["won_gross_cents"])
	// "Ganhos" with no window is a number nobody can check.
	assert.Equal(t, time.Now().Format("2006-01"), tiles["period_label"])
}

func TestCommandCenter_TheMiniFunnelIsTheBoardNotTheQueue(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	funnelID := seedCCFunnel(t, accountID)
	seedPendingDraft(t, accountID, funnelID, seedSaleContact(t, accountID), 1)

	payload := commandCenter(t, token)
	stages, _ := payload["funnel"].([]any)
	require.NotEmpty(t, stages)

	for _, raw := range stages {
		stage, _ := raw.(map[string]any)
		// pending_approval lasts minutes and belongs to the queue. As a bar it
		// would make the board's shape a lie.
		assert.NotEqual(t, "pending_approval", stage["status"])
	}
}

func TestCommandCenter_ASnoozeHidesOneRowForOneUserAndAlwaysExpires(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	otherToken := tokenForSeededAccount(t, accountID, "admin")

	sig := seedUnconfirmedSale(t, accountID, seedSaleContact(t, accountID), 10000000)
	require.Len(t, ccActions(t, commandCenter(t, token)), 1)

	w := doRequest("POST", "/v1/me/pending-actions/dismiss", map[string]any{
		"kind": "confirm_sale", "target_id": sig, "snooze_hours": 24,
	}, token)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())

	// Gone for the person who judged it.
	assert.Empty(t, ccActions(t, commandCenter(t, token)))
	// Still there for everybody else: hiding a R$100.000 deadline from a whole
	// company because one person swiped it away is how a deadline gets missed.
	assert.Len(t, ccActions(t, commandCenter(t, otherToken)), 1)

	// And the snooze has a real deadline in the database — there is no forever.
	var until time.Time
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT snoozed_until FROM pending_action_dismissals
		 WHERE account_id = $1 AND kind = 'confirm_sale' AND target_id = $2`,
		accountID, sig).Scan(&until))
	assert.True(t, until.After(time.Now()))
	assert.True(t, until.Before(time.Now().Add(48*time.Hour)))
}

func TestCommandCenter_ReSnoozingMovesTheDeadlineInsteadOfStacking(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	sig := seedUnconfirmedSale(t, accountID, seedSaleContact(t, accountID), 100)

	for _, hours := range []int{24, 72} {
		w := doRequest("POST", "/v1/me/pending-actions/dismiss", map[string]any{
			"kind": "confirm_sale", "target_id": sig, "snooze_hours": hours,
		}, token)
		require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	}

	var rows int
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM pending_action_dismissals WHERE account_id = $1`,
		accountID).Scan(&rows))
	// "It came back, the answer is still not now" is the common case; a pile of
	// historical dismissals would make the filter read grow forever.
	assert.Equal(t, 1, rows)
}

func TestCommandCenter_RefusesASnoozeItCannotHonour(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	cases := []struct {
		name string
		body map[string]any
	}{
		{"unknown kind", map[string]any{"kind": "whatever", "target_id": "x"}},
		{"no target", map[string]any{"kind": "confirm_sale"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doRequest("POST", "/v1/me/pending-actions/dismiss", tc.body, token)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
}

func TestCommandCenter_AnEmptyAccountSaysSoInsteadOfShowingZeros(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	payload := commandCenter(t, token)

	// A brand-new account reading "0 · 0 · 0 · R$ 0" learns that the product
	// does nothing. The flag is what lets the client show activation instead.
	assert.Equal(t, true, payload["empty"])
	// And the empty state is exactly when a suggestion earns its place.
	assert.NotNil(t, payload["suggestion"])
}

func TestCommandCenter_LeaksNothingAcrossAccounts(t *testing.T) {
	cleanDB(t)
	mine := seedIdentityAccount(t)
	myToken := tokenForSeededAccount(t, mine, "admin")
	seedUnconfirmedSale(t, mine, seedSaleContact(t, mine), 100000)

	theirs := seedIdentityAccount(t)
	seedUnconfirmedSale(t, theirs, seedSaleContact(t, theirs), 99900000)
	seedOverdueReorder(t, theirs, seedSaleContact(t, theirs), 99900000, -5)

	actions := ccActions(t, commandCenter(t, myToken))

	// One row, mine. Their overdue R$999k must not reach the top of my screen.
	require.Len(t, actions, 1)
	assert.Equal(t, float64(100000), actions[0]["money_at_stake_cents"])
}
