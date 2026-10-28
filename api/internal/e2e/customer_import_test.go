package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Importing the customer list, over the real HTTP path.
//
// The use case is covered with fakes. What only a real request proves is the
// multipart upload, the role the route is mounted under, and — the one that
// matters most — that an imported customer and the WhatsApp message they send
// afterwards land on the SAME contact row.

func uploadCustomerList(t *testing.T, csv, token string) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "clientes.csv")
	require.NoError(t, err)
	_, err = part.Write([]byte(csv))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest("POST", "/v1/contacts/import/preview", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)
	return rec
}

type customerPreview struct {
	Rows []struct {
		Line    int    `json:"line"`
		Company string `json:"company"`
		Name    string `json:"name"`
		Phone   string `json:"phone"`
		Email   string `json:"email"`
		Problem string `json:"problem"`
	} `json:"rows"`
	Importable    int `json:"importable"`
	Problems      int `json:"problems"`
	AlreadyInBase int `json:"already_in_base"`
}

func TestCustomerImport_PreviewThenConfirmBuildsTheBase(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	// Semicolon-delimited with phones typed the way a person types them: what a
	// customer list exported from a pt-BR Excel actually looks like.
	rec := uploadCustomerList(t,
		"Empresa;Contato;Telefone;E-mail\n"+
			"Clínica Aurora;Marina;(51) 98888-7766;marina@aurora.com.br\n"+
			"Hospital Vale;;51988887755;\n",
		token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var preview customerPreview
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preview))
	require.Len(t, preview.Rows, 2)
	assert.Equal(t, 2, preview.Importable)
	assert.Equal(t, 0, preview.Problems)
	assert.Equal(t, "+5551988887766", preview.Rows[0].Phone)

	var before int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM contacts WHERE account_id = $1`, accountID).Scan(&before))
	assert.Equal(t, 0, before, "the preview must not create contacts")

	confirm := doRequest("POST", "/v1/contacts/import",
		map[string]any{"rows": preview.Rows}, token)
	require.Equal(t, http.StatusOK, confirm.Code, confirm.Body.String())

	var result struct {
		Created      int `json:"created"`
		LeadsCreated int `json:"leads_created"`
		Skipped      int `json:"skipped"`
	}
	require.NoError(t, json.Unmarshal(confirm.Body.Bytes(), &result))
	assert.Equal(t, 2, result.Created)
	// The customers she brought ARE her pipeline: each one is a lead in the
	// default funnel without a second gesture (2026-09-01).
	assert.Equal(t, 2, result.LeadsCreated)
	assert.Equal(t, 0, result.Skipped)

	var externalID, name, phone string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT external_id, COALESCE(name,''), COALESCE(phone,'')
		 FROM contacts WHERE account_id = $1 AND external_id = '5551988887766'`,
		accountID).Scan(&externalID, &name, &phone))
	assert.Equal(t, "Marina", name)
	assert.Equal(t, "+5551988887766", phone)

	// The organization is the name when there is no person at it — a contact
	// called "" cannot be recognised in the screen that approves a message.
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COALESCE(name,'') FROM contacts
		 WHERE account_id = $1 AND external_id = '5551988887755'`, accountID).Scan(&name))
	assert.Equal(t, "Hospital Vale", name)

	// And every address is indexed, which is what makes the next message from
	// this customer resolve to this contact instead of creating a stranger.
	var identities int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM contact_identities WHERE account_id = $1`,
		accountID).Scan(&identities))
	assert.Equal(t, 3, identities, "two phones and one email")
}

// The whole point of the feature: she imports her customers, and when one of
// them writes on WhatsApp it is the same row — not a second stranger with the
// same phone number.
func TestCustomerImport_TheCustomerWhoWritesLaterIsTheSameContact(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	rec := uploadCustomerList(t,
		"Empresa,Telefone\nClínica Aurora,(51) 98888-7766\n", token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var preview customerPreview
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preview))

	confirm := doRequest("POST", "/v1/contacts/import",
		map[string]any{"rows": preview.Rows}, token)
	require.Equal(t, http.StatusOK, confirm.Code, confirm.Body.String())

	var importedID string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT id FROM contacts WHERE account_id = $1`, accountID).Scan(&importedID))

	// Now WhatsApp: Meta writes the sender as bare digits, and that is the key
	// an inbound message arrives on. If the import had stored "+5551…" this
	// lookup would miss and the customer would exist twice.
	var found string
	err := testPool.QueryRow(ctx,
		`SELECT id FROM contacts WHERE account_id = $1 AND external_id = $2`,
		accountID, "5551988887766").Scan(&found)
	require.NoError(t, err, "a WhatsApp inbound would not have found the imported customer")
	assert.Equal(t, importedID, found)

	// The identity index answers the same question from any channel.
	var byIdentity string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT contact_id FROM contact_identities
		 WHERE account_id = $1 AND kind = 'phone' AND value = $2`,
		accountID, "+5551988887766").Scan(&byIdentity))
	assert.Equal(t, importedID, byIdentity)
}

// A customer the account already has from WhatsApp has no identity row — they
// predate the index. Asking only the index would import them a second time.
func TestCustomerImport_AWhatsAppContactIsRecognisedWithNoIdentityRow(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	existing := ulid.Make().String()
	_, err := testPool.Exec(ctx,
		`INSERT INTO contacts (id, account_id, external_id, name, first_seen_at, last_seen_at, created_at, updated_at)
		 VALUES ($1, $2, '5551988887766', 'Aurora (WhatsApp)', NOW(), NOW(), NOW(), NOW())`,
		existing, accountID)
	require.NoError(t, err)

	rec := uploadCustomerList(t,
		"Empresa,Telefone\nClínica Aurora,(51) 98888-7766\n", token)
	require.Equal(t, http.StatusOK, rec.Code)

	var preview customerPreview
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preview))
	assert.Equal(t, 1, preview.AlreadyInBase)
	assert.Equal(t, 0, preview.Importable)

	confirm := doRequest("POST", "/v1/contacts/import",
		map[string]any{"rows": preview.Rows}, token)
	require.Equal(t, http.StatusOK, confirm.Code)

	var count int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM contacts WHERE account_id = $1`, accountID).Scan(&count))
	assert.Equal(t, 1, count, "the customer was imported a second time")

	// And the name the channel learned outranks the one in the spreadsheet.
	var name string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT name FROM contacts WHERE id = $1`, existing).Scan(&name))
	assert.Equal(t, "Aurora (WhatsApp)", name)
}

// A file whose columns give no way to reach anybody is the WRONG file, not an
// empty one.
func TestCustomerImport_UnrecognisableHeaderSaysSo(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	rec := uploadCustomerList(t, "empresa;cidade\nClínica Aurora;Porto Alegre\n", token)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "CUSTOMER_IMPORT_NO_HEADER")
}

// The import writes the account's customer base in bulk. That is an owner's
// decision, the same one the Sent-folder import makes.
func TestCustomerImport_AnAttendantCannotImportTheCustomerBase(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "attendant")

	rec := uploadCustomerList(t, "empresa,telefone\nClínica Aurora,51988887766\n", token)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	confirm := doRequest("POST", "/v1/contacts/import",
		map[string]any{"rows": []map[string]any{{"company": "X", "phone": "51988887766"}}}, token)
	assert.Equal(t, http.StatusForbidden, confirm.Code, confirm.Body.String())
}

// Two thousand rows against a real Postgres.
//
// The dedupe used to ask per row: up to three lookups × 2000, once in the
// preview and again on confirm. This runs the whole file through the bulk index
// (three queries, `= ANY($n)`) — raw SQL that no fake can check — and asserts
// the answers with a base that already contains some of the rows, both the way
// the identity index knows them and the way WhatsApp keys them.
// bigImportKey is the channel key (digits, no plus) of row n in the generated
// file. A real Brazilian mobile shape — 55 + area 65 + a 9-digit subscriber
// number — because NormalizeTypedPhone refuses anything shorter and the rows
// would silently fall back to being keyed by e-mail instead.
func bigImportKey(n int) string { return fmt.Sprintf("55659%08d", n) }

func TestCustomerImport_TwoThousandRowsWithDuplicatesAndAKnownBase(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	// Already in the base, known by IDENTITY.
	knownByIdentity := ulid.Make().String()
	_, err := testPool.Exec(ctx,
		`INSERT INTO contacts (id, account_id, external_id, created_at, updated_at, first_seen_at, last_seen_at)
		 VALUES ($1, $2, $3, NOW(), NOW(), NOW(), NOW())`,
		knownByIdentity, accountID, bigImportKey(7))
	require.NoError(t, err)
	_, err = testPool.Exec(ctx,
		`INSERT INTO contact_identities (id, account_id, contact_id, kind, value, created_at, updated_at)
		 VALUES ($1, $2, $3, 'phone', $4, NOW(), NOW())`,
		ulid.Make().String(), accountID, knownByIdentity, "+"+bigImportKey(7))
	require.NoError(t, err)

	// Already in the base the way WhatsApp keys it: bare digits as external_id
	// and NO identity row. Without the external-id fallback the import would
	// report this customer as new and create a second row for them.
	_, err = testPool.Exec(ctx,
		`INSERT INTO contacts (id, account_id, external_id, created_at, updated_at, first_seen_at, last_seen_at)
		 VALUES ($1, $2, $3, NOW(), NOW(), NOW(), NOW())`,
		ulid.Make().String(), accountID, bigImportKey(13))
	require.NoError(t, err)

	// 2000 rows; every 100th repeats the row before it, which a hand-maintained
	// spreadsheet always has.
	var csv bytes.Buffer
	csv.WriteString("empresa;contato;telefone;email\n")
	distinct := map[string]bool{}
	for i := 0; i < 2000; i++ {
		n := i
		if i%100 == 99 {
			n = i - 1
		}
		fmt.Fprintf(&csv, "Hospital %d;Compras;+%s;compras%d@h.com.br\n", n, bigImportKey(n), n)
		distinct[bigImportKey(n)] = true
	}

	rec := uploadCustomerList(t, csv.String(), token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var preview customerPreview
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preview))
	require.Len(t, preview.Rows, 2000)
	// The two customers the base already has, recognised through BOTH lookups.
	assert.Equal(t, 2, preview.AlreadyInBase,
		"one known by identity, one known only by the WhatsApp external id")

	confirm := doRequest("POST", "/v1/contacts/import",
		map[string]any{"rows": preview.Rows}, token)
	require.Equal(t, http.StatusOK, confirm.Code, confirm.Body.String())

	var result struct {
		Created int `json:"created"`
		Skipped int `json:"skipped"`
	}
	require.NoError(t, json.Unmarshal(confirm.Body.Bytes(), &result))
	assert.Equal(t, 2000, result.Created+result.Skipped,
		"every confirmed row is accounted for")

	// The base ends up with one row per distinct customer plus the two it
	// already had — no doubles from the in-file repeats, none from the rows it
	// already knew.
	var total int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM contacts WHERE account_id = $1`, accountID).Scan(&total))
	assert.Equal(t, len(distinct), total)

	// Running the same file again must change nothing at all.
	second := doRequest("POST", "/v1/contacts/import",
		map[string]any{"rows": preview.Rows}, token)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	var again int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM contacts WHERE account_id = $1`, accountID).Scan(&again))
	assert.Equal(t, total, again, "a re-import must not grow the base")
}
