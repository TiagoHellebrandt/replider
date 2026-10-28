package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Contracts against a real database (§6.11, G4).
//
// The tests are about the three things that make this table different from every
// other one: what a role may READ, that every read is logged, and that a
// machine's reading is never presented as a fact until a person signs it.

func supplyContractBody() map[string]any {
	return map[string]any{
		"title":            "Fornecimento de luvas",
		"subject":          "Fornecimento mensal de EPI",
		"value_cents":      50000000,
		"payment_terms":    "30 dias após a nota",
		"starts_on":        "2026-04-01",
		"ends_on":          "2027-03-31",
		"auto_renewal":     true,
		"notice_days":      60,
		"adjustment_index": "IPCA",
		"adjustment_month": 3,
		"sla_text":         "entrega em 48h",
		"exclusivity":      true,
		"exclusivity_text": "exclusividade na região sul",
		"guarantees":       "fiança bancária",
		"termination":      "aviso prévio de 90 dias",
		"parties": []map[string]any{
			{"name": "Hospital São Lucas", "role": "contratante", "document": "12.345.678/0001-90"},
			{"name": "MedDistribui", "role": "contratada"},
		},
		"penalties": []map[string]any{
			{"description": "atraso na entrega", "percent": 10, "trigger": "atraso superior a 5 dias"},
		},
	}
}

func createContract(t *testing.T, token string, body map[string]any) map[string]any {
	t.Helper()
	w := doRequest("POST", "/v1/contracts", body, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return parseJSON(t, w)
}

func auditActions(t *testing.T, accountID, entityID string) []string {
	t.Helper()
	rows, err := testPool.Query(context.Background(),
		`SELECT action FROM audit_logs
		  WHERE account_id = $1 AND entity_type = 'contract' AND entity_id = $2
		  ORDER BY created_at`, accountID, entityID)
	require.NoError(t, err)
	defer rows.Close()

	var out []string
	for rows.Next() {
		var action string
		require.NoError(t, rows.Scan(&action))
		out = append(out, action)
	}
	require.NoError(t, rows.Err())
	return out
}

func TestContracts_FileConfirmAndDerivedDates(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	created := createContract(t, token, supplyContractBody())
	id, _ := created["id"].(string)
	require.NotEmpty(t, id)

	// No document and no AI wired: the reading is "skipped", not a pending job
	// that will never run.
	assert.Equal(t, "skipped", created["extraction_status"])
	assert.Equal(t, false, created["confirmed"])
	// The two dates every screen would otherwise compute differently.
	assert.Equal(t, "2027-01-30T00:00:00Z", created["notice_deadline"])
	assert.NotNil(t, created["days_until_end"])

	// A correction that also confirms: somebody who fixed the window and pressed
	// save has validated the reading.
	body := supplyContractBody()
	body["notice_days"] = 90
	body["confirm"] = true
	w := doRequest("PUT", "/v1/contracts/"+id, body, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	updated := parseJSON(t, w)
	assert.Equal(t, true, updated["confirmed"])
	assert.NotEmpty(t, updated["confirmed_by_user_id"])
	assert.Equal(t, "2026-12-31T00:00:00Z", updated["notice_deadline"])

	assert.Len(t, parseJSONArray(t, doRequest("GET", "/v1/contracts", nil, token)), 1)
}

func TestContracts_RefusesWhatCannotHaveBeenRead(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	cases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"no title", func(b map[string]any) { delete(b, "title") }},
		{"negative value", func(b map[string]any) { b["value_cents"] = -1 }},
		{"absurd notice window", func(b map[string]any) { b["notice_days"] = 5000 }},
		{"term ends before it starts", func(b map[string]any) {
			b["starts_on"], b["ends_on"] = "2027-04-01", "2026-04-01"
		}},
		{"penalty over 100%", func(b map[string]any) {
			b["penalties"] = []map[string]any{{"description": "multa", "percent": 400}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := supplyContractBody()
			tc.mutate(body)
			w := doRequest("POST", "/v1/contracts", body, token)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/contracts", nil, token)))
}

func TestContracts_AmendmentsChainToTheOriginal(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	parent := createContract(t, token, supplyContractBody())
	parentID, _ := parent["id"].(string)

	first := createContract(t, token, map[string]any{
		"title": "Aditivo 1 — reajuste", "parent_contract_id": parentID,
		"value_cents": 55000000,
	})
	assert.EqualValues(t, 2, first["version"])

	// An amendment OF an amendment lands on the same chain: "aditivo 3" is the
	// third change to the original, not a tree nobody can read.
	firstID, _ := first["id"].(string)
	second := createContract(t, token, map[string]any{
		"title": "Aditivo 2 — prazo", "parent_contract_id": firstID,
	})
	assert.EqualValues(t, 3, second["version"])
	assert.Equal(t, parentID, second["parent_contract_id"])

	// The list is the repository, not the history: amendments belong to their
	// parent's detail.
	list := parseJSONArray(t, doRequest("GET", "/v1/contracts", nil, token))
	require.Len(t, list, 1)

	detail := parseJSON(t, doRequest("GET", "/v1/contracts/"+parentID, nil, token))
	amendments, _ := detail["amendments"].([]any)
	require.Len(t, amendments, 2)

	// Asking for the amendment shows the same chain — otherwise "which version am
	// I looking at" depends on which link somebody clicked.
	fromChild := parseJSON(t, doRequest("GET", "/v1/contracts/"+firstID, nil, token))
	childChain, _ := fromChild["amendments"].([]any)
	assert.Len(t, childChain, 2)

	// Deleting the original takes its chain with it: an amendment to a contract
	// that no longer exists amends nothing.
	require.Equal(t, http.StatusNoContent,
		doRequest("DELETE", "/v1/contracts/"+parentID, nil, token).Code)
	assert.Equal(t, http.StatusNotFound,
		doRequest("GET", "/v1/contracts/"+firstID, nil, token).Code)
}

// The redaction is the authorization: it happens on the way out, in the domain,
// so a client cannot undo it.
func TestContracts_RedactedForTheFloorAndLoggedForEverybody(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	adminToken := tokenForSeededAccount(t, accountID, "admin")
	attendantToken := tokenForSeededAccount(t, accountID, "attendant")

	created := createContract(t, adminToken, supplyContractBody())
	id, _ := created["id"].(string)

	admin := parseJSON(t, doRequest("GET", "/v1/contracts/"+id, nil, adminToken))
	assert.EqualValues(t, 50000000, admin["value_cents"])
	assert.Nil(t, admin["redacted"])

	attendant := parseJSON(t, doRequest("GET", "/v1/contracts/"+id, nil, attendantToken))
	// The response ADMITS it hid something — a value of 0 with no flag would
	// read as "this contract is free".
	assert.Equal(t, true, attendant["redacted"])
	assert.EqualValues(t, 0, attendant["value_cents"])
	assert.Nil(t, attendant["penalties"])
	assert.Nil(t, attendant["payment_terms"])
	assert.Nil(t, attendant["guarantees"])
	assert.Nil(t, attendant["exclusivity_text"])
	// What serving the customer needs stays readable: the term and the SLA.
	assert.Equal(t, "entrega em 48h", attendant["sla_text"])
	assert.NotNil(t, attendant["ends_on"])
	// Exclusivity as a FACT stays, so nobody promises a competitor's region.
	assert.Equal(t, true, attendant["exclusivity"])

	// The same redaction on the list, not only on the detail.
	listed := parseJSONArray(t, doRequest("GET", "/v1/contracts", nil, attendantToken))
	require.Len(t, listed, 1)
	first, _ := listed[0].(map[string]any)
	assert.Equal(t, true, first["redacted"])
	assert.EqualValues(t, 0, first["value_cents"])

	// Writing, deleting and downloading are admin-only: a signed URL IS the
	// document, with every clause the redaction just removed.
	assert.Equal(t, http.StatusForbidden,
		doRequest("POST", "/v1/contracts", supplyContractBody(), attendantToken).Code)
	assert.Equal(t, http.StatusForbidden,
		doRequest("PUT", "/v1/contracts/"+id, supplyContractBody(), attendantToken).Code)
	assert.Equal(t, http.StatusForbidden,
		doRequest("GET", "/v1/contracts/"+id+"/download", nil, attendantToken).Code)
	assert.Equal(t, http.StatusForbidden,
		doRequest("DELETE", "/v1/contracts/"+id, nil, attendantToken).Code)

	// Every read is on the trail, with who and from where — "who opened this" is
	// the question an incident starts with.
	actions := auditActions(t, accountID, id)
	assert.Contains(t, actions, "contract.created")
	// Two detail reads (admin, then attendant) and nothing for the list: a
	// listing is not a document read, and logging one per row would drown the
	// trail that matters.
	assert.Equal(t, 2, countOf(actions, "contract.viewed"))

	var ip, agent *string
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT ip, user_agent FROM audit_logs
		  WHERE entity_id = $1 AND action = 'contract.viewed' LIMIT 1`, id).
		Scan(&ip, &agent))
	assert.NotNil(t, ip, "the access log records where the read came from")
}

func countOf(list []string, want string) int {
	n := 0
	for _, v := range list {
		if v == want {
			n++
		}
	}
	return n
}

func TestContracts_TenancyAndStorageGaps(t *testing.T) {
	cleanDB(t)
	accountA := seedIdentityAccount(t)
	accountB := seedIdentityAccount(t)
	tokenA := tokenForSeededAccount(t, accountA, "admin")
	tokenB := tokenForSeededAccount(t, accountB, "admin")

	created := createContract(t, tokenA, supplyContractBody())
	id, _ := created["id"].(string)

	// Another company's contract is not readable, writable or deletable by
	// guessing an id.
	assert.Equal(t, http.StatusNotFound,
		doRequest("GET", "/v1/contracts/"+id, nil, tokenB).Code)
	assert.Equal(t, http.StatusNotFound,
		doRequest("PUT", "/v1/contracts/"+id, supplyContractBody(), tokenB).Code)
	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/contracts", nil, tokenB)))

	// An amendment cannot be attached across the tenancy boundary either.
	assert.Equal(t, http.StatusNotFound,
		doRequest("POST", "/v1/contracts", map[string]any{
			"title": "Aditivo alheio", "parent_contract_id": id,
		}, tokenB).Code)

	// No bucket in this environment: upload and download say the storage is
	// unavailable instead of pretending to have filed something.
	up := doRequest("POST", "/v1/contracts/upload-url", map[string]any{
		"filename": "contrato.pdf", "size": 1024,
	}, tokenA)
	assert.Equal(t, http.StatusServiceUnavailable, up.Code, up.Body.String())
	// Input is still validated BEFORE storage is probed, so a .docx gets the
	// same answer whether or not the bucket is wired.
	bad := doRequest("POST", "/v1/contracts/upload-url", map[string]any{
		"filename": "contrato.docx", "size": 1024,
	}, tokenA)
	assert.Equal(t, http.StatusBadRequest, bad.Code)
	assert.Equal(t, "INVALID_FILE_TYPE", parseJSON(t, bad)["code"])

	huge := doRequest("POST", "/v1/contracts/upload-url", map[string]any{
		"filename": "contrato.pdf", "size": 40 * 1024 * 1024,
	}, tokenA)
	assert.Equal(t, http.StatusBadRequest, huge.Code)
	assert.Equal(t, "CONTRACT_FILE_TOO_LARGE", parseJSON(t, huge)["code"])

	// A contract with no document cannot be downloaded or re-read, and says so
	// as a state rather than a validation error.
	down := doRequest("GET", "/v1/contracts/"+id+"/download", nil, tokenA)
	assert.Equal(t, http.StatusNotFound, down.Code)
	assert.Equal(t, "CONTRACT_NO_DOCUMENT", parseJSON(t, down)["code"])

	// Re-reading with no AI provider wired is a deployment gap (503), never
	// "your contract could not be read".
	re := doRequest("POST", "/v1/contracts/"+id+"/reextract", nil, tokenA)
	assert.Equal(t, http.StatusServiceUnavailable, re.Code, re.Body.String())
	assert.Equal(t, "CONTRACT_EXTRACT_UNAVAILABLE", parseJSON(t, re)["code"])
}
