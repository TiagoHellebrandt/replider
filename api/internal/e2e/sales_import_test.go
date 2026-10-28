package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// The sales-history import against a real database.
//
// The use case is covered with fakes. What only a real request proves is the
// multipart upload, the admin gate, and — the one that matters — that an
// imported history makes the portfolio and the reorder engine SEE purchases
// per product, which is the whole reason the import exists.

// newE2ESaleHandler wires sales the way main.go does: the lead link and the
// history import both attached.
func newE2ESaleHandler(
	pool *pgxpool.Pool, contactRepo domain.ContactRepository,
	productRepo domain.ProductRepository, accountRepo domain.AccountRepository,
	leadRepo domain.LeadRepository,
) *handlers.SaleHandler {
	saleRepo := postgres.NewSaleRepository(pool)
	uc := usecases.NewSaleUseCase(saleRepo, contactRepo)
	uc.SetLeadRepo(leadRepo)
	h := handlers.NewSaleHandler(uc, saleRepo)
	importUC := usecases.NewImportSalesUseCase(saleRepo,
		postgres.NewSaleImportIndexRepository(pool), contactRepo, productRepo, accountRepo)
	importUC.SetIdentityRepo(postgres.NewContactIdentityRepository(pool))
	h.SetImport(handlers.NewSaleImportHandler(importUC))
	return h
}

// seedNamedProduct is seedProduct with a name and a consumption cycle.
func seedNamedProduct(t *testing.T, accountID, name string, cycleDays int) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO products (id, account_id, name, description, price_cents,
		    currency, attributes, active, consumption_cycle_days, created_at, updated_at)
		 VALUES ($1,$2,$3,'',5000,'brl','{}',true,$4,NOW(),NOW())`,
		id, accountID, name, cycleDays)
	require.NoError(t, err)
	return id
}

// seedEmailContact seeds a customer the way the customer import leaves one:
// keyed on the address, with the name a spreadsheet would carry.
func seedEmailContact(t *testing.T, accountID, email, name string) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO contacts (id, account_id, external_id, channel_type, name, email, created_at, updated_at)
		 VALUES ($1, $2, $3, 'email', $4, $3, NOW(), NOW())`,
		id, accountID, email, name)
	require.NoError(t, err)
	return id
}

type saleImportPreview struct {
	Rows []struct {
		Line      int    `json:"line"`
		Customer  string `json:"customer"`
		Product   string `json:"product"`
		ContactID string `json:"contact_id"`
		ProductID string `json:"product_id"`
		Problem   string `json:"problem"`
		Duplicate bool   `json:"duplicate"`
	} `json:"rows"`
	Importable       int `json:"importable"`
	Problems         int `json:"problems"`
	AlreadyRecorded  int `json:"already_recorded"`
	UnknownCustomers int `json:"unknown_customers"`
	raw              map[string]any
}

func previewSales(t *testing.T, csv, token string) saleImportPreview {
	t.Helper()
	w := doMultipartCSV(t, "/v1/sales/import/preview", csv, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out saleImportPreview
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out.raw))
	return out
}

func confirmSales(t *testing.T, preview saleImportPreview, token string) map[string]any {
	t.Helper()
	w := doRequest("POST", "/v1/sales/import", map[string]any{"rows": preview.raw["rows"]}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return parseJSON(t, w)
}

// Two products for one customer, both with a cycle, bought on dates that put
// both inside the reorder window: the portfolio must show ONE customer with
// TWO purchases, and the reorder list must carry BOTH products — the
// first-item-only reading used to see one.
func TestSalesImport_TwoProductsFeedThePortfolioPerProduct(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	contactID := seedEmailContact(t, accountID, "compras@aurora.com.br", "Clínica Aurora")
	gloves := seedNamedProduct(t, accountID, "Luvas de procedimento", 30)
	gauze := seedNamedProduct(t, accountID, "Gaze estéril", 60)

	// Dates in pt-BR notation, semicolon-delimited, amount with a comma: what
	// an ERP export looks like. Gloves 28 days ago (due in 2), gauze 58 days
	// ago (due in 2).
	glovesDay := time.Now().AddDate(0, 0, -28).Format("02/01/2006")
	gauzeDay := time.Now().AddDate(0, 0, -58).Format("02/01/2006")
	preview := previewSales(t,
		"Cliente;Produto;Data;Valor;Quantidade\n"+
			"compras@aurora.com.br;luvas de procedimento;"+glovesDay+";1.500,00;200\n"+
			"compras@aurora.com.br;Gaze estéril;"+gauzeDay+";320,50;40\n",
		token)

	require.Equal(t, 2, preview.Importable, "%+v", preview.Rows)
	assert.Equal(t, 0, preview.Problems)
	require.Len(t, preview.Rows, 2)
	assert.Equal(t, contactID, preview.Rows[0].ContactID)
	assert.Equal(t, gloves, preview.Rows[0].ProductID, "case-insensitive catalogue match")
	assert.Equal(t, gauze, preview.Rows[1].ProductID)

	result := confirmSales(t, preview, token)
	assert.EqualValues(t, 2, result["created"])

	// The rows landed as imported sales, in the account's currency, with the
	// product on the item.
	var count int
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM sales WHERE account_id = $1 AND contact_id = $2
		    AND source = 'imported' AND items -> 0 ->> 'product_id' IN ($3, $4)`,
		accountID, contactID, gloves, gauze).Scan(&count))
	assert.Equal(t, 2, count)

	// Portfolio: one customer, two purchases, R$ 1.820,50 lifetime.
	list := parseJSONArray(t, doRequest("GET", "/v1/portfolio", nil, token))
	require.Len(t, list, 1)
	row := list[0].(map[string]any)
	assert.Equal(t, contactID, row["contact_id"])
	assert.EqualValues(t, 2, row["purchases"])
	assert.EqualValues(t, 182050, row["gross_cents"])
	assert.NotNil(t, row["days_until_due"])

	// Reorders: one candidate PER PRODUCT, each naming its own item.
	reorders := parseJSONArray(t, doRequest("GET", "/v1/portfolio/reorders", nil, token))
	require.Len(t, reorders, 2, "one reorder per product, not per customer")
	products := map[string]bool{}
	for _, r := range reorders {
		items := r.(map[string]any)["items"].([]any)
		require.Len(t, items, 1)
		products[items[0].(map[string]any)["product_id"].(string)] = true
	}
	assert.True(t, products[gloves] && products[gauze], "%v", products)
}

// The same file twice must not double the history: the preview says so and
// the import records nothing even if the rows are sent again.
func TestSalesImport_SecondImportOfTheSameFileRecordsNothing(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	seedEmailContact(t, accountID, "compras@aurora.com.br", "Clínica Aurora")

	const csv = "Cliente,Produto,Data,Valor\ncompras@aurora.com.br,Luvas,2026-03-10,1500.00\n"

	first := confirmSales(t, previewSales(t, csv, token), token)
	assert.EqualValues(t, 1, first["created"])

	second := previewSales(t, csv, token)
	assert.Equal(t, 1, second.AlreadyRecorded)
	assert.Equal(t, 0, second.Importable)
	require.Len(t, second.Rows, 1)
	assert.True(t, second.Rows[0].Duplicate)

	result := confirmSales(t, second, token)
	assert.EqualValues(t, 0, result["created"])
	assert.EqualValues(t, 1, result["skipped"])

	var count int
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM sales WHERE account_id = $1`, accountID).Scan(&count))
	assert.Equal(t, 1, count, "re-importing doubled the history")
}

// A customer the base does not have is refused, not created: a sale filed
// against nobody is a sale nobody can call about.
func TestSalesImport_UnknownCustomerIsReportedNotInvented(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	known := seedSaleContact(t, accountID)

	preview := previewSales(t,
		"Cliente,Produto,Data,Valor\n"+
			"Clínica Bem Viver,Luvas,10/03/2026,\"1.500,00\"\n"+
			"ninguem@nada.com.br,Luvas,10/03/2026,\"1.500,00\"\n",
		token)

	assert.Equal(t, 1, preview.Importable)
	assert.Equal(t, 1, preview.UnknownCustomers)
	require.Len(t, preview.Rows, 2)
	assert.Equal(t, known, preview.Rows[0].ContactID, "resolved by name")
	assert.Equal(t, usecases.ProblemSaleCustomerUnknown, preview.Rows[1].Problem)

	confirmSales(t, preview, token)
	var contacts int
	require.NoError(t, testPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM contacts WHERE account_id = $1`, accountID).Scan(&contacts))
	assert.Equal(t, 1, contacts, "the import must not create customers")
}

// Writing the portfolio in bulk is an admin act, like the customer import.
func TestSalesImport_AttendantsCannotImport(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	attendant := tokenForSeededAccount(t, accountID, "attendant")

	w := doMultipartCSV(t, "/v1/sales/import/preview",
		"Cliente,Data,Valor\nx@y.com,10/03/2026,100\n", attendant)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// The wrong file says which columns it is missing.
func TestSalesImport_MissingColumnsAreNamed(t *testing.T) {
	cleanDB(t)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	w := doMultipartCSV(t, "/v1/sales/import/preview",
		"Nome,Preço\nGaze,49.90\n", token)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "SALE_IMPORT_NO_HEADER")
}

// ── Reorders per product on manual sales too ──────────────────────────────

// A customer who reordered gloves last week is not asked about gloves — but
// IS still asked about the gauze from two months ago. The old rule hid every
// product behind the customer's newest purchase of anything.
func TestPortfolio_ANewerPurchaseOfAnotherProductDoesNotCancelTheReorder(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	contactID := seedSaleContact(t, accountID)
	gloves := seedNamedProduct(t, accountID, "Luvas", 30)
	gauze := seedNamedProduct(t, accountID, "Gaze", 30)

	seedProductSale(t, accountID, contactID, gauze, 30*24*time.Hour)  // due now
	seedProductSale(t, accountID, contactID, gloves, 30*24*time.Hour) // due now
	seedProductSale(t, accountID, contactID, gloves, 1*24*time.Hour)  // gloves reordered

	list := parseJSONArray(t, doRequest("GET", "/v1/portfolio/reorders", nil, token))

	require.Len(t, list, 1, "gauze is still due; gloves were just reordered")
	items := list[0].(map[string]any)["items"].([]any)
	assert.Equal(t, gauze, items[0].(map[string]any)["product_id"])

	// And the customer's next_due_at is the gauze date, not the fresh gloves.
	detail := parseJSON(t, doRequest("GET", "/v1/portfolio/"+contactID, nil, token))
	assert.LessOrEqual(t, detail["days_until_due"].(float64), float64(0))
}

// seedProductSale is a sale of ONE catalogue product, with no sale-level
// cycle: the product's own cycle is what predicts.
func seedProductSale(t *testing.T, accountID, contactID, productID string, soldAgo time.Duration) string {
	t.Helper()
	id := ulid.Make().String()
	items, _ := json.Marshal([]domain.SaleItem{{ProductID: productID, Quantity: 10}})
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO sales (id, account_id, contact_id, items, gross_cents,
		    cost_cents, tax_cents, currency, sold_at, cycle_days, source)
		 VALUES ($1,$2,$3,$4,100000,0,0,'brl',NOW() - $5::interval,0,'manual')`,
		id, accountID, contactID, items, soldAgo.String())
	require.NoError(t, err)
	return id
}

// One sale carrying BOTH cycled products: offering the gloves must not
// silence the gauze — the nudge key is per product (migration 000224).
func TestPortfolio_OneSaleTwoProductsYieldsTwoReorders(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	contactID := seedSaleContact(t, accountID)
	gloves := seedNamedProduct(t, accountID, "Luvas", 30)
	gauze := seedNamedProduct(t, accountID, "Gaze", 30)

	saleID := ulid.Make().String()
	items, _ := json.Marshal([]domain.SaleItem{
		{ProductID: gloves, Quantity: 10},
		{ProductID: gauze, Quantity: 5},
	})
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO sales (id, account_id, contact_id, items, gross_cents,
		    cost_cents, tax_cents, currency, sold_at, cycle_days, source)
		 VALUES ($1,$2,$3,$4,100000,0,0,'brl',NOW() - INTERVAL '31 days',0,'manual')`,
		saleID, accountID, contactID, items)
	require.NoError(t, err)

	list := parseJSONArray(t, doRequest("GET", "/v1/portfolio/reorders", nil, token))
	require.Len(t, list, 2, "both products of the one sale are due")

	// The gloves were already offered (a nudge exists for THAT product)…
	_, err = testPool.Exec(context.Background(),
		`INSERT INTO reorder_nudges (id, account_id, contact_id, sale_id, product_key, due_at)
		 VALUES ($1,$2,$3,$4,$5,NOW())`,
		ulid.Make().String(), accountID, contactID, saleID, gloves)
	require.NoError(t, err)

	// …and the gauze prediction survives.
	list = parseJSONArray(t, doRequest("GET", "/v1/portfolio/reorders", nil, token))
	require.Len(t, list, 1, "the gauze must still be offered — the old per-sale key hid it forever")
	items2 := list[0].(map[string]any)["items"].([]any)
	assert.Equal(t, gauze, items2[0].(map[string]any)["product_id"])
}
