package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sales against a real database. This record feeds a commission payout, so
// the tests are about the money being right and about tenancy.

func seedSaleContact(t *testing.T, accountID string) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO contacts (id, account_id, external_id, name, created_at, updated_at)
		 VALUES ($1, $2, $3, 'Clínica Bem Viver', NOW(), NOW())`,
		// The ULID tail, not the head: the head is a timestamp, and two
		// contacts seeded in the same millisecond would collide on the
		// unique (account, external_id) index.
		id, accountID, "+55"+ulid.Make().String()[15:])
	require.NoError(t, err)
	return id
}

func distributionSaleBody(contactID string) map[string]any {
	return map[string]any{
		"contact_id":  contactID,
		"gross_cents": 10000000, // R$ 100.000
		"cost_cents":  7200000,
		"tax_cents":   1400000,
		"cycle_days":  30,
		"items": []map[string]any{
			{"description": "Luvas de procedimento cx 100", "quantity": 200},
		},
	}
}

func TestSales_RecordAndRead(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	contactID := seedSaleContact(t, accountID)

	w := doRequest("POST", "/v1/sales", distributionSaleBody(contactID), token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	created := parseJSON(t, w)
	// The number a commission should be calculated on, computed once on the
	// server so the client cannot disagree with it.
	assert.EqualValues(t, 1400000, created["net_cents"])
	assert.Equal(t, false, created["underwater"])
	// The repurchase engine's date, derived from the cycle.
	assert.NotEmpty(t, created["next_purchase_due"])

	assert.Len(t, parseJSONArray(t, doRequest("GET", "/v1/sales", nil, token)), 1)
}

// Surfaced, never refused: only a person can tell a loss leader from a typo.
// But nobody should find out about one in a commission statement.
func TestSales_UnderwaterIsRecordedAndFlagged(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	body := distributionSaleBody(seedSaleContact(t, accountID))
	body["cost_cents"] = 9500000

	created := parseJSON(t, doRequest("POST", "/v1/sales", body, token))

	assert.Equal(t, true, created["underwater"])
	assert.EqualValues(t, -900000, created["net_cents"])
}

// A form reports everything at once rather than one problem per submit.
func TestSales_RefuseAnIncompleteSaleWithEveryReason(t *testing.T) {
	cleanDB(t)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	w := doRequest("POST", "/v1/sales", map[string]any{}, token)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "informe o cliente")
	assert.Contains(t, body, "informe o valor da venda")
}

// Without this a sale could be filed against another account's contact and
// appear in their portfolio.
func TestSales_RefuseAContactFromAnotherAccount(t *testing.T) {
	cleanDB(t)
	stranger := seedSaleContact(t, seedIdentityAccount(t))
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	w := doRequest("POST", "/v1/sales", distributionSaleBody(stranger), token)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "CONTACT_NOT_FOUND")
}

// The portfolio view: everything one customer bought.
func TestSales_PortfolioIsPerContact(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	first := seedSaleContact(t, accountID)
	second := seedSaleContact(t, accountID)

	require.Equal(t, http.StatusOK,
		doRequest("POST", "/v1/sales", distributionSaleBody(first), token).Code)
	require.Equal(t, http.StatusOK,
		doRequest("POST", "/v1/sales", distributionSaleBody(second), token).Code)

	list := parseJSONArray(t,
		doRequest("GET", "/v1/contacts/"+first+"/sales", nil, token))

	require.Len(t, list, 1)
	assert.Equal(t, first, list[0].(map[string]any)["contact_id"])
}

// Deleting a sale moves money in a commission statement.
func TestSales_OnlyAnAdminDeletes(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	admin := tokenForSeededAccount(t, accountID, "admin")
	attendant := tokenForSeededAccount(t, accountID, "attendant")
	contactID := seedSaleContact(t, accountID)
	saleID := parseJSON(t, doRequest("POST", "/v1/sales",
		distributionSaleBody(contactID), admin))["id"].(string)

	assert.Equal(t, http.StatusForbidden,
		doRequest("DELETE", "/v1/sales/"+saleID, nil, attendant).Code)
	assert.Equal(t, http.StatusNoContent,
		doRequest("DELETE", "/v1/sales/"+saleID, nil, admin).Code)
}

// Recording what they sold is the salesperson's own job; gating it behind an
// admin is how a portfolio ends up empty.
func TestSales_AttendantsCanRecord(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	attendant := tokenForSeededAccount(t, accountID, "attendant")
	contactID := seedSaleContact(t, accountID)

	w := doRequest("POST", "/v1/sales", distributionSaleBody(contactID), attendant)

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// One account's sales are not another's.
func TestSales_AreScopedToTheAccount(t *testing.T) {
	cleanDB(t)
	owner := seedIdentityAccount(t)
	ownerToken := tokenForSeededAccount(t, owner, "admin")
	require.Equal(t, http.StatusOK, doRequest("POST", "/v1/sales",
		distributionSaleBody(seedSaleContact(t, owner)), ownerToken).Code)

	other := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/sales", nil, other)))
}

// A model that files sales on its own turns a misread message into a
// commission somebody is owed. The suite wires no AI provider, so the honest
// answer is a refusal — never an invented sale.
func TestSales_ExtractRefusesWithNoProvider(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	w := doRequest("POST", "/v1/conversations/"+ulid.Make().String()+"/extract-sale",
		nil, token)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "SALE_EXTRACT_UNAVAILABLE")
}

// A sale filed from a lead's card names the LEAD, not the customer: the
// server resolves the contact from it and closes the deal on the board. The
// app used to send the lead's own id as contact_id and get CONTACT_NOT_FOUND.
func TestSales_FromALeadResolvesTheCustomerAndWinsTheLead(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	contactID := seedSaleContact(t, accountID)
	funnelID := seedFunnelOn(t, accountID)
	leadID := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO leads (id, account_id, funnel_id, contact_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'qualified', NOW(), NOW())`,
		leadID, accountID, funnelID, contactID)
	require.NoError(t, err)

	body := distributionSaleBody("")
	body["lead_id"] = leadID
	body["sold_at"] = "10/03/2026"
	w := doRequest("POST", "/v1/sales", body, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	created := parseJSON(t, w)
	assert.Equal(t, contactID, created["contact_id"], "resolved from the lead")
	assert.Equal(t, leadID, created["lead_id"])
	assert.Contains(t, created["sold_at"], "2026-03-10", "a bare date is accepted")

	var status string
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT status FROM leads WHERE id = $1`, leadID).Scan(&status))
	assert.Equal(t, "won", status)
}

// Another account's lead is indistinguishable from a missing one.
func TestSales_RefuseALeadFromAnotherAccount(t *testing.T) {
	cleanDB(t)
	stranger := seedIdentityAccount(t)
	leadID := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO leads (id, account_id, funnel_id, contact_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'new', NOW(), NOW())`,
		leadID, stranger, seedFunnelOn(t, stranger), seedSaleContact(t, stranger))
	require.NoError(t, err)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	body := distributionSaleBody("")
	body["lead_id"] = leadID
	w := doRequest("POST", "/v1/sales", body, token)

	// 404, not 403: the answer that leaks nothing.
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "LEAD_NOT_FOUND")
}
