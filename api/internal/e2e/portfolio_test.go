package e2e

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The portfolio against a real database. The predicted date is the whole
// feature, and the query that produces it is the part worth pinning.

func seedSaleRow(t *testing.T, accountID, contactID string, soldAgo time.Duration, cycleDays int) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO sales (id, account_id, contact_id, items, gross_cents,
		    cost_cents, tax_cents, currency, sold_at, cycle_days, source)
		 VALUES ($1,$2,$3,$4,10000000,7200000,1400000,'brl',NOW() - $5::interval,$6,'manual')`,
		id, accountID, contactID,
		`[{"description":"Luvas de procedimento cx 100","quantity":200}]`,
		soldAgo.String(), cycleDays)
	require.NoError(t, err)
	return id
}

func TestPortfolio_ListsCustomersWhoBought(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	contactID := seedSaleContact(t, accountID)
	seedSaleRow(t, accountID, contactID, 10*24*time.Hour, 30)

	list := parseJSONArray(t, doRequest("GET", "/v1/portfolio", nil, token))

	require.Len(t, list, 1)
	row := list[0].(map[string]any)
	assert.EqualValues(t, 1, row["purchases"])
	// Net leads over gross: what a customer is worth is what the company
	// kept.
	assert.EqualValues(t, 1400000, row["net_cents"])
	// Bought 10 days ago on a 30-day cycle: 20 days to go.
	assert.EqualValues(t, 19, row["days_until_due"])
}

// A guessed date puts a real customer on a follow-up list on a day nobody
// chose.
func TestPortfolio_NoCycleMeansNoPrediction(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	seedSaleRow(t, accountID, seedSaleContact(t, accountID), 10*24*time.Hour, 0)

	list := parseJSONArray(t, doRequest("GET", "/v1/portfolio", nil, token))

	require.Len(t, list, 1)
	assert.Nil(t, list[0].(map[string]any)["days_until_due"])
}

// The person opening a portfolio has an hour, and the question is who to call
// FIRST — so the order is decided in SQL, before pagination.
func TestPortfolio_MostUrgentComesFirst(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	comfortable := seedSaleContact(t, accountID)
	seedSaleRow(t, accountID, comfortable, 5*24*time.Hour, 90)
	overdue := seedSaleContact(t, accountID)
	seedSaleRow(t, accountID, overdue, 60*24*time.Hour, 30)

	list := parseJSONArray(t, doRequest("GET", "/v1/portfolio", nil, token))

	require.Len(t, list, 2)
	assert.Equal(t, overdue, list[0].(map[string]any)["contact_id"])
	// Negative, not clamped: "acabou há 30 dias" is the most actionable
	// thing on the screen.
	assert.Negative(t, list[0].(map[string]any)["days_until_due"].(float64))
}

// One customer, everything they bought.
func TestPortfolio_DetailCarriesTheHistory(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	contactID := seedSaleContact(t, accountID)
	seedSaleRow(t, accountID, contactID, 40*24*time.Hour, 30)
	seedSaleRow(t, accountID, contactID, 10*24*time.Hour, 30)

	detail := parseJSON(t, doRequest("GET", "/v1/portfolio/"+contactID, nil, token))

	assert.EqualValues(t, 2, detail["purchases"])
	sales, _ := detail["sales"].([]any)
	assert.Len(t, sales, 2)
}

// One account's customers are not another's.
func TestPortfolio_IsScopedToTheAccount(t *testing.T) {
	cleanDB(t)
	owner := seedIdentityAccount(t)
	seedSaleRow(t, owner, seedSaleContact(t, owner), 5*24*time.Hour, 30)
	other := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/portfolio", nil, other)))
}

// ── Reorders (F3) ─────────────────────────────────────────────────────────

func TestPortfolio_UpcomingReordersListsWhatIsRunningOut(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	// Due in three days: inside the lead-time window.
	soon := seedSaleContact(t, accountID)
	seedSaleRow(t, accountID, soon, 27*24*time.Hour, 30)
	// Due in fifty days: known, not yet worth a call.
	later := seedSaleContact(t, accountID)
	seedSaleRow(t, accountID, later, 10*24*time.Hour, 60)

	list := parseJSONArray(t,
		doRequest("GET", "/v1/portfolio/reorders", nil, token))

	require.Len(t, list, 1)
	assert.Equal(t, soon, list[0].(map[string]any)["contact_id"])
}

// Asking somebody to buy what they just bought is worse than not asking.
func TestPortfolio_ANewerPurchaseCancelsTheReorder(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	contactID := seedSaleContact(t, accountID)

	seedSaleRow(t, accountID, contactID, 30*24*time.Hour, 30) // due now
	seedSaleRow(t, accountID, contactID, 1*24*time.Hour, 30)  // already reordered

	list := parseJSONArray(t,
		doRequest("GET", "/v1/portfolio/reorders", nil, token))

	assert.Empty(t, list)
}

// The screen must only ever see its own account, and the filter runs in SQL
// so a busy deployment does not hide one tenant behind another's page.
func TestPortfolio_ReordersAreScopedToTheAccount(t *testing.T) {
	cleanDB(t)
	owner := seedIdentityAccount(t)
	seedSaleRow(t, owner, seedSaleContact(t, owner), 29*24*time.Hour, 30)
	other := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	assert.Empty(t, parseJSONArray(t,
		doRequest("GET", "/v1/portfolio/reorders", nil, other)))
}

// ── Product cycles ────────────────────────────────────────────────────────

func seedProduct(t *testing.T, accountID string) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO products (id, account_id, name, description, price_cents,
		    currency, attributes, active, created_at, updated_at)
		 VALUES ($1,$2,'Luvas','',5000,'brl','{}',true,NOW(),NOW())`,
		id, accountID)
	require.NoError(t, err)
	return id
}

func TestProductCycles_SetAndRead(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	productID := seedProduct(t, accountID)

	require.Equal(t, http.StatusNoContent,
		doRequest("PUT", "/v1/products/"+productID+"/cycles", map[string]any{
			"consumption_cycle_days": 30, "shelf_life_days": 730,
		}, token).Code)

	var consumption, shelf int
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT consumption_cycle_days, shelf_life_days FROM products WHERE id = $1`,
		productID).Scan(&consumption, &shelf))
	assert.Equal(t, 30, consumption)
	assert.Equal(t, 730, shelf)
}

// A cycle of 99999 days silently removes a product from the reorder engine
// forever, which is a typo nobody would ever notice.
func TestProductCycles_RefuseAnAbsurdCycle(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	productID := seedProduct(t, accountID)

	w := doRequest("PUT", "/v1/products/"+productID+"/cycles",
		map[string]any{"consumption_cycle_days": 99999}, token)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProductCycles_RefuseAnotherAccountsProduct(t *testing.T) {
	cleanDB(t)
	stranger := seedProduct(t, seedIdentityAccount(t))
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	w := doRequest("PUT", "/v1/products/"+stranger+"/cycles",
		map[string]any{"consumption_cycle_days": 30}, token)

	// 404, not 403: another account's product is indistinguishable from one
	// that does not exist, which is the answer that leaks nothing.
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "PRODUCT_NOT_FOUND")
}
