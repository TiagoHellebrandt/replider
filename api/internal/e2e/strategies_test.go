package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A strategy is the written-down version of "como a gente vende", and it
// exists as an entity for one reason: attribution. Without a strategy id and
// VERSION on every lead, an A/B compares two unnamed things and "essa
// abordagem funciona" is a feeling.

func createStrategy(t *testing.T, token string, body map[string]any) map[string]any {
	t.Helper()
	if body["name"] == nil {
		body["name"] = "Consultiva — saúde"
	}
	w := doRequest("PUT", "/v1/strategies/new", body, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return parseJSON(t, w)
}

func TestStrategies_CreateAndList(t *testing.T) {
	cleanDB(t)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	created := createStrategy(t, token, map[string]any{
		"icp":       "hospitais de 50 a 200 leitos",
		"tone":      "consultivo, sem jargão",
		"arguments": []string{"redução de tempo de setup", "suporte 24h"},
		"objections": []any{
			map[string]any{"objection": "já temos fornecedor",
				"response": "não pedimos troca — pedimos um piloto num setor"},
		},
	})

	// New strategies start as drafts: half a strategy applied to a real
	// prospect is worse than none.
	assert.Equal(t, "draft", created["status"])
	assert.EqualValues(t, 1, created["version"])
	// Authorship is recognition, and recognition is what makes a salesperson
	// contribute one.
	assert.NotEmpty(t, created["author_user_id"])

	assert.Len(t, parseJSONArray(t, doRequest("GET", "/v1/strategies", nil, token)), 1)
}

// "Converte 14%" is meaningless if the arguments were rewritten twice since
// the number was earned.
func TestStrategies_EditingTheContentBumpsTheVersion(t *testing.T) {
	cleanDB(t)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")
	created := createStrategy(t, token, map[string]any{"icp": "hospitais"})
	id := created["id"].(string)

	updated := parseJSON(t, doRequest("PUT", "/v1/strategies/"+id, map[string]any{
		"name": "Consultiva — saúde", "icp": "clínicas de imagem",
	}, token))

	assert.EqualValues(t, 2, updated["version"])
}

// Renaming must not orphan the numbers it earned — the same rule as email
// templates, for the same reason.
func TestStrategies_RenamingDoesNotBumpTheVersion(t *testing.T) {
	cleanDB(t)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")
	created := createStrategy(t, token, map[string]any{"icp": "hospitais"})
	id := created["id"].(string)

	updated := parseJSON(t, doRequest("PUT", "/v1/strategies/"+id, map[string]any{
		"name": "Outro nome", "icp": "hospitais",
	}, token))

	assert.EqualValues(t, 1, updated["version"])
	assert.Equal(t, "Outro nome", updated["name"])
}

// Promoting one must not bump the version either, or the promotion would
// orphan the numbers that justified it.
func TestStrategies_PromotingKeepsTheVersion(t *testing.T) {
	cleanDB(t)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")
	created := createStrategy(t, token, map[string]any{"icp": "hospitais"})
	id := created["id"].(string)

	require.Equal(t, http.StatusNoContent,
		doRequest("PUT", "/v1/strategies/"+id+"/status",
			map[string]any{"status": "promoted"}, token).Code)

	got := parseJSON(t, doRequest("GET", "/v1/strategies/"+id, nil, token))
	assert.Equal(t, "promoted", got["status"])
	assert.EqualValues(t, 1, got["version"])
}

// The status reaches a CHECK constraint, so an invented one is a 400 rather
// than a 500 from Postgres.
func TestStrategies_RefuseAnUnknownStatus(t *testing.T) {
	cleanDB(t)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")
	id := createStrategy(t, token, map[string]any{})["id"].(string)

	w := doRequest("PUT", "/v1/strategies/"+id+"/status",
		map[string]any{"status": "inventada"}, token)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Deciding the whole team's method is an admin call; writing one is not —
// gating authorship behind an admin is how a strategy bank ends up containing
// only what management wrote.
func TestStrategies_AttendantsWriteButDoNotPromote(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	attendant := tokenForSeededAccount(t, accountID, "attendant")

	created := createStrategy(t, attendant, map[string]any{"icp": "clínicas"})

	w := doRequest("PUT", "/v1/strategies/"+created["id"].(string)+"/status",
		map[string]any{"status": "promoted"}, attendant)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ── E3: attribution ───────────────────────────────────────────────────────

// The numbers are grouped by (strategy, VERSION): an approach sent under
// version 2 must not count toward version 3's.
func TestStrategies_StatsAreCountedPerVersion(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	strategyID := createStrategy(t, token, map[string]any{"icp": "hospitais"})["id"].(string)

	// Two leads under version 1 — one of which replied — and one under
	// version 2.
	seedAttributedLead(t, accountID, strategyID, 1, "responded")
	seedAttributedLead(t, accountID, strategyID, 1, "contacted")
	seedAttributedLead(t, accountID, strategyID, 2, "won")

	detail := parseJSON(t, doRequest("GET", "/v1/strategies/"+strategyID, nil, token))
	stats, _ := detail["stats"].([]any)
	require.Len(t, stats, 2, "one row per version")

	v1 := stats[0].(map[string]any)
	assert.EqualValues(t, 1, v1["version"])
	assert.EqualValues(t, 2, v1["approaches"])
	assert.EqualValues(t, 1, v1["replied"])
	assert.EqualValues(t, 0, v1["won"])

	v2 := stats[1].(map[string]any)
	assert.EqualValues(t, 2, v2["version"])
	assert.EqualValues(t, 1, v2["won"])
}

// A lead worked without a strategy is an honest state. Defaulting it to some
// arbitrary strategy would make the first measurement a lie.
func TestStrategies_UnattributedLeadsCountTowardNothing(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	strategyID := createStrategy(t, token, map[string]any{"icp": "hospitais"})["id"].(string)
	seedLeadForSequence(t, accountID) // no strategy stamp

	detail := parseJSON(t, doRequest("GET", "/v1/strategies/"+strategyID, nil, token))

	stats, _ := detail["stats"].([]any)
	assert.Empty(t, stats)
}

func seedAttributedLead(t *testing.T, accountID, strategyID string, version int, status string) {
	t.Helper()
	ctx := context.Background()

	funnelID := ulid.Make().String()
	_, err := testPool.Exec(ctx,
		`INSERT INTO sales_funnels (id, account_id, name, created_at, updated_at)
		 VALUES ($1, $2, 'Vendas', NOW(), NOW())`, funnelID, accountID)
	require.NoError(t, err)

	contactID := ulid.Make().String()
	_, err = testPool.Exec(ctx,
		`INSERT INTO contacts (id, account_id, external_id, name, created_at, updated_at)
		 VALUES ($1, $2, $3, 'Contato', NOW(), NOW())`,
		// The TAIL of the ULID, not the head: the first characters are a
		// timestamp, so three contacts seeded in the same millisecond would
		// collide on the unique (account, external_id) index.
		contactID, accountID, "+55"+ulid.Make().String()[15:])
	require.NoError(t, err)

	_, err = testPool.Exec(ctx,
		`INSERT INTO leads (id, account_id, funnel_id, contact_id, status,
		    strategy_id, strategy_version, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`,
		ulid.Make().String(), accountID, funnelID, contactID, status,
		strategyID, version)
	require.NoError(t, err)
}
