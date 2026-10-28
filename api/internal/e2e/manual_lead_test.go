package e2e

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A funnel could only be filled by the Google Places import or by an
// inbound message, so a referral or a card picked up at a conference had no
// route into the pipeline except the MCP tool. These pin the manual door.

func createFunnel(t *testing.T, token, name string) string {
	t.Helper()
	w := doRequest("POST", "/v1/funnels", map[string]interface{}{"name": name}, token)
	require.Equal(t, 201, w.Code, "create funnel failed: %s", w.Body.String())
	var funnel map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &funnel))
	id, _ := funnel["id"].(string)
	require.NotEmpty(t, id)
	return id
}

func TestE2E_ManualLeadEntersTheFunnelAndIsIdempotent(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Clinic Co", "sdr@test.com", "pass1234")
	verifyEmail(t, "sdr@test.com")
	token := login(t, "sdr@test.com", "pass1234")
	funnelID := createFunnel(t, token, "Clínicas SP")

	w := doRequest("POST", "/v1/funnels/"+funnelID+"/leads", map[string]interface{}{
		"name":  "Clínica Aurora",
		"phone": "+5511999900101",
	}, token)
	require.Equal(t, 201, w.Code, "create lead failed: %s", w.Body.String())
	body := parseJSON(t, w)
	assert.Equal(t, true, body["created"])
	lead := body["lead"].(map[string]interface{})
	leadID := lead["id"].(string)
	assert.Equal(t, "new", lead["status"], "a manual lead starts at the top of the pipeline")
	assert.Equal(t, "manual", lead["source"])

	// Re-submitting the same phone must read as "already in the pipeline",
	// not create a second card for the same clinic.
	w = doRequest("POST", "/v1/funnels/"+funnelID+"/leads", map[string]interface{}{
		"name":  "Clínica Aurora (dup)",
		"phone": "+5511999900101",
	}, token)
	require.Equal(t, 200, w.Code, "second create failed: %s", w.Body.String())
	body = parseJSON(t, w)
	assert.Equal(t, false, body["created"], "200 means matched, 201 means created")
	assert.Equal(t, leadID, body["lead"].(map[string]interface{})["id"])

	// One card on the board, not two.
	w = doRequest("GET", "/v1/funnels/"+funnelID+"/leads", nil, token)
	require.Equal(t, 200, w.Code)
	assert.Len(t, parseJSONArray(t, w), 1)
}

func TestE2E_ManualLeadRejectsAnUnusablePhone(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Clinic Co", "sdr@test.com", "pass1234")
	verifyEmail(t, "sdr@test.com")
	token := login(t, "sdr@test.com", "pass1234")
	funnelID := createFunnel(t, token, "Clínicas SP")

	// The phone doubles as the messaging address, so a lead we could never
	// contact must not enter the pipeline looking actionable.
	w := doRequest("POST", "/v1/funnels/"+funnelID+"/leads", map[string]interface{}{
		"name":  "Sem telefone",
		"phone": "não é um telefone",
	}, token)
	assert.GreaterOrEqual(t, w.Code, 400, "body: %s", w.Body.String())
	assert.Less(t, w.Code, 500)
}

// The route lives under the funnel, so the funnel is the ownership check.
func TestE2E_ManualLeadCannotTargetAnotherAccountsFunnel(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Clinic Co", "owner@test.com", "pass1234")
	verifyEmail(t, "owner@test.com")
	ownerToken := login(t, "owner@test.com", "pass1234")
	funnelID := createFunnel(t, ownerToken, "Clínicas SP")

	registerUser(t, "Other Co", "stranger@test.com", "pass1234")
	verifyEmail(t, "stranger@test.com")
	strangerToken := login(t, "stranger@test.com", "pass1234")

	w := doRequest("POST", "/v1/funnels/"+funnelID+"/leads", map[string]interface{}{
		"name":  "Intruso",
		"phone": "+5511999900199",
	}, strangerToken)
	assert.NotEqual(t, 201, w.Code, "a stranger seeded another account's funnel")
	assert.GreaterOrEqual(t, w.Code, 400, "body: %s", w.Body.String())

	w = doRequest("GET", "/v1/funnels/"+funnelID+"/leads", nil, ownerToken)
	require.Equal(t, 200, w.Code)
	assert.Empty(t, parseJSONArray(t, w))
}

// Re-registering a number returns the existing lead "untouched" — which has
// to include the contact. The contact's name is what the board shows AND
// what the send confirmation quotes, so a second entry under a different
// name silently renames a clinic inside an approval already waiting on a
// human.
func TestE2E_ManualLeadDoesNotRenameAKnownContact(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Clinic Co", "sdr@test.com", "pass1234")
	verifyEmail(t, "sdr@test.com")
	token := login(t, "sdr@test.com", "pass1234")
	funnelID := createFunnel(t, token, "Clínicas SP")

	w := doRequest("POST", "/v1/funnels/"+funnelID+"/leads", map[string]interface{}{
		"name":  "Clínica Aurora",
		"phone": "+5511999900101",
	}, token)
	require.Equal(t, 201, w.Code, "body: %s", w.Body.String())
	leadID := parseJSON(t, w)["lead"].(map[string]interface{})["id"].(string)

	w = doRequest("POST", "/v1/funnels/"+funnelID+"/leads", map[string]interface{}{
		"name":  "Typo Ltda",
		"phone": "+5511999900101",
	}, token)
	require.Equal(t, 200, w.Code, "body: %s", w.Body.String())

	w = doRequest("GET", "/v1/leads/"+leadID, nil, token)
	require.Equal(t, 200, w.Code)
	lead := parseJSON(t, w)
	assert.Equal(t, "Clínica Aurora", lead["contact_label"],
		"the second entry renamed a contact it said it left untouched")
}

// The approval step exists to catch a wrong recipient, and a name cannot do
// that: two clinics can share one, and a mistyped number keeps whatever
// name was typed with it. The payload has to carry the address too.
func TestE2E_LeadCarriesTheAddressNotJustTheName(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Clinic Co", "sdr@test.com", "pass1234")
	verifyEmail(t, "sdr@test.com")
	token := login(t, "sdr@test.com", "pass1234")
	funnelID := createFunnel(t, token, "Clínicas SP")

	w := doRequest("POST", "/v1/funnels/"+funnelID+"/leads", map[string]interface{}{
		"name":  "Clínica Aurora",
		"phone": "+5511999900101",
	}, token)
	require.Equal(t, 201, w.Code, "body: %s", w.Body.String())
	leadID := parseJSON(t, w)["lead"].(map[string]interface{})["id"].(string)

	w = doRequest("GET", "/v1/leads/"+leadID, nil, token)
	require.Equal(t, 200, w.Code)
	lead := parseJSON(t, w)
	assert.Equal(t, "Clínica Aurora", lead["contact_label"], "the board still shows a name")
	assert.Equal(t, "+5511999900101", lead["contact_address"],
		"the approval step needs the number the message goes to")
}

// A lead in this product is a company with somebody in it, and a hospital's
// purchasing manager reached by e-mail is as real a lead as a clinic owner
// on WhatsApp (2026-08-19). The company, role and city land in the lead's
// enrichment under the researcher's own keys, so the card shows the company
// and the draft can cite the person.
func TestE2E_ManualLeadCanBeACompanyReachedByEmail(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Cuiabamed", "dona@cuiabamed.test", "pass1234")
	verifyEmail(t, "dona@cuiabamed.test")
	token := login(t, "dona@cuiabamed.test", "pass1234")
	funnelID := createFunnel(t, token, "Hospitais MT")

	w := doRequest("POST", "/v1/funnels/"+funnelID+"/leads", map[string]interface{}{
		"company": "Hospital Regional de Cuiabá",
		"name":    "Maria Souza",
		"role":    "Gerente de compras",
		"email":   "Compras@HospitalRegional.test",
		"city":    "Cuiabá, MT",
	}, token)
	require.Equal(t, 201, w.Code, w.Body.String())
	lead := parseJSON(t, w)["lead"].(map[string]interface{})
	enrich := lead["enrichment"].(map[string]interface{})
	assert.Equal(t, "Hospital Regional de Cuiabá", enrich["company_name"])
	assert.Equal(t, "Maria Souza", enrich["decision_maker_name"])
	assert.Equal(t, "Gerente de compras", enrich["decision_maker_role"])
	assert.Equal(t, "Cuiabá, MT", enrich["address"])

	// The contact is keyed on the lower-cased e-mail and flagged as an
	// email contact — the next message from that address lands here.
	var externalID, channel string
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT external_id, channel_type FROM contacts WHERE id = $1`,
		lead["contact_id"]).Scan(&externalID, &channel))
	assert.Equal(t, "compras@hospitalregional.test", externalID)
	assert.Equal(t, "email", channel)

	// Same e-mail again = same lead, not a second card.
	w = doRequest("POST", "/v1/funnels/"+funnelID+"/leads", map[string]interface{}{
		"company": "Hospital Regional de Cuiabá", "email": "compras@hospitalregional.test",
	}, token)
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Equal(t, lead["id"], parseJSON(t, w)["lead"].(map[string]interface{})["id"])
}

func TestE2E_ManualLeadNeedsAWayToBeReached(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Cuiabamed", "dona@cuiabamed.test", "pass1234")
	verifyEmail(t, "dona@cuiabamed.test")
	token := login(t, "dona@cuiabamed.test", "pass1234")
	funnelID := createFunnel(t, token, "Hospitais MT")

	w := doRequest("POST", "/v1/funnels/"+funnelID+"/leads", map[string]interface{}{
		"company": "Hospital sem contato",
	}, token)
	assert.Equal(t, 400, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "LEAD_NEEDS_PHONE_OR_EMAIL")

	w = doRequest("POST", "/v1/funnels/"+funnelID+"/leads", map[string]interface{}{
		"company": "Hospital", "email": "isto não é e-mail",
	}, token)
	assert.Equal(t, 400, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "LEAD_INVALID_EMAIL")
}
