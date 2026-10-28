package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The adjustment a contract is owed, against a real database (§6.11, G5/G6).
//
// This is the number the product is sold on: a supply contract whose index was
// never applied loses the difference every month, permanently. So the tests are
// about it being right, and about it being ABSENT whenever it cannot be right.

// seedIndexSeries writes a flat monthly series into the shared cache.
func seedIndexSeries(t *testing.T, code string, first time.Time, months int, rate float64) {
	t.Helper()
	for i := 0; i < months; i++ {
		month := first.AddDate(0, i, 0)
		_, err := testPool.Exec(context.Background(),
			`INSERT INTO economic_index_values (index_code, reference_month, monthly_rate)
			 VALUES ($1,$2,$3)
			 ON CONFLICT (index_code, reference_month) DO UPDATE
			    SET monthly_rate = EXCLUDED.monthly_rate`,
			code, month, rate)
		require.NoError(t, err)
	}
}

// adjustableContract is a supply contract whose price rises with IPCA every
// March, started two years ago and never adjusted — the customer with the most
// money on the table.
func adjustableContract() map[string]any {
	start := time.Now().UTC().AddDate(-2, 0, 0)
	body := supplyContractBody()
	body["starts_on"] = time.Date(start.Year(), time.March, 1, 0, 0, 0, 0, time.UTC).
		Format("2006-01-02")
	body["ends_on"] = time.Date(start.Year()+3, time.March, 1, 0, 0, 0, 0, time.UTC).
		Format("2006-01-02")
	body["adjustment_index"] = "IPCA"
	body["adjustment_month"] = 3
	body["value_cents"] = 10000000 // R$ 100.000
	// Confirmed on creation: an unconfirmed reading must never drive a price
	// suggestion, which the next test pins.
	body["confirm"] = true
	return body
}

func TestContractAdjustment_ShowsWhatThePriceIsOwed(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	// A generous window of series so every month between the base date and the
	// last closed month is present.
	seedIndexSeries(t, "ipca",
		time.Date(time.Now().UTC().Year()-4, time.January, 1, 0, 0, 0, 0, time.UTC),
		72, 0.5)

	created := createContract(t, token, adjustableContract())
	id, _ := created["id"].(string)
	require.NotEmpty(t, id)

	detail := parseJSON(t, doRequest("GET", "/v1/contracts/"+id, nil, token))
	adjustment, _ := detail["adjustment"].(map[string]any)
	require.NotNil(t, adjustment, "a confirmed contract with an index is owed an adjustment")

	accumulation, _ := adjustment["accumulation"].(map[string]any)
	require.NotNil(t, accumulation)
	assert.Equal(t, true, accumulation["complete"])
	// Compounded, never summed: twelve months of 0.5% is 6.17%, not 6.00%.
	percent, _ := accumulation["percent"].(float64)
	assert.Greater(t, percent, 6.0, "at least a year of 0.5%% compounded")

	// The number that makes somebody open the alert.
	require.NotNil(t, adjustment["suggested_value_cents"])
	suggested, _ := adjustment["suggested_value_cents"].(float64)
	gain, _ := adjustment["gain_cents"].(float64)
	assert.Greater(t, suggested, 10000000.0)
	assert.InDelta(t, suggested-10000000, gain, 1)
	// Two years in with a March adjustment never applied: overdue.
	assert.Equal(t, true, adjustment["overdue"])
}

// The gate: no suggested price on a series with a hole in it. A figure that is
// quietly low is what gets quoted to a customer who then holds the correct one.
func TestContractAdjustment_PartialSeriesCarriesNoPrice(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	// Only three months, nowhere near the range the accumulation needs.
	seedIndexSeries(t, "ipca",
		time.Date(time.Now().UTC().Year(), time.January, 1, 0, 0, 0, 0, time.UTC),
		3, 0.5)

	created := createContract(t, token, adjustableContract())
	id, _ := created["id"].(string)

	detail := parseJSON(t, doRequest("GET", "/v1/contracts/"+id, nil, token))
	adjustment, _ := detail["adjustment"].(map[string]any)
	require.NotNil(t, adjustment)

	accumulation, _ := adjustment["accumulation"].(map[string]any)
	require.NotNil(t, accumulation)
	assert.Equal(t, false, accumulation["complete"])
	assert.NotEmpty(t, accumulation["missing_months"], "the gap has to be named")
	// The date still fires — the reminder is a fact even when the percentage is
	// not — but there is nothing to quote.
	assert.NotEmpty(t, adjustment["due_on"])
	assert.Nil(t, adjustment["suggested_value_cents"])
	assert.Nil(t, adjustment["gain_cents"])
}

func TestContractAdjustment_AbsentUntilAHumanConfirms(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	seedIndexSeries(t, "ipca",
		time.Date(time.Now().UTC().Year()-4, time.January, 1, 0, 0, 0, 0, time.UTC),
		72, 0.5)

	// Same contract, NOT confirmed.
	body := adjustableContract()
	delete(body, "confirm")
	created := createContract(t, token, body)
	id, _ := created["id"].(string)

	detail := parseJSON(t, doRequest("GET", "/v1/contracts/"+id, nil, token))
	assert.Nil(t, detail["adjustment"],
		"suggesting a new price from an unconfirmed reading is the product inventing "+
			"a number to quote a customer")

	// Confirming it turns the reading into a fact, and the number appears.
	body["confirm"] = true
	w := doRequest("PUT", "/v1/contracts/"+id, body, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	confirmed := parseJSON(t, doRequest("GET", "/v1/contracts/"+id, nil, token))
	require.NotNil(t, confirmed["adjustment"])
}

func TestContractAdjustment_UnknownIndexAndTheRegistryBalance(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	// An index nobody can look up: the alert still stands, with no percentage.
	body := adjustableContract()
	body["adjustment_index"] = "tabela própria do fornecedor"
	created := createContract(t, token, body)
	id, _ := created["id"].(string)

	detail := parseJSON(t, doRequest("GET", "/v1/contracts/"+id, nil, token))
	adjustment, _ := detail["adjustment"].(map[string]any)
	require.NotNil(t, adjustment)
	assert.Nil(t, adjustment["accumulation"])
	assert.Nil(t, adjustment["suggested_value_cents"])
	assert.NotEmpty(t, adjustment["due_on"])

	// The ata de registro de preços: pre-approved revenue with a balance and the
	// date that balance was last known. A balance with no date is a number nobody
	// can trust, so both round-trip.
	registry := adjustableContract()
	registry["title"] = "Ata 42/2026 — luvas"
	registry["price_registry"] = true
	registry["registry_number"] = "42/2026"
	registry["registry_balance_cents"] = 8000000
	registry["registry_balance_on"] = time.Now().UTC().Format("2006-01-02")
	ata := createContract(t, token, registry)

	assert.Equal(t, true, ata["price_registry"])
	assert.Equal(t, "42/2026", ata["registry_number"])
	assert.EqualValues(t, 8000000, ata["registry_balance_cents"])
	assert.NotEmpty(t, ata["registry_balance_on"])

	// A negative balance is refused by the column itself — pre-approved revenue
	// cannot be less than nothing.
	broken := registry
	broken["registry_balance_cents"] = -1
	w := doRequest("POST", "/v1/contracts", broken, token)
	assert.NotEqual(t, http.StatusOK, w.Code, fmt.Sprintf("body: %s", w.Body.String()))
}

// Recording the raise stops the accumulation from double-counting it: without
// last_adjusted_on the claim would run from the contract's start forever.
func TestContractAdjustment_RecordingTheRaiseResetsTheBase(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	seedIndexSeries(t, "ipca",
		time.Date(time.Now().UTC().Year()-4, time.January, 1, 0, 0, 0, 0, time.UTC),
		72, 0.5)

	created := createContract(t, token, adjustableContract())
	id, _ := created["id"].(string)
	before := parseJSON(t, doRequest("GET", "/v1/contracts/"+id, nil, token))
	beforeAdj, _ := before["adjustment"].(map[string]any)
	beforeAcc, _ := beforeAdj["accumulation"].(map[string]any)
	beforeMonths, _ := beforeAcc["months"].(float64)
	require.Greater(t, beforeMonths, 12.0)

	// The price was raised last month.
	body := adjustableContract()
	body["last_adjusted_on"] = time.Now().UTC().AddDate(0, -1, 0).Format("2006-01-02")
	w := doRequest("PUT", "/v1/contracts/"+id, body, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	after := parseJSON(t, doRequest("GET", "/v1/contracts/"+id, nil, token))
	afterAdj, _ := after["adjustment"].(map[string]any)
	require.NotNil(t, afterAdj)
	afterAcc, _ := afterAdj["accumulation"].(map[string]any)
	afterMonths, _ := afterAcc["months"].(float64)

	assert.Less(t, afterMonths, beforeMonths,
		"the accumulation counts from the last raise, not from the contract's start")
	// And the next date is a year out, not overdue.
	assert.Equal(t, false, afterAdj["overdue"])
}
