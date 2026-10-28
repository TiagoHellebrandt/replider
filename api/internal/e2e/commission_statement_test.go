package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/auth"
	"github.com/replider/api/internal/core/domain"
)

// The monthly close against a real database (§6.6, G2).
//
// The tests are about one property above all others: a PAID line never changes.
// A rule edited today must not rewrite what was transferred last month — the
// difference has to come back as a visible adjustment, or the statement stops
// being evidence of anything.

// seedSeller inserts a user and returns (id, token) — the statement is keyed by
// the salesperson, so the tests need both.
func seedSeller(t *testing.T, accountID, role, name string) (string, string) {
	t.Helper()
	userID := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO users (id, account_id, name, email, password_hash, role, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'x', $5, true, NOW(), NOW())`,
		userID, accountID, name, userID+"@teste.local", role)
	require.NoError(t, err)

	token, err := auth.NewJWTService("e2e-test-jwt-secret", 24).GenerateToken(&domain.User{
		ID: userID, AccountID: accountID, Name: name, Role: &role,
	})
	require.NoError(t, err)
	return userID, token
}

// seedClosedSale writes a sale directly: the statement reads sales, and going
// through the HTTP endpoint would tie these tests to that handler's own rules.
func seedClosedSale(t *testing.T, accountID, contactID, sellerID string,
	gross, cost int64, soldAt time.Time) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO sales (id, account_id, contact_id, seller_user_id, items,
		    gross_cents, cost_cents, tax_cents, currency, sold_at, cycle_days,
		    source, notes, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,'[]',$5,$6,0,'brl',$7,30,'manual','',NOW(),NOW())`,
		id, accountID, contactID, sellerID, gross, cost, soldAt)
	require.NoError(t, err)
	return id
}

// lastMonth is a cycle that has already ENDED, so marking it paid needs no
// override — the open-cycle guard has its own test.
func lastMonth(t *testing.T) (string, time.Time) {
	t.Helper()
	// Mid-month, so a timezone offset cannot push the date into a neighbour.
	ref := time.Now().UTC().AddDate(0, -1, 0)
	day := time.Date(ref.Year(), ref.Month(), 15, 12, 0, 0, 0, time.UTC)
	return fmt.Sprintf("%04d-%02d", day.Year(), int(day.Month())), day
}

func seedGlobalRule(t *testing.T, token string, ratePercent float64, base string) string {
	t.Helper()
	w := doRequest("POST", "/v1/commission-rules", map[string]any{
		"name": "Padrão da casa", "scope": "global", "base": base,
		"rate_percent": ratePercent,
	}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	id, _ := parseJSON(t, w)["id"].(string)
	require.NotEmpty(t, id)
	return id
}

func statementOf(t *testing.T, body map[string]any, sellerID string) map[string]any {
	t.Helper()
	list, _ := body["statements"].([]any)
	for _, raw := range list {
		st, _ := raw.(map[string]any)
		if st["seller_user_id"] == sellerID {
			return st
		}
	}
	return nil
}

func TestCommissionStatement_RecomputeFromSalesAndRules(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	adminToken := tokenForSeededAccount(t, accountID, "admin")
	sellerID, _ := seedSeller(t, accountID, "attendant", "Joana")
	contactID := seedSaleContact(t, accountID)
	cycle, day := lastMonth(t)

	seedGlobalRule(t, adminToken, 5, "margin")
	// R$ 100.000 gross, R$ 70.000 cost → R$ 30.000 margin → 5% = R$ 1.500.
	seedClosedSale(t, accountID, contactID, sellerID, 10000000, 7000000, day)

	w := doRequest("POST", "/v1/commissions/statements/recompute?cycle="+cycle, nil, adminToken)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := parseJSON(t, w)
	assert.EqualValues(t, 150000, body["total_pending_cents"])

	st := statementOf(t, body, sellerID)
	require.NotNil(t, st)
	assert.Equal(t, "Joana", st["seller_name"])
	assert.EqualValues(t, 1, st["sales_count"])
	assert.EqualValues(t, 150000, st["commission_cents"])
	assert.EqualValues(t, 0, st["paid_cents"])

	// Every line says why it paid what it paid — the answer to "por quê?" has
	// to be on the row, not in somebody's memory.
	entries, _ := st["entries"].([]any)
	require.Len(t, entries, 1)
	line, _ := entries[0].(map[string]any)
	assert.NotEmpty(t, line["explanation"])
	assert.Equal(t, "pending", line["status"])

	// Idempotent: running the close twice does not double the month.
	again := doRequest("POST", "/v1/commissions/statements/recompute?cycle="+cycle, nil, adminToken)
	require.Equal(t, http.StatusOK, again.Code, again.Body.String())
	assert.EqualValues(t, 150000, parseJSON(t, again)["total_pending_cents"])
}

func TestCommissionStatement_PaidLinesSurviveARuleChange(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	adminToken := tokenForSeededAccount(t, accountID, "admin")
	sellerID, _ := seedSeller(t, accountID, "attendant", "Joana")
	contactID := seedSaleContact(t, accountID)
	cycle, day := lastMonth(t)

	ruleID := seedGlobalRule(t, adminToken, 5, "margin")
	seedClosedSale(t, accountID, contactID, sellerID, 10000000, 7000000, day)
	require.Equal(t, http.StatusOK,
		doRequest("POST", "/v1/commissions/statements/recompute?cycle="+cycle, nil, adminToken).Code)

	paid := doRequest("POST", "/v1/commissions/statements/pay", map[string]any{
		"cycle": cycle, "seller_user_id": sellerID, "note": "PIX",
	}, adminToken)
	require.Equal(t, http.StatusOK, paid.Code, paid.Body.String())
	assert.EqualValues(t, 150000, parseJSON(t, paid)["paid_cents"])
	assert.EqualValues(t, 0, parseJSON(t, paid)["pending_cents"])

	// Paying twice has nothing left to settle, and says so rather than
	// silently reporting another transfer.
	assert.Equal(t, http.StatusConflict,
		doRequest("POST", "/v1/commissions/statements/pay", map[string]any{
			"cycle": cycle, "seller_user_id": sellerID,
		}, adminToken).Code)

	// The rate doubles AFTER the money left. The paid line must not change;
	// the difference appears as an adjustment on the same cycle.
	upd := doRequest("PUT", "/v1/commission-rules/"+ruleID, map[string]any{
		"name": "Padrão da casa", "scope": "global", "base": "margin",
		"rate_percent": 10,
	}, adminToken)
	require.Equal(t, http.StatusOK, upd.Code, upd.Body.String())

	after := doRequest("POST", "/v1/commissions/statements/recompute?cycle="+cycle, nil, adminToken)
	require.Equal(t, http.StatusOK, after.Code, after.Body.String())
	st := statementOf(t, parseJSON(t, after), sellerID)
	require.NotNil(t, st)

	assert.EqualValues(t, 150000, st["paid_cents"], "settled money must not move")
	// 10% of R$ 30.000 = R$ 3.000; R$ 1.500 already went out.
	assert.EqualValues(t, 150000, st["adjustment_cents"])
	assert.EqualValues(t, 150000, st["pending_cents"])
	assert.EqualValues(t, 300000, st["total_cents"])
}

func TestCommissionStatement_AdjustmentsAndOpenCycleGuard(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	adminToken := tokenForSeededAccount(t, accountID, "admin")
	sellerID, sellerToken := seedSeller(t, accountID, "attendant", "Joana")
	contactID := seedSaleContact(t, accountID)

	seedGlobalRule(t, adminToken, 5, "margin")
	// A sale in the CURRENT month, so the open-cycle guard applies.
	seedClosedSale(t, accountID, contactID, sellerID, 10000000, 7000000, time.Now().UTC())
	require.Equal(t, http.StatusOK,
		doRequest("POST", "/v1/commissions/statements/recompute", nil, adminToken).Code)

	// Paying a month that has not ended is legitimate but must be deliberate:
	// sales landing after the transfer produce lines nobody budgeted.
	blocked := doRequest("POST", "/v1/commissions/statements/pay", map[string]any{
		"seller_user_id": sellerID,
	}, adminToken)
	require.Equal(t, http.StatusConflict, blocked.Code, blocked.Body.String())
	assert.Equal(t, "COMMISSION_CYCLE_STILL_OPEN", parseJSON(t, blocked)["code"])

	// An adjustment with no reason is refused: a number nobody explained is a
	// number nobody can defend next month.
	assert.Equal(t, http.StatusBadRequest,
		doRequest("POST", "/v1/commissions/statements/adjust", map[string]any{
			"seller_user_id": sellerID, "amount_cents": 50000,
		}, adminToken).Code)

	adj := doRequest("POST", "/v1/commissions/statements/adjust", map[string]any{
		"seller_user_id": sellerID, "amount_cents": -20000,
		"note": "Devolução do pedido 881",
	}, adminToken)
	require.Equal(t, http.StatusOK, adj.Code, adj.Body.String())

	// A recomputation must NOT wipe the human's correction.
	after := doRequest("POST", "/v1/commissions/statements/recompute", nil, adminToken)
	require.Equal(t, http.StatusOK, after.Code, after.Body.String())
	st := statementOf(t, parseJSON(t, after), sellerID)
	require.NotNil(t, st)
	assert.EqualValues(t, -20000, st["adjustment_cents"])
	assert.EqualValues(t, 130000, st["pending_cents"])

	// The salesperson sees their own number and nothing else.
	mine := doRequest("GET", "/v1/commissions/mine", nil, sellerToken)
	require.Equal(t, http.StatusOK, mine.Code, mine.Body.String())
	assert.EqualValues(t, 130000, parseJSON(t, mine)["pending_cents"])

	// And the statements themselves stay out of reach.
	assert.Equal(t, http.StatusForbidden,
		doRequest("GET", "/v1/commissions/statements", nil, sellerToken).Code)
	assert.Equal(t, http.StatusForbidden,
		doRequest("POST", "/v1/commissions/statements/pay", map[string]any{
			"seller_user_id": sellerID,
		}, sellerToken).Code)

	// Advancing an open month works once it is asked for explicitly.
	advanced := doRequest("POST", "/v1/commissions/statements/pay", map[string]any{
		"seller_user_id": sellerID, "allow_open_cycle": true, "note": "adiantamento",
	}, adminToken)
	require.Equal(t, http.StatusOK, advanced.Code, advanced.Body.String())
	assert.EqualValues(t, 130000, parseJSON(t, advanced)["paid_cents"])
}

func TestCommissionStatement_EstimateUsesTheCallersOwnRule(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	adminToken := tokenForSeededAccount(t, accountID, "admin")
	sellerID, sellerToken := seedSeller(t, accountID, "attendant", "Joana")

	// No rule yet: the answer is "nenhuma regra cobre isso", never zero.
	empty := doRequest("GET", "/v1/commissions/estimate?gross_cents=1000000", nil, sellerToken)
	require.Equal(t, http.StatusOK, empty.Code, empty.Body.String())
	assert.Nil(t, parseJSON(t, empty)["amount_cents"])

	seedGlobalRule(t, adminToken, 5, "margin")
	// House rule: 5% of a 30% margin on R$ 10.000 = R$ 150.
	house := doRequest("GET", "/v1/commissions/estimate?gross_cents=1000000&margin_percent=30", nil, sellerToken)
	require.Equal(t, http.StatusOK, house.Code, house.Body.String())
	assert.EqualValues(t, 15000, parseJSON(t, house)["amount_cents"])

	// An exception for THIS salesperson wins: somebody on a different rate must
	// never be shown the house number, because they remember what they saw.
	own := doRequest("POST", "/v1/commission-rules", map[string]any{
		"name": "Veterana", "scope": "seller", "scope_id": sellerID,
		"base": "margin", "rate_percent": 10,
	}, adminToken)
	require.Equal(t, http.StatusOK, own.Code, own.Body.String())

	mine := doRequest("GET", "/v1/commissions/estimate?gross_cents=1000000&margin_percent=30", nil, sellerToken)
	require.Equal(t, http.StatusOK, mine.Code, mine.Body.String())
	assert.EqualValues(t, 30000, parseJSON(t, mine)["amount_cents"])
}

func TestCommissionStatement_SplitsFanOutAndCSVCarriesTheRow(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	adminToken := tokenForSeededAccount(t, accountID, "admin")
	sellerID, _ := seedSeller(t, accountID, "attendant", "Joana")
	contactID := seedSaleContact(t, accountID)
	cycle, day := lastMonth(t)

	rule := doRequest("POST", "/v1/commission-rules", map[string]any{
		"name": "Com indicação", "scope": "global", "base": "margin",
		"rate_percent": 5,
		"splits": []map[string]any{
			{"role": "closer", "share_percent": 70},
			{"role": "quem indicou", "share_percent": 30},
		},
	}, adminToken)
	require.Equal(t, http.StatusOK, rule.Code, rule.Body.String())
	seedClosedSale(t, accountID, contactID, sellerID, 10000000, 7000000, day)

	w := doRequest("POST", "/v1/commissions/statements/recompute?cycle="+cycle, nil, adminToken)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	st := statementOf(t, parseJSON(t, w), sellerID)
	require.NotNil(t, st)

	// The parts add up to the whole: R$ 1.050 + R$ 450 = R$ 1.500.
	assert.EqualValues(t, 150000, st["commission_cents"])
	entries, _ := st["entries"].([]any)
	require.Len(t, entries, 2)
	roles := map[string]int64{}
	for _, raw := range entries {
		e, _ := raw.(map[string]any)
		role, _ := e["role"].(string)
		amount, _ := e["amount_cents"].(float64)
		roles[role] = int64(amount)
	}
	assert.EqualValues(t, 105000, roles["closer"])
	assert.EqualValues(t, 45000, roles["quem indicou"])

	// The export is what gets reconciled against the bank, so every line — not
	// just the total — has to be in it, in reais.
	csvResp := doRequest("GET", "/v1/commissions/statements/export.csv?cycle="+cycle, nil, adminToken)
	require.Equal(t, http.StatusOK, csvResp.Code)
	assert.Contains(t, csvResp.Header().Get("Content-Type"), "text/csv")
	body := csvResp.Body.String()
	assert.Contains(t, body, "Joana")
	assert.Contains(t, body, "1050,00")
	assert.Contains(t, body, "450,00")
	assert.Equal(t, 3, strings.Count(strings.TrimSpace(body), "\n")+1,
		"header plus one row per line")
}

func TestCommissionStatement_TenancyAndBadCycle(t *testing.T) {
	cleanDB(t)
	accountA := seedIdentityAccount(t)
	accountB := seedIdentityAccount(t)
	tokenA := tokenForSeededAccount(t, accountA, "admin")
	tokenB := tokenForSeededAccount(t, accountB, "admin")
	sellerID, _ := seedSeller(t, accountA, "attendant", "Joana")
	contactID := seedSaleContact(t, accountA)
	cycle, day := lastMonth(t)

	seedGlobalRule(t, tokenA, 5, "margin")
	seedClosedSale(t, accountA, contactID, sellerID, 10000000, 7000000, day)
	require.Equal(t, http.StatusOK,
		doRequest("POST", "/v1/commissions/statements/recompute?cycle="+cycle, nil, tokenA).Code)

	// Another company's payroll is not visible, and recomputing from their side
	// produces nothing rather than reading across the boundary.
	other := doRequest("GET", "/v1/commissions/statements?cycle="+cycle, nil, tokenB)
	require.Equal(t, http.StatusOK, other.Code, other.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(other.Body.Bytes(), &body))
	list, _ := body["statements"].([]any)
	assert.Empty(t, list)
	assert.EqualValues(t, 0, body["total_pending_cents"])

	// A malformed cycle is refused rather than silently answered for "now".
	bad := doRequest("GET", "/v1/commissions/statements?cycle=agosto", nil, tokenA)
	assert.Equal(t, http.StatusBadRequest, bad.Code, bad.Body.String())
}
