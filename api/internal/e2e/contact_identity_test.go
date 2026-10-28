package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
)

// One customer, one key — over the real doors.
//
// The same hospital used to become several contact rows because the CSV
// import, the manual lead form, the Places search and the CNES radar each
// keyed contacts.external_id their own way. These tests walk two doors and
// count rows.

func defaultFunnelID(t *testing.T, token string) string {
	t.Helper()
	rec := doRequest("GET", "/v1/funnels", nil, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, raw := range parseJSONArray(t, rec) {
		f := raw.(map[string]interface{})
		if f["is_default"] == true {
			return f["id"].(string)
		}
	}
	t.Fatal("the account has no default funnel")
	return ""
}

// She imports her base with "+55 65 3618-8000"; later somebody types the same
// hospital into the lead form as "+556536188000". One contact. And the hunt,
// which used to flag only park-offered establishments, knows the hospital.
func TestContactIdentity_ImportThenManualLeadIsOneContact(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	seedCNESPark(t) // Hospital São Mateus, CNES 2655411, phone "65 3618-8000"
	registerUser(t, "Cuiabamed", "dona@cuiabamed.test", "pass1234")
	verifyEmail(t, "dona@cuiabamed.test")
	token := login(t, "dona@cuiabamed.test", "pass1234")
	w := doRequest("PATCH", "/v1/accounts", map[string]interface{}{
		"territory": map[string]interface{}{"uf": "mt", "cities": []string{"Cuiabá"}},
	}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var accountID string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT account_id FROM users WHERE email = 'dona@cuiabamed.test'`).Scan(&accountID))

	// Door 1: the spreadsheet, with a person AND a company on the row.
	rec := uploadCustomerList(t,
		"Empresa;Contato;Telefone\nHospital São Mateus;Dra. Ana;+55 65 3618-8000\n", token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var preview customerPreview
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preview))
	confirm := doRequest("POST", "/v1/contacts/import", map[string]any{"rows": preview.Rows}, token)
	require.Equal(t, http.StatusOK, confirm.Code, confirm.Body.String())

	var externalID, name, company string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT external_id, COALESCE(name,''), COALESCE(company_name,'')
		   FROM contacts WHERE account_id = $1`, accountID).Scan(&externalID, &name, &company))
	assert.Equal(t, "556536188000", externalID, "the one key: phone digits, no plus")
	assert.Equal(t, "Dra. Ana", name)
	assert.Equal(t, "Hospital São Mateus", company, "the company column was dropped because the row had a person")

	// Door 2: the manual lead form, same number spelled the other way.
	funnelID := defaultFunnelID(t, token)
	lead := doRequest("POST", "/v1/funnels/"+funnelID+"/leads", map[string]interface{}{
		"company": "Hospital São Mateus", "name": "Dra. Ana", "phone": "+556536188000",
	}, token)
	require.Less(t, lead.Code, 300, lead.Body.String())

	var contacts, identities int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM contacts WHERE account_id = $1`, accountID).Scan(&contacts))
	assert.Equal(t, 1, contacts, "the manual lead minted a second contact for the same number")
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM contact_identities WHERE account_id = $1 AND kind = 'phone'`,
		accountID).Scan(&identities))
	assert.Equal(t, 1, identities)

	var leadContact string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT l.contact_id FROM leads l WHERE l.account_id = $1`, accountID).Scan(&leadContact))
	var importedID string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT id FROM contacts WHERE account_id = $1`, accountID).Scan(&importedID))
	assert.Equal(t, importedID, leadContact, "the lead points at a contact other than the imported one")

	// Door 3, read-only: the hunt sees the hospital as already known — by its
	// phone, since nobody park-offered it.
	w = doRequest("POST", "/v1/me/territory/hunt",
		map[string]interface{}{"equipment": "tomógrafo"}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	holders := parseJSON(t, w)["holders"].([]interface{})
	require.Len(t, holders, 1)
	assert.Equal(t, true, holders[0].(map[string]interface{})["already_known"],
		"a hospital the base has by phone was offered as cold ground")
}

// The real index: an address that already belongs to a contact is never
// re-pointed by a later write, and the row that stands is what comes back.
func TestContactIdentity_ALaterDoorNeverStealsAnAddress(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	contacts := postgres.NewContactRepository(testPool)
	identities := postgres.NewContactIdentityRepository(testPool)

	older, err := contacts.Upsert(ctx, &domain.Contact{
		ID: ulid.Make().String(), AccountID: accountID, ExternalID: "556536188000"})
	require.NoError(t, err)
	newer, err := contacts.Upsert(ctx, &domain.Contact{
		ID: ulid.Make().String(), AccountID: accountID, ExternalID: "cnes:2655411"})
	require.NoError(t, err)

	first, err := identities.Upsert(ctx, &domain.ContactIdentity{
		AccountID: accountID, ContactID: older.ID, Kind: domain.IdentityKindPhone, Value: "+556536188000"})
	require.NoError(t, err)
	assert.Equal(t, older.ID, first.ContactID)

	second, err := identities.Upsert(ctx, &domain.ContactIdentity{
		AccountID: accountID, ContactID: newer.ID, Kind: domain.IdentityKindPhone, Value: "+556536188000"})
	require.NoError(t, err)
	assert.Equal(t, older.ID, second.ContactID, "the returned row must be the one that stands")

	owner, err := identities.ContactIDByValue(ctx, accountID, domain.IdentityKindPhone, "+556536188000")
	require.NoError(t, err)
	assert.Equal(t, older.ID, owner, "the identity was stolen by the newer contact")
}

// The contact row keeps what it knew: a second door reaching the same key
// fills blanks and never blanks a filled column.
func TestContactIdentity_UpsertCompletesNeverErases(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	contacts := postgres.NewContactRepository(testPool)

	phone, company := "+556536188000", "Hospital São Mateus"
	_, err := contacts.Upsert(ctx, &domain.Contact{
		ID: ulid.Make().String(), AccountID: accountID, ExternalID: "556536188000",
		Phone: &phone, CompanyName: &company})
	require.NoError(t, err)

	email := "compras@saomateus.br"
	again, err := contacts.Upsert(ctx, &domain.Contact{
		ID: ulid.Make().String(), AccountID: accountID, ExternalID: "556536188000", Email: &email})
	require.NoError(t, err)
	require.NotNil(t, again.Phone)
	assert.Equal(t, phone, *again.Phone)
	require.NotNil(t, again.CompanyName)
	assert.Equal(t, company, *again.CompanyName)
	require.NotNil(t, again.Email)
	assert.Equal(t, email, *again.Email)
}
