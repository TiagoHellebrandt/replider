package e2e

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The two endpoints a mail client hits. Unauthenticated by nature — the
// caller is Gmail fetching an image, or a customer clicking a link in their
// inbox — so the signed token is the whole security model.

const e2eTrackSecret = "e2e-test-jwt-secret"

func getPublic(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func seedTemplate(t *testing.T, accountID string) string {
	t.Helper()
	id := ulid.Make().String()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO email_templates (id, account_id, name, subject, body, version)
		 VALUES ($1,$2,'Teste','Assunto','Corpo',1)`, id, accountID)
	require.NoError(t, err)
	return id
}

func statsFor(t *testing.T, templateID string) domain.EmailTemplateStats {
	t.Helper()
	var s domain.EmailTemplateStats
	err := testPool.QueryRow(context.Background(),
		`SELECT sent, opened, clicked, replied, bounced
		   FROM email_template_stats WHERE template_id = $1 AND version = 1`,
		templateID).Scan(&s.Sent, &s.Opened, &s.Clicked, &s.Replied, &s.Bounced)
	if err != nil {
		return domain.EmailTemplateStats{}
	}
	return s
}

func TestTracking_OpenCountsAndAlwaysReturnsThePixel(t *testing.T) {
	cleanDB(t)
	tplID := seedTemplate(t, seedIdentityAccount(t))

	tok := domain.MintTrackToken(e2eTrackSecret, domain.TrackToken{
		Kind: domain.TrackOpen, MessageID: "msg1", TemplateID: tplID, Version: 1,
	})
	w := getPublic(t, "/v1/t/o/"+tok)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/gif", w.Header().Get("Content-Type"))
	// A cached pixel would count one open for a message somebody opened five
	// times, and the cache in question is Gmail's.
	assert.Contains(t, w.Header().Get("Cache-Control"), "no-store")
	assert.Equal(t, 1, statsFor(t, tplID).Opened)
}

// A broken image in a stranger's cold email is a worse outcome than an
// uncounted open — and a 404 would also tell a prober they hit something real.
func TestTracking_AnInvalidOpenTokenStillReturnsThePixel(t *testing.T) {
	cleanDB(t)

	w := getPublic(t, "/v1/t/o/lixo")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/gif", w.Header().Get("Content-Type"))
}

func TestTracking_ClickCountsAndRedirects(t *testing.T) {
	cleanDB(t)
	tplID := seedTemplate(t, seedIdentityAccount(t))

	const dest = "https://clinica.com.br/proposta?id=42"
	tok := domain.MintTrackToken(e2eTrackSecret, domain.TrackToken{
		Kind: domain.TrackClick, MessageID: "msg1", TemplateID: tplID,
		Version: 1, URL: dest,
	})
	w := getPublic(t, "/v1/t/c/"+tok)

	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, dest, w.Header().Get("Location"))
	// The destination must not learn which tracking URL sent the visitor:
	// that URL identifies the message and therefore the recipient.
	assert.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, 1, statsFor(t, tplID).Clicked)
}

// The attack this whole design exists to prevent: a link on our domain
// pointing at someone else's page, carrying whatever trust our domain has.
func TestTracking_RefusesAForgedDestination(t *testing.T) {
	cleanDB(t)
	tplID := seedTemplate(t, seedIdentityAccount(t))

	forged := domain.MintTrackToken("outro-segredo", domain.TrackToken{
		Kind: domain.TrackClick, MessageID: "msg1", TemplateID: tplID,
		Version: 1, URL: "https://phishing.example/roubo",
	})
	w := getPublic(t, "/v1/t/c/"+forged)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Empty(t, w.Header().Get("Location"))
	assert.Zero(t, statsFor(t, tplID).Clicked)
}

// Even correctly signed, a non-http destination never redirects: the
// signature proves nobody tampered with it, not that we were right to sign it.
func TestTracking_RefusesANonHTTPDestination(t *testing.T) {
	cleanDB(t)

	tok := domain.MintTrackToken(e2eTrackSecret, domain.TrackToken{
		Kind: domain.TrackClick, MessageID: "m", Version: 1,
		URL: "javascript:steal()",
	})
	w := getPublic(t, "/v1/t/c/"+tok)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// An open token must not work on the click endpoint: it carries no
// destination, and accepting it would mean deciding where to send someone.
func TestTracking_TokenKindsDoNotCross(t *testing.T) {
	cleanDB(t)

	openTok := domain.MintTrackToken(e2eTrackSecret, domain.TrackToken{
		Kind: domain.TrackOpen, MessageID: "m", Version: 1,
	})

	assert.Equal(t, http.StatusNotFound, getPublic(t, "/v1/t/c/"+openTok).Code)
}
