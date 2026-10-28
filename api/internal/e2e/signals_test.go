package e2e

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/http/handlers"
	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Signals against a real database. The two things worth proving here are the
// dedup key (a republished tender must update, not duplicate) and the expiry
// filter running in SQL.

func seedSignal(t *testing.T, accountID string, mut func(*domain.Signal)) *domain.Signal {
	t.Helper()
	closes := time.Now().Add(72 * time.Hour)
	s := &domain.Signal{
		ID: ulid.Make().String(), AccountID: accountID,
		Source: domain.SignalPNCP, ExternalID: "pncp-" + ulid.Make().String(),
		Organization: "Hospital Municipal", Title: "Monitores multiparâmetro",
		State: "SP", ValueCents: 45000000,
		PublishedAt: time.Now().Add(-24 * time.Hour), ClosesAt: &closes,
	}
	if mut != nil {
		mut(s)
	}
	require.NoError(t, postgres.NewSignalRepository(testPool).
		Upsert(context.Background(), s))
	// A signal only belongs to an account while some rule of that account
	// still claims it, so seeding one without a watch produces a row the feed
	// deliberately hides — the orphan rule that cleared eighteen PVC-reducer
	// tenders off a hospital-equipment radar. Fixtures therefore seed the
	// claim too, which is also the shape production has.
	seedWatchFor(t, accountID, s.Source)
	return s
}

// seedWatchFor gives the account an active watch on this source, once.
func seedWatchFor(t *testing.T, accountID string, source domain.SignalSource) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), `
		INSERT INTO signal_watches (id, account_id, source, keywords, states, is_active, created_at, updated_at)
		SELECT $1, $2, $3, ARRAY['monitor'], ARRAY['SP'], TRUE, NOW(), NOW()
		 WHERE NOT EXISTS (
		     SELECT 1 FROM signal_watches
		      WHERE account_id = $2 AND source = $3
		 )`, ulid.Make().String(), accountID, string(source))
	require.NoError(t, err)
}

func TestSignals_FeedShowsOpenTenders(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	seedSignal(t, accountID, nil)

	list := parseJSONArray(t, doRequest("GET", "/v1/signals", nil, token))

	require.Len(t, list, 1)
	row := list[0].(map[string]any)
	assert.Equal(t, "Monitores multiparâmetro", row["title"])
	// Urgency is the whole value: three days is a reason to write today.
	assert.EqualValues(t, 2, row["days_left"])
}

// Approaching somebody about a bid they already closed proves nobody read it.
// The filter runs in SQL — applied in Go over a page it would show ten rows,
// hide eight, and leave the salesperson wondering where the feed went.
func TestSignals_ClosedTendersNeverAppear(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	past := time.Now().Add(-1 * time.Hour)
	seedSignal(t, accountID, func(s *domain.Signal) { s.ClosesAt = &past })

	list := parseJSONArray(t, doRequest("GET", "/v1/signals", nil, token))

	assert.Empty(t, list)
}

// A tender with no published deadline is still live: hiding it would be a
// worse error than showing an old one, whose date the salesperson can see.
func TestSignals_NoDeadlineStillShows(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	seedSignal(t, accountID, func(s *domain.Signal) { s.ClosesAt = nil })

	list := parseJSONArray(t, doRequest("GET", "/v1/signals", nil, token))

	require.Len(t, list, 1)
	assert.Nil(t, list[0].(map[string]any)["days_left"])
}

// PNCP republishes a tender as it is amended. A salesperson who sees the same
// notice three times stops reading them.
func TestSignals_RepublishedTenderUpdatesInsteadOfDuplicating(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	first := seedSignal(t, accountID, nil)

	amended := *first
	amended.ID = ulid.Make().String()
	amended.Title = "Monitores multiparâmetro — RETIFICADO"
	newClose := time.Now().Add(120 * time.Hour)
	amended.ClosesAt = &newClose
	require.NoError(t, postgres.NewSignalRepository(testPool).
		Upsert(context.Background(), &amended))

	list := parseJSONArray(t, doRequest("GET", "/v1/signals", nil, token))

	require.Len(t, list, 1, "one tender, not two")
	// The amended version is the one that matters: a deadline that moved is
	// the whole reason to read it again.
	assert.Equal(t, "Monitores multiparâmetro — RETIFICADO",
		list[0].(map[string]any)["title"])
}

// A human's decision outranks the source's redelivery: the next poll must not
// resurrect something somebody already said no to.
func TestSignals_DismissedStaysDismissedAcrossARepublish(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	s := seedSignal(t, accountID, nil)

	require.Equal(t, http.StatusNoContent,
		doRequest("POST", "/v1/signals/"+s.ID+"/dismiss", nil, token).Code)

	republished := *s
	republished.ID = ulid.Make().String()
	require.NoError(t, postgres.NewSignalRepository(testPool).
		Upsert(context.Background(), &republished))

	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/signals", nil, token)))
}

// One account's feed is not another's.
func TestSignals_AreScopedToTheAccount(t *testing.T) {
	cleanDB(t)
	owner := seedIdentityAccount(t)
	seedSignal(t, owner, nil)
	otherToken := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/signals", nil, otherToken)))
}

// ── Watches ───────────────────────────────────────────────────────────────

func TestSignalWatches_CreateListDelete(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	created := parseJSON(t, doRequest("PUT", "/v1/signal-watches/new", map[string]any{
		"source":          "pncp",
		"keywords":        []string{"monitor", "monitorização"},
		"states":          []string{"SP"},
		"min_value_cents": 5000000,
	}, token))
	id := created["id"].(string)
	require.NotEmpty(t, id)

	list := parseJSONArray(t, doRequest("GET", "/v1/signal-watches", nil, token))
	require.Len(t, list, 1)

	require.Equal(t, http.StatusNoContent,
		doRequest("DELETE", "/v1/signal-watches/"+id, nil, token).Code)
	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/signal-watches", nil, token)))
}

// The source reaches a TEXT column and drives a poller. An unknown one is a
// watch that would never fire, which is worse than a refusal.
func TestSignalWatches_RefuseAnUnknownSource(t *testing.T) {
	cleanDB(t)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	w := doRequest("PUT", "/v1/signal-watches/new", map[string]any{
		"source": "inventada", "keywords": []string{"x"},
	}, token)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── External ingestion (D2) ───────────────────────────────────────────────
//
// PNCP we read ourselves. The other useful Brazilian signals live behind
// paid vendors whose contracts differ per customer, so the seam is a signed
// push: any feed they already pay for lands in the same inbox.

func postSignalIngest(t *testing.T, accountID string, payload map[string]any, secret string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	// The feed signs with the ACCOUNT's derived key, never the global
	// secret (docs/webhooks.md, "Inbound signals").
	mac := hmac.New(sha256.New, []byte(handlers.AccountIngestSecret(secret, accountID)))
	mac.Write(body)

	req := httptest.NewRequest(http.MethodPost,
		"/v1/webhooks/signals/"+accountID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signal-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	return w
}

func newCompanyPayload() map[string]any {
	return map[string]any{
		"source":       "new_company",
		"external_id":  "cnpj-12345678000195",
		"organization": "Clínica Nova Vida LTDA",
		"cnpj":         "12.345.678/0001-95",
		"title":        "CNPJ aberto — atividade hospitalar",
		"state":        "sp",
	}
}

func TestSignalIngest_AcceptsASignedPush(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	w := postSignalIngest(t, accountID, newCompanyPayload(), "e2e-signal-secret")

	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	list := parseJSONArray(t, doRequest("GET", "/v1/signals", nil, token))
	require.Len(t, list, 1)
	row := list[0].(map[string]any)
	assert.Equal(t, "Clínica Nova Vida LTDA", row["organization"])
	// Digits only: the CNPJ is the join key to everything else we know about
	// this company, and a punctuated one joins to nothing.
	assert.Equal(t, "12345678000195", row["cnpj"])
}

// An open endpoint that inserts into a prospecting feed is a way to make a
// salesperson email whoever an attacker chooses.
func TestSignalIngest_RefusesAWrongSignature(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	w := postSignalIngest(t, accountID, newCompanyPayload(), "segredo-errado")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Empty(t, parseJSONArray(t, doRequest("GET", "/v1/signals", nil, token)))
}

func TestSignalIngest_RefusesAnUnsignedPush(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)

	body, _ := json.Marshal(newCompanyPayload())
	req := httptest.NewRequest(http.MethodPost,
		"/v1/webhooks/signals/"+accountID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// A feed that pushes the same company every hour would otherwise fill the
// inbox with one company.
func TestSignalIngest_DeduplicatesOnTheExternalID(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")

	require.Equal(t, http.StatusAccepted,
		postSignalIngest(t, accountID, newCompanyPayload(), "e2e-signal-secret").Code)
	require.Equal(t, http.StatusAccepted,
		postSignalIngest(t, accountID, newCompanyPayload(), "e2e-signal-secret").Code)

	assert.Len(t, parseJSONArray(t, doRequest("GET", "/v1/signals", nil, token)), 1)
}

// A signal with no id cannot be deduplicated and one with no title has
// nothing to match a watch against.
func TestSignalIngest_RefusesAnUnusablePayload(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)

	for _, payload := range []map[string]any{
		{"source": "new_company", "title": "sem id"},
		{"source": "new_company", "external_id": "x"},
		{"source": "inventada", "external_id": "x", "title": "y"},
	} {
		w := postSignalIngest(t, accountID, payload, "e2e-signal-secret")
		assert.Equal(t, http.StatusBadRequest, w.Code, "payload %v", payload)
	}
}

func TestSignals_UpsertReadsBackTheCommittedRow(t *testing.T) {
	// The auto-offer path decides from what Upsert returns: a republished
	// amendment must come back with the ORIGINAL row id and its lead link,
	// or the poller would mint a second lead for a tender somebody worked.
	cleanDB(t)
	ctx := context.Background()
	accountID := seedIdentityAccount(t)
	repo := postgres.NewSignalRepository(testPool)

	original := seedSignal(t, accountID, nil)
	require.NoError(t, repo.LinkLead(ctx, original.ID, accountID, "01LEADLEADLEADLEADLEADLEAD"))

	republished := &domain.Signal{
		ID: ulid.Make().String(), AccountID: accountID,
		Source: domain.SignalPNCP, ExternalID: original.ExternalID,
		Organization: original.Organization, Title: original.Title + " (retificado)",
		State: "SP", PublishedAt: time.Now(),
	}
	require.NoError(t, repo.Upsert(ctx, republished))

	assert.Equal(t, original.ID, republished.ID, "the committed id survives the republish")
	assert.Equal(t, "01LEADLEADLEADLEADLEADLEAD", republished.LeadID, "the lead link is visible to the caller")
}

// A watch with no UF used to mean "anywhere", and the poll's scope is the
// UNION of the watches in it — so one unscoped row dragged every other
// account's fetch across the whole country. Refused at the route, where the
// person can still answer.
func TestSignalWatches_RefuseAWatchWithNoState(t *testing.T) {
	cleanDB(t)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	w := doRequest("PUT", "/v1/signal-watches/new", map[string]any{
		"source": "pncp", "keywords": []string{"oxigênio"},
	}, token)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	// And a state that is not a state is the same refusal: "XX" passes any
	// shape test and then matches no tender ever.
	bad := doRequest("PUT", "/v1/signal-watches/new", map[string]any{
		"source": "pncp", "keywords": []string{"oxigênio"}, "states": []string{"XX"},
	}, token)

	assert.Equal(t, http.StatusBadRequest, bad.Code)
}

// ── Source health ─────────────────────────────────────────────────────────

// The radar could not tell an outage from a quiet week: it renders what is
// stored, so a portal refusing every request looked exactly like a portal
// with nothing new. These pin the two invariants that make the banner
// trustworthy — a failure never moves the success stamp, and a success
// clears the streak.
func TestSignalSourceHealth_AFailureNeverMovesTheSuccessStamp(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewSignalRepository(testPool)
	ctx := context.Background()
	good := time.Now().Add(-8 * time.Hour).UTC().Truncate(time.Second)

	require.NoError(t, repo.RecordAttempt(ctx, domain.SignalPNCP, good, nil))
	require.NoError(t, repo.RecordAttempt(ctx, domain.SignalPNCP, time.Now(),
		errors.New("pncp: status 429 after 3 retries")))
	require.NoError(t, repo.RecordAttempt(ctx, domain.SignalPNCP, time.Now(),
		errors.New("pncp: source unreachable")))

	list, err := repo.ListHealth(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	h := list[0]

	assert.True(t, h.Unreachable())
	assert.Equal(t, 2, h.ConsecutiveFailures, "the streak counts, so the screen can say how long")
	require.NotNil(t, h.LastSuccessAt)
	assert.WithinDuration(t, good, h.LastSuccessAt.UTC(), time.Second,
		"the success stamp is the only thing that can honestly date the feed")
	assert.Contains(t, h.LastError, "unreachable")
}

func TestSignalSourceHealth_ASuccessClearsTheStreak(t *testing.T) {
	cleanDB(t)
	repo := postgres.NewSignalRepository(testPool)
	ctx := context.Background()

	require.NoError(t, repo.RecordAttempt(ctx, domain.SignalPNCP, time.Now(), errors.New("502")))
	require.NoError(t, repo.RecordAttempt(ctx, domain.SignalPNCP, time.Now(), nil))

	list, err := repo.ListHealth(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)

	assert.False(t, list[0].Unreachable())
	assert.Zero(t, list[0].ConsecutiveFailures)
	assert.NotNil(t, list[0].LastSuccessAt)
}

// The route is what the screen reads. An unreported source must never render
// as an outage, so a fresh install draws no banner at all.
func TestSignalSources_RouteReportsWhatThePollFound(t *testing.T) {
	cleanDB(t)
	token := tokenForSeededAccount(t, seedIdentityAccount(t), "admin")

	assert.Empty(t, parseJSONArray(t,
		doRequest("GET", "/v1/signal-sources", nil, token)),
		"nothing polled yet is not an outage")

	require.NoError(t, postgres.NewSignalRepository(testPool).RecordAttempt(
		context.Background(), domain.SignalPNCP, time.Now(), errors.New("429")))

	list := parseJSONArray(t, doRequest("GET", "/v1/signal-sources", nil, token))
	require.Len(t, list, 1)
	row, ok := list[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "pncp", row["source"])
	assert.Equal(t, false, row["reachable"])
	// The source's own words stay server-side: a seller needs to know the
	// feed is stale, not that page 20 of modality 8 returned 429.
	assert.NotContains(t, row, "last_error")
}

// A polled signal is mine only while some rule of mine still claims it — and a
// PUSHED one is mine because somebody sent it.
//
// This guard exists because the rule shipped BROKEN. The edit that added it was
// written in one pass with a second edit that failed, so the file was never
// written, and what landed in production was the half that qualified the ORDER
// BY against a table alias that no longer existed. `/v1/signals` answered 500.
// Nobody noticed for a day because nobody opened the Radar.
//
// So the first assertion is not "does the filter filter" — it is "does this
// endpoint still answer at all". A query that cannot run is the failure this
// actually had.
func TestSignals_AnOrphanedSignalLeavesTheRadar(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	repo := postgres.NewSignalRepository(testPool)

	// Orphaned: a POLLED tender whose watch is gone. Exactly what migration
	// 000234 left behind when it deleted the unscoped watch — eighteen
	// national hits about PVC reducers stayed on a hospital-equipment radar.
	require.NoError(t, repo.Upsert(context.Background(), &domain.Signal{
		ID: ulid.Make().String(), AccountID: accountID,
		Source: domain.SignalPNCP, ExternalID: "pncp-" + ulid.Make().String(),
		Organization: "Prefeitura", Title: "Luva de redução em PVC",
		State: "SP", PublishedAt: time.Now().Add(-24 * time.Hour),
	}))

	// Pushed by an integration the account pays for. It has no watch and never
	// will: the sending IS the claim, and hiding these would empty every
	// vendor feed the product supports.
	require.NoError(t, repo.Upsert(context.Background(), &domain.Signal{
		ID: ulid.Make().String(), AccountID: accountID,
		Source: domain.SignalNewCompany, ExternalID: "cnpj-" + ulid.Make().String(),
		Organization: "Clínica Nova Vida LTDA", Title: "CNPJ aberto",
		State: "SP", PublishedAt: time.Now().Add(-24 * time.Hour),
	}))

	rec := doRequest("GET", "/v1/signals", nil, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	list := parseJSONArray(t, rec)
	require.Len(t, list, 1, "the orphaned tender goes, the pushed signal stays")
	row, ok := list[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Clínica Nova Vida LTDA", row["organization"])
	// Checked against the TITLES, never the raw body. A ULID is Crockford
	// base32 and eventually spells anything: CI failed this once on
	// `01M2GWDJNB10AD03PPVCD1R7MA`, where the filter had worked perfectly and
	// a random id happened to contain "PVC".
	for _, item := range list {
		r, ok := item.(map[string]any)
		require.True(t, ok)
		assert.NotContains(t, fmt.Sprint(r["title"]), "PVC",
			"a tender nothing of mine watches is not mine")
	}
}

// The stock, not the tap.
//
// The ingest gate has been right since it shipped; what it cannot do is reach
// rows already stored. On 15/09/2026 the account's feed held 62 signals of
// which 45 failed that gate, and the twenty the screen showed were a drone
// survey, braided safety netting, car servicing, room thermometers,
// "televisions and ceiling fans", and six budget entries that were not
// purchases at all. None got past the gate — they arrived while production ran
// a pre-gate image, because every automated API deploy had been broken since
// 01/09 and the gate reached production days after it merged.
//
// Filtering on READ is what makes that self-heal. This is an e2e and not a
// unit test because the rule rides a new EXISTS subquery and an extra scanned
// column: a fake repository would accept any SQL, and this file exists because
// hand-written SQL has broken in production three times before.
func TestSignals_DerivedWatchNoiseLeavesTheFeedOnRead(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	repo := postgres.NewSignalRepository(testPool)
	ctx := context.Background()

	// A watch the SYSTEM derived from the catalogue, carrying the ordinary
	// Portuguese words that produced the noise: `manual` matched "CONFECÇÃO
	// MANUAL SOB MEDIDA", `sensor` matched "sensor LiDAR".
	_, err := testPool.Exec(ctx, `
		INSERT INTO signal_watches (id, account_id, source, keywords, states,
		                            is_active, derived_from_catalog, created_at, updated_at)
		VALUES ($1, $2, 'pncp', ARRAY['manual','sensor','ventilador'], ARRAY['MT'],
		        TRUE, TRUE, NOW(), NOW())`,
		ulid.Make().String(), accountID)
	require.NoError(t, err)

	junk := []string{
		"[LICITANET] - aquisição de solução integrada de aerolevantamento, composta por aeronave " +
			"remotamente pilotada (RPA/DRONE), sensor LiDAR e software de processamento fotogramétrico",
		"FORNECIMENTO DE REDES DE PROTEÇÃO DE CORDA TRANÇADA, INCLUINDO REDES PRONTAS, CONFECÇÃO " +
			"MANUAL SOB MEDIDA E INSTALAÇÃO",
		"Aquisição de aparelhos de televisão e ventiladores de teto para as secretarias do município",
	}
	for _, title := range junk {
		require.NoError(t, repo.Upsert(ctx, &domain.Signal{
			ID: ulid.Make().String(), AccountID: accountID,
			Source: domain.SignalPNCP, ExternalID: "pncp-" + ulid.Make().String(),
			Organization: "MUNICIPIO DE SAPEZAL", Title: title,
			State: "MT", PublishedAt: time.Now().Add(-24 * time.Hour),
		}))
	}

	// And one that is genuinely this account's trade.
	require.NoError(t, repo.Upsert(ctx, &domain.Signal{
		ID: ulid.Make().String(), AccountID: accountID,
		Source: domain.SignalPNCP, ExternalID: "pncp-" + ulid.Make().String(),
		Organization: "INSTITUTO PATRIS",
		Title: "Contratação de empresa especializada para a locação de equipamentos médico-hospitalares " +
			"(berço aquecido neonatal, incubadora neonatal de transporte e ventilador pulmonar)",
		State: "MT", PublishedAt: time.Now().Add(-24 * time.Hour),
	}))

	rec := doRequest("GET", "/v1/signals", nil, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	list := parseJSONArray(t, rec)
	require.Len(t, list, 1, "the noise is still in the feed")
	row, ok := list[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "INSTITUTO PATRIS", row["organization"])

	// Filtered, NOT deleted and NOT dismissed. The rows are still there: a
	// dismissal is a human's decision and a delete is irreversible, while the
	// gate has changed three times in a week and rejects genuine purchases too
	// ("Luva 6,5 e Dispositivo para infusão"). They must come back if it
	// learns better.
	var stored, dismissed int
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT count(*), count(dismissed_at) FROM signals WHERE account_id = $1`,
		accountID).Scan(&stored, &dismissed))
	assert.Equal(t, 4, stored, "a filtered row was destroyed")
	assert.Zero(t, dismissed, "the system dismissed on the seller's behalf")
}

// The same noise, under a watch somebody TYPED. Nothing is filtered: those
// words are a decision, and the product does not overrule it — at ingest or
// on read.
func TestSignals_AHandWrittenWatchIsNotSecondGuessedOnRead(t *testing.T) {
	cleanDB(t)
	accountID := seedIdentityAccount(t)
	token := tokenForSeededAccount(t, accountID, "admin")
	repo := postgres.NewSignalRepository(testPool)
	ctx := context.Background()

	_, err := testPool.Exec(ctx, `
		INSERT INTO signal_watches (id, account_id, source, keywords, states,
		                            is_active, derived_from_catalog, created_at, updated_at)
		VALUES ($1, $2, 'pncp', ARRAY['corda'], ARRAY['MT'], TRUE, FALSE, NOW(), NOW())`,
		ulid.Make().String(), accountID)
	require.NoError(t, err)

	require.NoError(t, repo.Upsert(ctx, &domain.Signal{
		ID: ulid.Make().String(), AccountID: accountID,
		Source: domain.SignalPNCP, ExternalID: "pncp-" + ulid.Make().String(),
		Organization: "MUNICIPIO DE SAPEZAL",
		Title:        "FORNECIMENTO DE REDES DE PROTEÇÃO DE CORDA TRANÇADA",
		State:        "MT", PublishedAt: time.Now().Add(-24 * time.Hour),
	}))

	rec := doRequest("GET", "/v1/signals", nil, token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Len(t, parseJSONArray(t, rec), 1,
		"a watch somebody typed had its own words overruled")
}
