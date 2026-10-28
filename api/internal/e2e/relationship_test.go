package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Relationship routines and cross-sell against a real database (§6.5, F4/F5).
//
// Both features are almost entirely SQL — a date window evaluated in each
// account's own timezone, and a co-purchase aggregation — so a unit test with
// a fake repository would pin nothing that can actually break.

func seedSaleWithItems(t *testing.T, accountID, contactID string, items string) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO sales (id, account_id, contact_id, items, gross_cents,
		    cost_cents, tax_cents, currency, sold_at, cycle_days, source)
		 VALUES ($1,$2,$3,$4,100000,60000,0,'brl',NOW() - INTERVAL '10 days',30,'manual')`,
		ulid.Make().String(), accountID, contactID, items)
	require.NoError(t, err)
}

func setAccountTimezone(t *testing.T, accountID, tz string) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`UPDATE accounts SET timezone = $2 WHERE id = $1`, accountID, tz)
	require.NoError(t, err)
}

func TestRelationship_SavesAndListsADate(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	contactID := seedSaleContact(t, accountID)

	res := doRequest("POST", "/v1/relationship-dates", map[string]any{
		"kind": "birthday", "contact_id": contactID, "month": 5, "day": 12,
		"label": "Aniversário da Dra. Ana",
	}, token)
	require.Equal(t, http.StatusOK, res.Code)

	list := parseJSONArray(t, doRequest("GET", "/v1/relationship-dates", nil, token))
	require.Len(t, list, 1)
	assert.Equal(t, "birthday", list[0].(map[string]any)["kind"])
}

// The catalogue exists so nobody has to research that Dia do Médico is the
// 18th of October.
func TestRelationship_SectorKeyFillsTheDate(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	res := doRequest("POST", "/v1/relationship-dates", map[string]any{
		"kind": "sector", "sector_key": "medico",
	}, token)
	require.Equal(t, http.StatusOK, res.Code)

	body := parseJSON(t, res)
	assert.EqualValues(t, 10, body["month"])
	assert.EqualValues(t, 18, body["day"])
}

// One account's dates are not another's.
func TestRelationship_DatesAreScopedToTheAccount(t *testing.T) {
	cleanDB(t)
	mine := seedIdentityAccount(t)
	theirs := seedIdentityAccount(t)
	myToken := tokenForSeededAccount(t, mine, "admin")

	require.Equal(t, http.StatusOK, doRequest("POST", "/v1/relationship-dates", map[string]any{
		"kind": "custom", "month": 3, "day": 3, "label": "deles",
	}, tokenForSeededAccount(t, theirs, "admin")).Code)

	list := parseJSONArray(t, doRequest("GET", "/v1/relationship-dates", nil, myToken))
	assert.Empty(t, list)
}

func TestRelationship_SectorsCatalogueIsServed(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	list := parseJSONArray(t, doRequest("GET", "/v1/relationship-dates/sectors", nil, token))
	// "sectors" must not be swallowed as a date id by the :id route.
	require.NotEmpty(t, list)
}

// The query the worker runs. A birthday two days out is due; one a week out
// is not.
func TestRelationship_DueDatesRespectsTheLeadWindow(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	setAccountTimezone(t, accountID, "America/Sao_Paulo")
	contactID := seedSaleContact(t, accountID)

	ctx := context.Background()
	repo := postgres.NewRelationshipRepository(testPool)
	now := time.Now()
	soon := now.AddDate(0, 0, 1)
	later := now.AddDate(0, 0, 20)

	require.NoError(t, repo.Upsert(ctx, &domain.RelationshipDate{
		ID: ulid.Make().String(), AccountID: accountID, ContactID: contactID,
		Kind: domain.DateBirthday, Month: int(soon.Month()), Day: soon.Day(),
		Label: "soon"}))
	require.NoError(t, repo.Upsert(ctx, &domain.RelationshipDate{
		ID: ulid.Make().String(), AccountID: accountID, ContactID: contactID,
		Kind: domain.DateBirthday, Month: int(later.Month()), Day: later.Day(),
		Label: "later"}))

	due, err := repo.DueDates(ctx, now, 50)
	require.NoError(t, err)
	require.Len(t, due, 1)
	assert.Equal(t, "soon", due[0].Label)
}

// The guard that stops a daily worker greeting somebody every day of the
// window.
func TestRelationship_MarkFiredStopsARepeat(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	contactID := seedSaleContact(t, accountID)

	ctx := context.Background()
	repo := postgres.NewRelationshipRepository(testPool)
	now := time.Now()
	today := now.AddDate(0, 0, 1)

	id := ulid.Make().String()
	require.NoError(t, repo.Upsert(ctx, &domain.RelationshipDate{
		ID: id, AccountID: accountID, ContactID: contactID,
		Kind: domain.DateBirthday, Month: int(today.Month()), Day: today.Day()}))

	due, err := repo.DueDates(ctx, now, 50)
	require.NoError(t, err)
	require.Len(t, due, 1)

	require.NoError(t, repo.MarkFired(ctx, id, now))

	due, err = repo.DueDates(ctx, now, 50)
	require.NoError(t, err)
	assert.Empty(t, due, "a date already fired this year must not come back")
}

// An invalid timezone on ONE account must not take the query down for every
// other account — AT TIME ZONE raises on an unknown name.
func TestRelationship_ABadTimezoneDoesNotBreakTheQuery(t *testing.T) {
	cleanDB(t)
	broken := seedIdentityAccount(t)
	healthy := seedIdentityAccount(t)
	setAccountTimezone(t, broken, "Mars/Olympus_Mons")
	setAccountTimezone(t, healthy, "Europe/Lisbon")

	ctx := context.Background()
	repo := postgres.NewRelationshipRepository(testPool)
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)

	for _, accountID := range []string{broken, healthy} {
		require.NoError(t, repo.Upsert(ctx, &domain.RelationshipDate{
			ID: ulid.Make().String(), AccountID: accountID,
			ContactID: seedSaleContact(t, accountID),
			Kind:      domain.DateBirthday, Month: int(tomorrow.Month()), Day: tomorrow.Day()}))
	}

	due, err := repo.DueDates(ctx, now, 50)
	require.NoError(t, err, "one broken timezone must not fail the whole query")
	assert.Len(t, due, 2)
}

// A one-off that came and went stays due: a missed maintenance does not stop
// mattering because the day passed.
func TestRelationship_OverdueMaintenanceStaysDue(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	contactID := seedSaleContact(t, accountID)

	ctx := context.Background()
	repo := postgres.NewRelationshipRepository(testPool)
	past := time.Now().AddDate(0, 0, -40)

	require.NoError(t, repo.Upsert(ctx, &domain.RelationshipDate{
		ID: ulid.Make().String(), AccountID: accountID, ContactID: contactID,
		Kind: domain.DateMaintenance, DueAt: &past, Label: "autoclave"}))

	due, err := repo.DueDates(ctx, time.Now(), 50)
	require.NoError(t, err)
	require.Len(t, due, 1)
	assert.Equal(t, "autoclave", due[0].Label)
}

// Only customers. Messaging every silent contact would turn a relationship
// routine into a second cold-outreach channel.
func TestRelationship_CheckInOnlyCoversCustomers(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	buyer := seedSaleContact(t, accountID)
	stranger := seedSaleContact(t, accountID)

	_, err := testPool.Exec(context.Background(),
		`INSERT INTO sales (id, account_id, contact_id, items, gross_cents,
		    cost_cents, tax_cents, currency, sold_at, cycle_days, source)
		 VALUES ($1,$2,$3,'[]',100000,0,0,'brl',NOW() - INTERVAL '120 days',0,'manual')`,
		ulid.Make().String(), accountID, buyer)
	require.NoError(t, err)

	repo := postgres.NewRelationshipRepository(testPool)
	candidates, err := repo.DueForCheckIn(context.Background(),
		time.Now().AddDate(0, 0, -domain.CheckInAfterDays), 50)
	require.NoError(t, err)

	require.Len(t, candidates, 1)
	assert.Equal(t, buyer, candidates[0].ContactID)
	assert.NotEqual(t, stranger, candidates[0].ContactID)
}

// A customer who stays quiet must not be messaged every day they stay quiet.
func TestRelationship_CheckInIsNotOfferedTwice(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	buyer := seedSaleContact(t, accountID)

	_, err := testPool.Exec(context.Background(),
		`INSERT INTO sales (id, account_id, contact_id, items, gross_cents,
		    cost_cents, tax_cents, currency, sold_at, cycle_days, source)
		 VALUES ($1,$2,$3,'[]',100000,0,0,'brl',NOW() - INTERVAL '120 days',0,'manual')`,
		ulid.Make().String(), accountID, buyer)
	require.NoError(t, err)

	ctx := context.Background()
	repo := postgres.NewRelationshipRepository(testPool)
	silentSince := time.Now().AddDate(0, 0, -domain.CheckInAfterDays)

	require.NoError(t, repo.RecordCheckIn(ctx, ulid.Make().String(),
		accountID, buyer, time.Now()))

	candidates, err := repo.DueForCheckIn(ctx, silentSince, 50)
	require.NoError(t, err)
	assert.Empty(t, candidates)
}

// F5. The pattern has to survive the way people actually type product names.
func TestCrossSell_LearnsFromTheAccountsOwnSales(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	// Four customers buy gloves; three of them also buy masks.
	buyers := make([]string, 4)
	for i := range buyers {
		buyers[i] = seedSaleContact(t, accountID)
		items := `[{"description":"Luvas de procedimento cx 100"}]`
		if i < 3 {
			// Deliberately spelled differently every time — packaging,
			// accents, plural. The normaliser is what makes this one product.
			spellings := []string{
				`[{"description":"Luvas de procedimento cx 100"},{"description":"Máscara cirúrgica cx 50"}]`,
				`[{"description":"LUVA DE PROCEDIMENTO caixa c/100"},{"description":"mascaras cirurgicas"}]`,
				`[{"description":"luva de procedimento"},{"description":"Mascara Cirurgica pct 50"}]`,
			}
			items = spellings[i]
		}
		seedSaleWithItems(t, accountID, buyers[i], items)
	}

	// The customer who buys gloves and NOT masks is the one to call.
	res := doRequest("GET", fmt.Sprintf("/v1/portfolio/%s/cross-sell", buyers[3]), nil, token)
	require.Equal(t, http.StatusOK, res.Code)

	body := parseJSON(t, res)
	suggestions, _ := body["suggestions"].([]any)
	require.Len(t, suggestions, 1)

	first := suggestions[0].(map[string]any)
	assert.Contains(t, first["suggest"], "mascara")
	// And it says why, so the salesperson can too.
	assert.Contains(t, first["because"], "luva")
	assert.EqualValues(t, 3, first["customers"])
}

// Never suggest what they already buy.
func TestCrossSell_SkipsWhatTheCustomerAlreadyBuys(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	var last string
	for i := 0; i < 4; i++ {
		last = seedSaleContact(t, accountID)
		seedSaleWithItems(t, accountID, last,
			`[{"description":"Luva de procedimento"},{"description":"Mascara cirurgica"}]`)
	}

	body := parseJSON(t, doRequest("GET",
		fmt.Sprintf("/v1/portfolio/%s/cross-sell", last), nil, token))
	suggestions, _ := body["suggestions"].([]any)
	assert.Empty(t, suggestions)
}

// "We do not know yet" is the honest answer in a distributor's first month —
// two customers is a coincidence, not a pattern.
func TestCrossSell_TwoCustomersIsNotAPattern(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	for i := 0; i < 2; i++ {
		seedSaleWithItems(t, accountID, seedSaleContact(t, accountID),
			`[{"description":"Luva"},{"description":"Mascara"}]`)
	}
	lonely := seedSaleContact(t, accountID)
	seedSaleWithItems(t, accountID, lonely, `[{"description":"Luva"}]`)

	body := parseJSON(t, doRequest("GET",
		fmt.Sprintf("/v1/portfolio/%s/cross-sell", lonely), nil, token))
	suggestions, _ := body["suggestions"].([]any)
	assert.Empty(t, suggestions)
}

// What a distributor sells to whom is their commercial position. Pooling it
// across accounts would mean one customer's product mix informing a
// competitor's pitch.
func TestCrossSell_NeverLearnsFromAnotherAccount(t *testing.T) {
	cleanDB(t)
	mine := seedIdentityAccount(t)
	theirs := seedIdentityAccount(t)
	myToken := tokenForSeededAccount(t, mine, "admin")

	for i := 0; i < 5; i++ {
		seedSaleWithItems(t, theirs, seedSaleContact(t, theirs),
			`[{"description":"Luva"},{"description":"Mascara"}]`)
	}
	myCustomer := seedSaleContact(t, mine)
	seedSaleWithItems(t, mine, myCustomer, `[{"description":"Luva"}]`)

	body := parseJSON(t, doRequest("GET",
		fmt.Sprintf("/v1/portfolio/%s/cross-sell", myCustomer), nil, myToken))
	suggestions, _ := body["suggestions"].([]any)
	assert.Empty(t, suggestions, "another account's sales must never inform mine")
}
