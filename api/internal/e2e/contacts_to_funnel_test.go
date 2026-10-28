package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// POST /funnels/:id/leads/from-contacts over HTTP.
//
// The route matters as much as the use case: gin runs a GROUP's middleware
// first, so a route mounted in the wrong group answers 403 no matter what the
// use case allows. That is exactly how the lead reads were unusable for a whole
// role while the code behind them was correct.

// seedPlainContact writes a contact in the shape the CSV import produces: an
// opaque ULID id, and an e-mail with no phone at all. The manual lead form
// refuses this row — it demands a typed phone — which is the reason this
// endpoint exists.
func seedPlainContact(t *testing.T, accountID, email string) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO contacts (id, account_id, external_id, email, channel_type,
		                       created_at, updated_at, first_seen_at, last_seen_at)
		 VALUES ($1, $2, $3, $3, 'email', NOW(), NOW(), NOW(), NOW())`,
		id, accountID, email)
	require.NoError(t, err)
	return id
}

type contactsToFunnelBody struct {
	Created  int      `json:"created"`
	Skipped  int      `json:"skipped"`
	Failed   int      `json:"failed"`
	LeadIDs  []string `json:"lead_ids"`
	Problems []struct {
		ContactID string `json:"contact_id"`
		Reason    string `json:"reason"`
	} `json:"problems"`
}

func TestContactsToFunnel_APhoneLessCustomerReachesTheFunnel(t *testing.T) {
	f := newRBACFixture(t)
	admin := seedRBACUser(t, f.accountID, "admin")
	token := tokenForUser(t, f.accountID, admin, "admin")

	// Two customers imported from a spreadsheet: e-mail only, no phone.
	a := seedPlainContact(t, f.accountID, "compras@hospital.com.br")
	b := seedPlainContact(t, f.accountID, "diretoria@clinica.com.br")

	rec := doRequest("POST", "/v1/funnels/"+f.funnelID+"/leads/from-contacts",
		map[string]any{"contact_ids": []string{a, b}}, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out contactsToFunnelBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 2, out.Created)
	assert.Zero(t, out.Failed)

	// The leads point at the contact ULID, which is what the timeline and the
	// approval queue resolve the recipient by.
	var count int
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM leads
		  WHERE funnel_id = $1 AND contact_id = ANY($2) AND source = 'existing_customer'`,
		f.funnelID, []string{a, b}).Scan(&count))
	assert.Equal(t, 2, count)
}

// Running it again is the normal case — she re-selects a page, or a colleague
// already imported half the base. The second run must not duplicate, and must
// not reset a pipeline that has moved on.
func TestContactsToFunnel_RunningItTwiceChangesNothing(t *testing.T) {
	f := newRBACFixture(t)
	admin := seedRBACUser(t, f.accountID, "admin")
	token := tokenForUser(t, f.accountID, admin, "admin")
	c := seedPlainContact(t, f.accountID, "compras@hospital.com.br")

	body := map[string]any{"contact_ids": []string{c}}
	first := doRequest("POST", "/v1/funnels/"+f.funnelID+"/leads/from-contacts", body, token)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())

	// Move the lead, so a "reset" would be visible.
	_, err := testPool.Exec(context.Background(),
		`UPDATE leads SET status = 'responded' WHERE funnel_id = $1 AND contact_id = $2`,
		f.funnelID, c)
	require.NoError(t, err)

	second := doRequest("POST", "/v1/funnels/"+f.funnelID+"/leads/from-contacts", body, token)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())

	var out contactsToFunnelBody
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &out))
	assert.Zero(t, out.Created)
	assert.Equal(t, 1, out.Skipped)

	var status string
	var rows int
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT count(*), max(status) FROM leads WHERE funnel_id = $1 AND contact_id = $2`,
		f.funnelID, c).Scan(&rows, &status))
	assert.Equal(t, 1, rows, "a second import must not create a second lead")
	assert.Equal(t, "responded", status,
		"the pipeline stage it had reached must survive a re-import")
}

// A contact id belonging to another tenant answers exactly like one that never
// existed. Anything else turns this into a probe for other accounts' ids.
func TestContactsToFunnel_AForeignContactIsJustNotFound(t *testing.T) {
	f := newRBACFixture(t)
	admin := seedRBACUser(t, f.accountID, "admin")
	token := tokenForUser(t, f.accountID, admin, "admin")

	other := seedIdentityAccount(t)
	foreign := seedPlainContact(t, other, "alvo@outraconta.com.br")

	rec := doRequest("POST", "/v1/funnels/"+f.funnelID+"/leads/from-contacts",
		map[string]any{"contact_ids": []string{foreign}}, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out contactsToFunnelBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Zero(t, out.Created)
	assert.Equal(t, 1, out.Failed)
	require.Len(t, out.Problems, 1)
	assert.Equal(t, "contact_not_found", out.Problems[0].Reason)

	// And nothing was written into the other account's data either.
	var leads int
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM leads WHERE contact_id = $1`, foreign).Scan(&leads))
	assert.Zero(t, leads)
}

// The destination is refused before the payload is judged.
func TestContactsToFunnel_AnotherAccountsFunnelIsRefused(t *testing.T) {
	f := newRBACFixture(t)
	admin := seedRBACUser(t, f.accountID, "admin")
	token := tokenForUser(t, f.accountID, admin, "admin")

	otherAccount := seedIdentityAccount(t)
	otherFunnel := seedFunnelOn(t, otherAccount)

	rec := doRequest("POST", "/v1/funnels/"+otherFunnel+"/leads/from-contacts",
		map[string]any{"contact_ids": []string{}}, token)

	assert.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}
