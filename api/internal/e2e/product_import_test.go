package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Importing a price list, over the real HTTP path. The use case is covered with
// fakes; what only a real request proves is the multipart upload and that the
// products land in the account that asked.

func uploadPriceList(t *testing.T, csv, token string) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "precos.csv")
	require.NoError(t, err)
	_, err = part.Write([]byte(csv))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest("POST", "/v1/products/import/preview", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)
	return rec
}

func TestProductImport_PreviewThenConfirmCreatesTheCatalogue(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	// Semicolon-delimited with comma decimals: what Excel writes in a pt-BR
	// locale, and the only shape that can express "1.234,56" at all.
	rec := uploadPriceList(t,
		"Nome;Descrição;Preço\nGaze estéril 10x10;Pacote com 500;49,90\nLuva cirúrgica;;1.234,56\n",
		token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var preview struct {
		Rows []struct {
			Line       int    `json:"line"`
			Name       string `json:"name"`
			PriceCents int64  `json:"price_cents"`
			Currency   string `json:"currency"`
			Problem    string `json:"problem"`
		} `json:"rows"`
		Importable int `json:"importable"`
		Problems   int `json:"problems"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preview))
	require.Len(t, preview.Rows, 2)
	assert.Equal(t, 2, preview.Importable)
	assert.Equal(t, 0, preview.Problems)
	assert.Equal(t, int64(4990), preview.Rows[0].PriceCents)
	assert.Equal(t, int64(123456), preview.Rows[1].PriceCents)

	// The preview created nothing.
	var before int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM products WHERE account_id = $1`, accountID).Scan(&before))
	assert.Equal(t, 0, before, "the preview must not create products")

	// Confirming the rows the person saw.
	confirm := doRequest("POST", "/v1/products/import",
		map[string]any{"rows": preview.Rows}, token)
	require.Equal(t, http.StatusOK, confirm.Code, confirm.Body.String())

	var result struct {
		Created int `json:"created"`
		Skipped int `json:"skipped"`
	}
	require.NoError(t, json.Unmarshal(confirm.Body.Bytes(), &result))
	assert.Equal(t, 2, result.Created)
	assert.Equal(t, 0, result.Skipped)

	var name string
	var cents int64
	var currency string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT name, price_cents, currency FROM products
		 WHERE account_id = $1 ORDER BY name`, accountID).Scan(&name, &cents, &currency))
	assert.Equal(t, "Gaze estéril 10x10", name)
	assert.Equal(t, int64(4990), cents)
	assert.Equal(t, "BRL", currency)
}

// Re-importing the same file must not double the catalogue.
func TestProductImport_SecondImportOfTheSameFileCreatesNothing(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	const csv = "Nome;Preço\nGaze estéril;49,90\n"

	rec := uploadPriceList(t, csv, token)
	require.Equal(t, http.StatusOK, rec.Code)
	var preview map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preview))

	first := doRequest("POST", "/v1/products/import",
		map[string]any{"rows": preview["rows"]}, token)
	require.Equal(t, http.StatusOK, first.Code)

	// Second pass: the preview now says the row is already there, and the import
	// creates nothing even if the rows are sent again.
	rec2 := uploadPriceList(t, csv, token)
	require.Equal(t, http.StatusOK, rec2.Code)
	var preview2 struct {
		AlreadyInCatalogue int              `json:"already_in_catalogue"`
		Importable         int              `json:"importable"`
		Rows               []map[string]any `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &preview2))
	assert.Equal(t, 1, preview2.AlreadyInCatalogue)
	assert.Equal(t, 0, preview2.Importable)

	second := doRequest("POST", "/v1/products/import",
		map[string]any{"rows": preview2.Rows}, token)
	require.Equal(t, http.StatusOK, second.Code)
	var result struct{ Created, Skipped int }
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &result))
	assert.Equal(t, 0, result.Created)
	assert.Equal(t, 1, result.Skipped)

	var count int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM products WHERE account_id = $1`, accountID).Scan(&count))
	assert.Equal(t, 1, count, "re-importing doubled the catalogue")
}

// An ambiguous price is carried to the person rather than guessed. This is the
// row that, guessed wrong, makes a price list off by a factor of a thousand.
func TestProductImport_AmbiguousPriceIsReportedNotGuessed(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	rec := uploadPriceList(t, "Nome;Preço\nGaze;1,234\n", token)
	require.Equal(t, http.StatusOK, rec.Code)

	var preview struct {
		Rows []struct {
			PriceCents int64  `json:"price_cents"`
			Problem    string `json:"problem"`
		} `json:"rows"`
		Importable int `json:"importable"`
		Problems   int `json:"problems"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preview))
	require.Len(t, preview.Rows, 1)
	assert.Equal(t, 0, preview.Importable)
	assert.Equal(t, 1, preview.Problems)
	assert.NotEmpty(t, preview.Rows[0].Problem)
	assert.Equal(t, int64(0), preview.Rows[0].PriceCents)

	_ = accountID
}

// A file whose columns we cannot recognise is the WRONG file, not an empty one.
func TestProductImport_UnrecognisableHeaderSaysSo(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	rec := uploadPriceList(t, "coluna a;coluna b\nx;y\n", token)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "PRODUCT_IMPORT_NO_HEADER")

	_ = accountID
}

// A product imported from a brochure must keep the brochure.
//
// It did not. `products.source_id` was written from the row the CLIENT posts
// back, and the Flutter import model parsed every field of the preview except
// that one — so the page number survived and the file id did not. Production:
// nineteen archived brochures, a hundred and fifty-nine extracted images, and
// ZERO of a hundred and three products carrying a source. The "open the
// brochure" button was built, routed and wired, and could never render for
// anybody, because the data it reads was never stored.
//
// Driven over HTTP because the failure lived in the round trip: every layer
// was individually correct.
func TestProductImport_ABrochureIDSurvivesTheRoundTrip(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	sourceID := seedProductSource(t, accountID, "Capuz de Oxigênio.pdf")

	confirm := doRequest("POST", "/v1/products/import", map[string]any{
		"rows": []map[string]any{{
			"name":        "Capuz de Oxigênio - Pequeno",
			"price_cents": 0,
			"currency":    "BRL",
			"source_id":   sourceID,
			"page":        1,
		}},
	}, token)
	require.Equal(t, http.StatusOK, confirm.Code, confirm.Body.String())

	var storedSource string
	var storedPage int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COALESCE(source_id, ''), COALESCE(source_page, 0)
		   FROM products WHERE account_id = $1`, accountID).Scan(&storedSource, &storedPage))

	assert.Equal(t, sourceID, storedSource, "the product must remember its brochure")
	assert.Equal(t, 1, storedPage)

	// The door itself — GET /products/:id/source — is covered by
	// ProductSourceLink's own tests, which can fake a GCS signer. This test
	// exists for the half that was broken and that only a real round trip can
	// show: what the client posts back is what gets stored.
}

// The brochure id arrives from a browser, so it is a foreign key from outside.
// One account must not be able to point its product at another's document.
func TestProductImport_ABrochureFromAnotherAccountIsIgnored(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	mine := seedIdentityAccount(t)
	theirs := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, mine, "admin")

	foreign := seedProductSource(t, theirs, "Documento de outra empresa.pdf")

	confirm := doRequest("POST", "/v1/products/import", map[string]any{
		"rows": []map[string]any{{
			"name":      "Produto qualquer",
			"currency":  "BRL",
			"source_id": foreign,
			"page":      2,
		}},
	}, token)
	require.Equal(t, http.StatusOK, confirm.Code, confirm.Body.String())

	var storedSource string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COALESCE(source_id, '') FROM products WHERE account_id = $1`, mine).Scan(&storedSource))

	// The product is created — the person asked for it — it just arrives
	// without a document rather than with somebody else's.
	assert.Empty(t, storedSource)
}

func seedProductSource(t *testing.T, accountID, filename string) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(), `
		INSERT INTO product_sources (id, account_id, filename, object_path, content_hash, pages, byte_size, created_at)
		VALUES ($1, $2, $3, $4, $5, 2, 1024, NOW())`,
		id, accountID, filename, "accounts/"+accountID+"/brochures/"+id+".pdf", id)
	require.NoError(t, err)
	return id
}
