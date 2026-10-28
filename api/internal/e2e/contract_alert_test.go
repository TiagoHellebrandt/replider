package e2e

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
	"github.com/replider/api/internal/core/usecases"
)

// Clause alerts, the renewal sweep, and the recovered-revenue ledger against a
// real database (§6.11, G5/G6).
//
// The three properties under test: the flag list comes only from a confirmed
// reading, scheduling the same deadline twice produces one task, and the headline
// number is what a HUMAN said was collected.

// renewingContract ends inside the 90-day horizon and renews itself with a
// 60-day notice window, so its deadline has already lapsed — the state that
// costs a year of a contract nobody wanted.
func renewingContract() map[string]any {
	now := time.Now().UTC()
	body := supplyContractBody()
	body["starts_on"] = now.AddDate(-1, 0, 0).Format("2006-01-02")
	body["ends_on"] = now.AddDate(0, 0, 45).Format("2006-01-02")
	body["auto_renewal"] = true
	body["notice_days"] = 60
	body["value_cents"] = 10000000
	body["confirm"] = true
	return body
}

func TestContractAlerts_FlagsComeFromAConfirmedReadingOnly(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	// Unconfirmed first: the list is empty AND says why. An empty list on its own
	// reads as a clean contract.
	unconfirmed := renewingContract()
	delete(unconfirmed, "confirm")
	created := createContract(t, token, unconfirmed)
	id, _ := created["id"].(string)

	before := parseJSON(t, doRequest("GET", "/v1/contracts/"+id+"/attention", nil, token))
	assert.Equal(t, false, before["confirmed"])
	assert.Empty(t, before["flags"])
	// The plain-language summary still works: it is composed from whatever fields
	// exist, and none of it is a claim about a clause.
	assert.NotEmpty(t, before["summary"])

	// Confirming turns the reading into facts, and the flags appear.
	confirmed := renewingContract()
	require.Equal(t, http.StatusOK,
		doRequest("PUT", "/v1/contracts/"+id, confirmed, token).Code)

	after := parseJSON(t, doRequest("GET", "/v1/contracts/"+id+"/attention", nil, token))
	assert.Equal(t, true, after["confirmed"])
	flags, _ := after["flags"].([]any)
	require.NotEmpty(t, flags)

	kinds := map[string]map[string]any{}
	for _, raw := range flags {
		f, _ := raw.(map[string]any)
		kinds[f["kind"].(string)] = f
	}
	// The renewal window lapsed 15 days ago (ends in 45, needs 60 days' notice).
	renewal, ok := kinds["renewal_notice"]
	require.True(t, ok, "automatic renewal must be flagged")
	assert.Equal(t, "critical", renewal["severity"])
	assert.EqualValues(t, 10000000, renewal["money_at_stake_cents"])
	// Exclusivity is on the list with no date — a constraint, not a deadline.
	exclusivity, ok := kinds["exclusivity"]
	require.True(t, ok)
	assert.Nil(t, exclusivity["due_on"])
	// And the total is what the whole list is worth.
	assert.Greater(t, after["total_at_stake_cents"], 0.0)
}

func TestContractAlerts_ScheduleIsOneClickAndIdempotent(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	created := createContract(t, token, renewingContract())
	id, _ := created["id"].(string)

	first := doRequest("POST", "/v1/contracts/"+id+"/alerts",
		map[string]any{"kind": "renewal_notice"}, token)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	alert := parseJSON(t, first)
	alertID, _ := alert["id"].(string)
	require.NotEmpty(t, alertID)
	// The date and the wording come from the CLAUSE, not from the request: a
	// client that could set its own date could schedule the reminder for the day
	// after the window closes.
	assert.NotEmpty(t, alert["due_on"])
	assert.Contains(t, alert["title"], "renov")

	// Asking twice is the same task, not two.
	second := doRequest("POST", "/v1/contracts/"+id+"/alerts",
		map[string]any{"kind": "renewal_notice"}, token)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	assert.Equal(t, alertID, parseJSON(t, second)["id"])

	// And the flag now reports itself scheduled, so the UI offers "agendado"
	// instead of a second button that would be a no-op.
	attention := parseJSON(t, doRequest("GET", "/v1/contracts/"+id+"/attention", nil, token))
	flags, _ := attention["flags"].([]any)
	var scheduled bool
	for _, raw := range flags {
		f, _ := raw.(map[string]any)
		if f["kind"] == "renewal_notice" {
			scheduled, _ = f["scheduled"].(bool)
		}
	}
	assert.True(t, scheduled)

	// A clause the contract does not contain cannot be scheduled: the request is
	// well-formed and the paper disagrees, which is a 409.
	noRegistry := doRequest("POST", "/v1/contracts/"+id+"/alerts",
		map[string]any{"kind": "registry_balance"}, token)
	assert.Equal(t, http.StatusConflict, noRegistry.Code, noRegistry.Body.String())
	assert.Equal(t, "CONTRACT_ALERT_NOT_APPLICABLE", parseJSON(t, noRegistry)["code"])

	// A custom reminder needs its own date and text.
	custom := doRequest("POST", "/v1/contracts/"+id+"/alerts", map[string]any{
		"kind": "custom", "due_on": time.Now().UTC().AddDate(0, 0, 10).Format("2006-01-02"),
		"title": "Pedir a certidão negativa",
	}, token)
	require.Equal(t, http.StatusOK, custom.Code, custom.Body.String())
	assert.Equal(t, http.StatusBadRequest,
		doRequest("POST", "/v1/contracts/"+id+"/alerts",
			map[string]any{"kind": "custom", "title": "sem data"}, token).Code)
}

func TestContractAlerts_DueListLeadsWithWhatAlreadyLapsed(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	created := createContract(t, token, renewingContract())
	id, _ := created["id"].(string)
	require.Equal(t, http.StatusOK,
		doRequest("POST", "/v1/contracts/"+id+"/alerts",
			map[string]any{"kind": "renewal_notice"}, token).Code)

	list := parseJSON(t, doRequest("GET", "/v1/contract-alerts", nil, token))
	alerts, _ := list["alerts"].([]any)
	require.NotEmpty(t, alerts)

	first, _ := alerts[0].(map[string]any)
	// Negative days: the deadline passed. It must not read as zero, and it sorts
	// first because a lapsed deadline is more urgent than one coming.
	days, _ := first["days_until_due"].(float64)
	assert.Less(t, days, 0.0)
	assert.Equal(t, true, first["overdue"])
	assert.EqualValues(t, 1, list["overdue"])
	assert.Greater(t, list["total_at_stake_cents"], 0.0)
	// The contract's title travels with the alert, so a list of deadlines is
	// readable without a fetch per row.
	assert.NotEmpty(t, first["contract_title"])
}

func TestContractAlerts_CompletingRecordsWhatWasCollected(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	created := createContract(t, token, renewingContract())
	id, _ := created["id"].(string)
	alert := parseJSON(t, doRequest("POST", "/v1/contracts/"+id+"/alerts",
		map[string]any{"kind": "renewal_notice"}, token))
	alertID, _ := alert["id"].(string)

	// Dismissing and claiming money at once is a contradiction: one says nothing
	// happened, the other says money arrived.
	bad := doRequest("POST", "/v1/contract-alerts/"+alertID+"/complete",
		map[string]any{"dismiss": true, "recovered_cents": 500000}, token)
	assert.Equal(t, http.StatusBadRequest, bad.Code, bad.Body.String())

	done := doRequest("POST", "/v1/contract-alerts/"+alertID+"/complete", map[string]any{
		"note": "renegociado com reajuste de 6,7%", "recovered_cents": 670000,
	}, token)
	require.Equal(t, http.StatusOK, done.Code, done.Body.String())
	assert.Equal(t, "done", parseJSON(t, done)["status"])

	// The headline: what this account went back and collected.
	recovered := parseJSON(t, doRequest("GET", "/v1/contract-recoveries", nil, token))
	assert.EqualValues(t, 670000, recovered["total_cents"])
	assert.EqualValues(t, 1, recovered["count"])
	byKind, _ := recovered["by_kind"].(map[string]any)
	// A renewal-notice deadline recovered renewal revenue: the split is what makes
	// "recuperei R$400k" mean something.
	assert.EqualValues(t, 670000, byKind["renewal"])
	// The list behind the total, because a headline nobody can drill into is a
	// headline nobody believes.
	ledger, _ := recovered["recoveries"].([]any)
	require.Len(t, ledger, 1)
	row, _ := ledger[0].(map[string]any)
	assert.NotEmpty(t, row["contract_title"])

	// Completing twice does not restamp the closing date, and does not double the
	// ledger.
	require.Equal(t, http.StatusOK,
		doRequest("POST", "/v1/contract-alerts/"+alertID+"/complete",
			map[string]any{"note": "de novo"}, token).Code)
	again := parseJSON(t, doRequest("GET", "/v1/contract-recoveries", nil, token))
	assert.EqualValues(t, 670000, again["total_cents"])

	// Revenue recovered WITHOUT the product prompting it still counts: a ledger
	// that only counted its own prompts would flatter the product.
	manual := doRequest("POST", "/v1/contract-recoveries", map[string]any{
		"contract_id": id, "kind": "adjustment", "amount_cents": 120000,
		"note": "reajuste aplicado na renovação anterior",
	}, token)
	require.Equal(t, http.StatusOK, manual.Code, manual.Body.String())
	total := parseJSON(t, doRequest("GET", "/v1/contract-recoveries", nil, token))
	assert.EqualValues(t, 790000, total["total_cents"])

	// A recovery with no amount is refused: the headline is the whole point.
	assert.Equal(t, http.StatusBadRequest,
		doRequest("POST", "/v1/contract-recoveries", map[string]any{
			"contract_id": id, "kind": "adjustment",
		}, token).Code)
}

// The sweep: dates become tasks, and the renewal becomes a LEAD in the funnel.
func TestContractAlertWorker_TurnsDatesIntoWorkOnce(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	created := createContract(t, token, renewingContract())
	id, _ := created["id"].(string)

	contractRepo := postgres.NewContractRepository(testPool)
	alertRepo := postgres.NewContractAlertRepository(testPool)
	worker := services.NewContractAlertWorker(contractRepo, alertRepo).
		WithPricer(usecases.NewContractAdjustmentUseCase(
			contractRepo, postgres.NewEconomicIndexRepository(testPool)))

	worker.RunOnce(context.Background())
	first, err := alertRepo.ListByContract(context.Background(), accountID, id)
	require.NoError(t, err)
	require.NotEmpty(t, first, "the sweep must create the deadlines")

	// Idempotent: every alert is keyed on (contract, kind, date) while pending, so
	// a second sweep today produces the same rows rather than a duplicate task.
	worker.RunOnce(context.Background())
	second, err := alertRepo.ListByContract(context.Background(), accountID, id)
	require.NoError(t, err)
	assert.Len(t, second, len(first))

	// Every alert the sweep wrote is dated and priced.
	for _, a := range second {
		assert.False(t, a.DueOn.IsZero())
		assert.NotEmpty(t, a.Title)
		assert.Equal(t, domain.AlertPending, a.Status)
	}

	// No contact on this contract, so no lead was opened — the deadline stands
	// anyway. Better a deadline with no lead than a lead with no counterpart.
	for _, a := range second {
		assert.Empty(t, a.LeadID)
	}
}

func TestContractAlerts_TenancyAndRole(t *testing.T) {
	cleanDB(t)
	accountA := seedIdentityAccount(t)
	accountB := seedIdentityAccount(t)
	tokenA := tokenForSeededAccount(t, accountA, "admin")
	tokenB := tokenForSeededAccount(t, accountB, "admin")
	attendant := tokenForSeededAccount(t, accountA, "attendant")

	created := createContract(t, tokenA, renewingContract())
	id, _ := created["id"].(string)
	alert := parseJSON(t, doRequest("POST", "/v1/contracts/"+id+"/alerts",
		map[string]any{"kind": "renewal_notice"}, tokenA))
	alertID, _ := alert["id"].(string)

	// Another company's deadlines are not readable, schedulable or closable.
	assert.Equal(t, http.StatusNotFound,
		doRequest("GET", "/v1/contracts/"+id+"/attention", nil, tokenB).Code)
	assert.Equal(t, http.StatusNotFound,
		doRequest("POST", "/v1/contracts/"+id+"/alerts",
			map[string]any{"kind": "renewal_notice"}, tokenB).Code)
	assert.Equal(t, http.StatusNotFound,
		doRequest("POST", "/v1/contract-alerts/"+alertID+"/complete",
			map[string]any{"note": "alheio"}, tokenB).Code)
	empty := parseJSON(t, doRequest("GET", "/v1/contract-alerts", nil, tokenB))
	assert.Empty(t, empty["alerts"])
	assert.EqualValues(t, 0, parseJSON(t,
		doRequest("GET", "/v1/contract-recoveries", nil, tokenB))["total_cents"])

	// The whole module is admin-only: the redaction that lets an attendant read a
	// contract's term does not extend to the money it recovered.
	for _, path := range []string{
		"/v1/contracts/" + id + "/attention",
		"/v1/contract-alerts",
		"/v1/contract-recoveries",
	} {
		assert.Equal(t, http.StatusForbidden,
			doRequest("GET", path, nil, attendant).Code, path)
	}
	assert.Equal(t, http.StatusForbidden,
		doRequest("POST", "/v1/contracts/"+id+"/alerts",
			map[string]any{"kind": "renewal_notice"}, attendant).Code)
}
