package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
)

// The Radar journey (Ofertas phase 2/3): territory + catalogue → the hunt
// finds a CNES establishment → "Preparar oferta" makes the lead in the
// default funnel → the approval queue is where it waits. Driven through the
// HTTP surface (no INSERT builds account state); the CNES rows are seeded
// through the same repo the ingest worker writes, because they are public
// data no API of ours creates.

func seedCNESPark(t *testing.T) {
	t.Helper()
	repo := postgres.NewCNESParkRepository(testPool)
	require.NoError(t, repo.ReplaceCompetencia("MT", "202607", []domain.CNESEquipmentRow{
		{CNES: "2655411", EquipCode: "0111", Competencia: "202607", CityCode: "510340", UF: "MT", QtExist: 2, QtUso: 1, SUS: true},
	}))
	phone := "65 3618-8000"
	require.NoError(t, repo.UpsertEstablishment(&domain.CNESEstablishment{
		CNES: "2655411", TradeName: "Hospital São Mateus",
		Address: "Av. X, Cuiabá", Phone: &phone,
	}))
}

func TestRadarJourney_HuntFindsParkAndParkOfferLandsOneLead(t *testing.T) {
	cleanDB(t)
	seedCNESPark(t)
	registerUser(t, "Cuiabamed", "dona@cuiabamed.test", "pass1234")
	verifyEmail(t, "dona@cuiabamed.test")
	token := login(t, "dona@cuiabamed.test", "pass1234")

	// Territory + a catalogue product whose name matches the CNES dictionary.
	w := doRequest("PATCH", "/v1/accounts", map[string]interface{}{
		"territory": map[string]interface{}{"uf": "mt", "cities": []string{"Cuiabá"}},
	}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	// Through the import door — the CRUD handler is deliberately nil in e2e.
	w = doRequest("POST", "/v1/products/import", map[string]interface{}{
		"rows": []map[string]interface{}{{
			"line": 1, "name": "Tomógrafo computadorizado GE",
			"price_cents": 100000, "currency": "brl",
		}},
	}, token)
	require.Less(t, w.Code, 300, w.Body.String())

	// The hunt answers with the establishment, counts only.
	w = doRequest("POST", "/v1/me/territory/hunt",
		map[string]interface{}{"equipment": "tomógrafo"}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	hunt := parseJSON(t, w)
	holders := hunt["holders"].([]interface{})
	require.Len(t, holders, 1)
	holder := holders[0].(map[string]interface{})
	assert.Equal(t, "2655411", holder["cnes"])
	assert.Equal(t, "Hospital São Mateus", holder["trade_name"])
	assert.Equal(t, false, holder["already_known"])
	// Which month, and which cities: the answer says both.
	assert.Equal(t, "2026-07", hunt["competencia"])
	assert.Equal(t, []interface{}{"Cuiabá"}, hunt["cities"])

	// The radar's park collector surfaces the same ground.
	w = doRequest("GET", "/v1/me/radar", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	radar := parseJSON(t, w)
	items := radar["items"].([]interface{})
	foundPark := false
	for _, raw := range items {
		item := raw.(map[string]interface{})
		if item["type"] == "parque" && item["cnes"] == "2655411" {
			foundPark = true
			// The honesty rule rides in the payload copy itself.
			assert.Contains(t, item["context"], "quantidade — não idade")
			// The territory names Cuiabá (IBGE 510340 = the seeded row's
			// city): the scope is the city, not the state, and the item
			// says which DATASUS month it is quoting.
			assert.Contains(t, item["context"], "no seu território (Cuiabá)")
			assert.Equal(t, "2026-07", item["competencia"])
		}
	}
	assert.True(t, foundPark, "the park item must be in the radar feed: %s", w.Body.String())

	// "Preparar oferta": no AI key in e2e, so the DRAFT is refused after the
	// lead exists (the provider init error surfaces as 502) — what matters
	// is that the lead was made exactly once and stays draftable by hand.
	countLeads := func() int {
		var n int
		require.NoError(t, testPool.QueryRow(context.Background(),
			`SELECT count(*) FROM leads WHERE source = 'cnes_park'`).Scan(&n))
		return n
	}
	w = doRequest("POST", "/v1/me/radar/park-offer",
		map[string]interface{}{"cnes": "2655411"}, token)
	assert.NotEqual(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 1, countLeads(), "the establishment became a lead in the default funnel")

	doRequest("POST", "/v1/me/radar/park-offer",
		map[string]interface{}{"cnes": "2655411"}, token)
	assert.Equal(t, 1, countLeads(), "a second click redrafts, never duplicates")

	// The hunt now labels the ground as known — the seller cannot
	// "discover" their own pipeline.
	w = doRequest("POST", "/v1/me/territory/hunt",
		map[string]interface{}{"equipment": "tomógrafo"}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	hunt = parseJSON(t, w)
	holder = hunt["holders"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, true, holder["already_known"])
}

// A tender the auto-offer already drafted must STAY on the Radar — with its
// lead_id, so the card says "abrir rascunho" — while leaving the /signals
// feed, which is the "not yet worked" list. Before this, LinkLead made the
// tender vanish from the Radar exactly when it became work.
func TestRadarJourney_WorkedTenderStaysOnTheRadarWithItsLead(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	repo := postgres.NewSignalRepository(testPool)

	// A fresh account has nothing: the poller reads this as "backfill".
	n, err := repo.CountByAccount(ctx, accountID)
	require.NoError(t, err)
	assert.Zero(t, n)

	worked := seedSignal(t, accountID, nil)
	require.NoError(t, repo.LinkLead(ctx, worked.ID, accountID, "01LEADLEADLEADLEADLEADLEAD"))
	open := seedSignal(t, accountID, nil)

	n, err = repo.CountByAccount(ctx, accountID)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "worked rows still count: the account is no longer fresh")

	w := doRequest("GET", "/v1/me/radar", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	leadByID := map[string]interface{}{}
	for _, raw := range parseJSON(t, w)["items"].([]interface{}) {
		item := raw.(map[string]interface{})
		if item["type"] == "licitacao" {
			leadByID[item["id"].(string)] = item["lead_id"]
		}
	}
	require.Len(t, leadByID, 2, "both tenders on the radar: %s", w.Body.String())
	assert.Equal(t, "01LEADLEADLEADLEADLEADLEAD", leadByID[worked.ID], "the worked one carries its lead")
	assert.Nil(t, leadByID[open.ID])

	// The /signals feed is still "not yet worked".
	w = doRequest("GET", "/v1/signals", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var feedIDs []string
	for _, raw := range parseJSONArray(t, w) {
		feedIDs = append(feedIDs, raw.(map[string]interface{})["id"].(string))
	}
	assert.Equal(t, []string{open.ID}, feedIDs)
}
