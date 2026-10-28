package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// The one-click route hands a credential that can redirect a company's mail
// to an outbound call. These tests are about that boundary.

type spyProvisioner struct {
	name       string
	credential string
	calls      int
	report     domain.DNSProvisionReport
	err        error
}

func (s *spyProvisioner) Name() string { return s.name }

func (s *spyProvisioner) Apply(_ context.Context, credential string, _ domain.EmailDNSSetup) (domain.DNSProvisionReport, error) {
	s.calls++
	s.credential = credential
	return s.report, s.err
}

func setupApplyRouter(t *testing.T, channelRepo *MockChannelRepo, prov domain.DNSProvisioner) *gin.Engine {
	t.Helper()
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	handler, router := setupChannelTestRouter(channelRepo, new(MockAgentRepo), jwtMock)
	handler.SetEmailDNSVerifier(services.NewEmailDNSVerifier(nil, domain.EmailSendingConfig{
		SPFInclude:   "_spf.replider.com",
		DKIMSelector: "rpl",
		DKIMTarget:   "dkim.replider.com",
	}))
	if prov != nil {
		handler.SetDNSProvisioners(prov)
	}
	router.POST("/v1/channels/:id/email-dns/apply",
		setupAuthMiddleware(jwtMock), middleware.RequireRole("admin"), handler.HandleApplyEmailDNS)
	return router
}

func postApply(router *gin.Engine, body map[string]any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost,
		"/v1/channels/"+testEmailChannelID+"/email-dns/apply", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func okReport() domain.DNSProvisionReport {
	return domain.DNSProvisionReport{
		Provider: "cloudflare", Zone: "clinica.com.br",
		Results: []domain.DNSProvisionResult{
			{Kind: domain.DNSKindSPF, Action: domain.DNSActionMerged},
		},
	}
}

func TestApplyDNS_PublishesAndReportsPerRecord(t *testing.T) {
	repo := new(MockChannelRepo)
	repo.On("GetByID", mock.Anything, testEmailChannelID).
		Return(emailChannel(map[string]interface{}{"from_address": "comercial@clinica.com.br"}), nil)
	prov := &spyProvisioner{name: "cloudflare", report: okReport()}

	w := postApply(setupApplyRouter(t, repo, prov),
		map[string]any{"provider": "cloudflare", "api_token": "cf-token"})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "cf-token", prov.credential)

	var got domain.DNSProvisionReport
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "clinica.com.br", got.Zone)
	// The interesting outcome is "merged", not "done": it means we found the
	// customer's own SPF and did not destroy it.
	assert.Equal(t, domain.DNSActionMerged, got.Results[0].Action)
}

// The token is read from the body and dropped when the request ends. Nothing
// is echoed back — a credential that can redirect a company's mail must not
// end up in a response body a browser will cache or a log will capture.
func TestApplyDNS_NeverEchoesTheToken(t *testing.T) {
	repo := new(MockChannelRepo)
	repo.On("GetByID", mock.Anything, testEmailChannelID).
		Return(emailChannel(map[string]interface{}{"from_address": "comercial@clinica.com.br"}), nil)

	w := postApply(setupApplyRouter(t, repo, &spyProvisioner{name: "cloudflare", report: okReport()}),
		map[string]any{"provider": "cloudflare", "api_token": "segredo-do-cliente"})

	assert.NotContains(t, w.Body.String(), "segredo-do-cliente")
}

// Another account's channel is not reachable, and the failure must not tell
// the caller whether the channel exists.
func TestApplyDNS_RefusesAnotherAccountsChannel(t *testing.T) {
	ch := emailChannel(map[string]interface{}{"from_address": "comercial@clinica.com.br"})
	ch.AccountID = "outra-conta"
	repo := new(MockChannelRepo)
	repo.On("GetByID", mock.Anything, testEmailChannelID).Return(ch, nil)
	prov := &spyProvisioner{name: "cloudflare", report: okReport()}

	w := postApply(setupApplyRouter(t, repo, prov),
		map[string]any{"provider": "cloudflare", "api_token": "cf-token"})

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Zero(t, prov.calls, "the token must never reach a provider for a foreign channel")
}

func TestApplyDNS_UnknownProviderIsRefusedBeforeAnyCall(t *testing.T) {
	repo := new(MockChannelRepo)
	repo.On("GetByID", mock.Anything, testEmailChannelID).
		Return(emailChannel(map[string]interface{}{"from_address": "comercial@clinica.com.br"}), nil)
	prov := &spyProvisioner{name: "cloudflare", report: okReport()}

	w := postApply(setupApplyRouter(t, repo, prov),
		map[string]any{"provider": "registrobr", "api_token": "x"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "DNS_PROVIDER_UNSUPPORTED")
	assert.Zero(t, prov.calls)
}

// Nothing wired is the normal state for a deployment with no provisioner —
// the manual instructions still work, so this must not 500.
func TestApplyDNS_NoProvisionerWiredIsAClearRefusal(t *testing.T) {
	repo := new(MockChannelRepo)
	repo.On("GetByID", mock.Anything, testEmailChannelID).
		Return(emailChannel(map[string]interface{}{"from_address": "comercial@clinica.com.br"}), nil)

	w := postApply(setupApplyRouter(t, repo, nil),
		map[string]any{"provider": "cloudflare", "api_token": "x"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "DNS_PROVIDER_UNSUPPORTED")
}

func TestApplyDNS_RequiresAToken(t *testing.T) {
	repo := new(MockChannelRepo)
	repo.On("GetByID", mock.Anything, testEmailChannelID).
		Return(emailChannel(map[string]interface{}{"from_address": "comercial@clinica.com.br"}), nil).Maybe()
	prov := &spyProvisioner{name: "cloudflare", report: okReport()}

	w := postApply(setupApplyRouter(t, repo, prov),
		map[string]any{"provider": "cloudflare"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Zero(t, prov.calls)
}

// A rejected token is one clear message: "wrong token" and "token without
// the permission" are fixed the same way.
func TestApplyDNS_SurfacesARejectedToken(t *testing.T) {
	repo := new(MockChannelRepo)
	repo.On("GetByID", mock.Anything, testEmailChannelID).
		Return(emailChannel(map[string]interface{}{"from_address": "comercial@clinica.com.br"}), nil)
	prov := &spyProvisioner{name: "cloudflare", err: domain.ErrDNSCredentialInvalid}

	w := postApply(setupApplyRouter(t, repo, prov),
		map[string]any{"provider": "cloudflare", "api_token": "ruim"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "DNS_CREDENTIAL_INVALID")
}
