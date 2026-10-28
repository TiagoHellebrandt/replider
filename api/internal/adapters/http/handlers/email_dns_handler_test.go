package handlers

import (
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

// US-M006/US-M022. The setup screen is the only place a customer can see why
// their mail is silently disappearing, so the endpoint has to be readable
// (per-record status) and honest about the boundaries: another tenant's
// channel does not exist, a non-email channel is not an email channel, and a
// channel with no from_address has nothing to verify.

// stubResolver answers from a fixed map; anything unlisted resolves as
// missing, which is how a record that was never published looks.
type stubResolver struct {
	txt   map[string][]string
	cname map[string]string
	ns    []string
}

func (s stubResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	if v, ok := s.txt[name]; ok {
		return v, nil
	}
	return nil, assertNotFound
}

func (s stubResolver) LookupCNAME(_ context.Context, name string) (string, error) {
	if v, ok := s.cname[name]; ok {
		return v, nil
	}
	return "", assertNotFound
}

func (s stubResolver) LookupNS(_ context.Context, _ string) ([]string, error) {
	return s.ns, nil
}

const testEmailChannelID = "chan-email-1"

var assertNotFound = &net404{}

type net404 struct{}

func (*net404) Error() string { return "no such host" }

func emailChannel(cfg map[string]interface{}) *domain.Channel {
	return &domain.Channel{
		ID:        testEmailChannelID,
		AccountID: testAccountID,
		Type:      "email",
		Name:      "E-mail",
		Config:    cfg,
	}
}

func setupEmailDNSRouter(
	t *testing.T,
	channelRepo *MockChannelRepo,
	resolver domain.DNSResolver,
) *gin.Engine {
	t.Helper()
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	handler, router := setupChannelTestRouter(channelRepo, new(MockAgentRepo), jwtMock)
	if resolver != nil {
		handler.SetEmailDNSVerifier(services.NewEmailDNSVerifier(resolver, domain.EmailSendingConfig{
			SPFInclude:    "_spf.replider.com",
			DKIMSelector:  "rpl",
			DKIMTarget:    "dkim.replider.com",
			DMARCReportTo: "dmarc@replider.com",
		}))
	}
	router.GET("/v1/channels/:id/email-dns",
		setupAuthMiddleware(jwtMock), middleware.RequireRole("admin"), handler.HandleEmailDNS)
	return router
}

func getEmailDNS(router *gin.Engine, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/v1/channels/"+testEmailChannelID+"/email-dns"+query, nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestEmailDNS_ReportsPerRecordStatus(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	channelRepo.On("GetByID", mock.Anything, testEmailChannelID).
		Return(emailChannel(map[string]interface{}{"from_address": "contato@clinica.com.br"}), nil)

	// BYO SMTP posture: AN SPF exists (whatever provider it authorizes) so
	// that row is OK; DMARC was never published, so it reads pending.
	router := setupEmailDNSRouter(t, channelRepo, stubResolver{
		txt: map[string][]string{
			"clinica.com.br": {"v=spf1 include:_spf.locaweb.com.br ~all"},
		},
	})

	w := getEmailDNS(router, "")
	require.Equal(t, http.StatusOK, w.Code)

	var setup domain.EmailDNSSetup
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &setup))
	assert.Equal(t, "clinica.com.br", setup.Domain)
	require.NotEmpty(t, setup.Records)

	byKind := map[domain.DNSRecordKind]domain.EmailDNSRecord{}
	for _, r := range setup.Records {
		byKind[r.Kind] = r
	}
	assert.Equal(t, domain.DNSStatusOK, byKind[domain.DNSKindSPF].Status,
		"an SMTP channel's SPF is presence-checked — we cannot know their provider's include")
	assert.Equal(t, domain.DNSStatusPending, byKind[domain.DNSKindDMARC].Status,
		"a record that was never published reads as pending, not as an error")
	// Not every required record is live, so the send caps must stay closed.
	assert.False(t, setup.Verified)
}

func TestEmailDNS_VerifiedOnlyWhenEveryRequiredRecordIsLive(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	channelRepo.On("GetByID", mock.Anything, testEmailChannelID).
		Return(emailChannel(map[string]interface{}{"from_address": "contato@clinica.com.br"}), nil)

	router := setupEmailDNSRouter(t, channelRepo, stubResolver{
		txt: map[string][]string{
			"clinica.com.br":        {"v=spf1 include:_spf.locaweb.com.br ~all"},
			"_dmarc.clinica.com.br": {"v=DMARC1; p=none; rua=mailto:dmarc@replider.com"},
		},
	})

	var setup domain.EmailDNSSetup
	w := getEmailDNS(router, "")
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &setup))
	assert.True(t, setup.Verified)
}

// A Gmail-OAuth channel on a Workspace domain must carry GOOGLE's include —
// an SPF that authorizes some other provider is a mismatch, not a pass.
func TestEmailDNS_GoogleMailboxDemandsGoogleSPF(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	channelRepo.On("GetByID", mock.Anything, testEmailChannelID).
		Return(emailChannel(map[string]interface{}{
			"from_address": "contato@clinica.com.br",
			"auth_method":  "google_oauth",
		}), nil)

	router := setupEmailDNSRouter(t, channelRepo, stubResolver{
		txt: map[string][]string{
			"clinica.com.br":        {"v=spf1 include:_spf.locaweb.com.br ~all"},
			"_dmarc.clinica.com.br": {"v=DMARC1; p=none"},
		},
	})

	var setup domain.EmailDNSSetup
	w := getEmailDNS(router, "")
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &setup))

	byKind := map[domain.DNSRecordKind]domain.EmailDNSRecord{}
	for _, r := range setup.Records {
		byKind[r.Kind] = r
	}
	assert.Equal(t, domain.DNSStatusMismatch, byKind[domain.DNSKindSPF].Status)
	assert.Contains(t, byKind[domain.DNSKindSPF].Value, "_spf.google.com")
	assert.False(t, setup.Verified)
}

// A personal @gmail.com has NOTHING to publish — Google owns and
// authenticates gmail.com. The screen used to demand DNS edits on a zone the
// customer could never touch (the incoherent eternal warning).
func TestEmailDNS_PersonalGmailIsProviderManaged(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	channelRepo.On("GetByID", mock.Anything, testEmailChannelID).
		Return(emailChannel(map[string]interface{}{
			"from_address": "clinica.cba@gmail.com",
			"auth_method":  "google_oauth",
		}), nil)

	router := setupEmailDNSRouter(t, channelRepo, stubResolver{})

	var setup domain.EmailDNSSetup
	w := getEmailDNS(router, "")
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &setup))
	assert.True(t, setup.ProviderManaged)
	assert.True(t, setup.Verified)
	assert.Empty(t, setup.Records)
}

// The screen renders instructions before it can afford a DNS round trip.
func TestEmailDNS_VerifyFalseSkipsTheNetwork(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	channelRepo.On("GetByID", mock.Anything, testEmailChannelID).
		Return(emailChannel(map[string]interface{}{"from_address": "contato@clinica.com.br"}), nil)

	// A resolver that would answer everything: if it were consulted, the
	// records would come back OK instead of pending.
	router := setupEmailDNSRouter(t, channelRepo, stubResolver{
		txt:   map[string][]string{"clinica.com.br": {"v=spf1 include:_spf.replider.com ~all"}},
		cname: map[string]string{"rpl._domainkey.clinica.com.br": "dkim.replider.com."},
	})

	var setup domain.EmailDNSSetup
	w := getEmailDNS(router, "?verify=false")
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &setup))
	require.NotEmpty(t, setup.Records)
	for _, r := range setup.Records {
		// The resolver would have answered OK for SPF and DKIM; everything
		// still reads pending, so nothing was looked up.
		assert.Equal(t, domain.DNSStatusPending, r.Status, r.Kind)
	}
	assert.False(t, setup.Verified)
}

func TestEmailDNS_RejectsOtherTenantsChannel(t *testing.T) {
	ch := emailChannel(map[string]interface{}{"from_address": "contato@clinica.com.br"})
	ch.AccountID = "another-account"
	channelRepo := new(MockChannelRepo)
	channelRepo.On("GetByID", mock.Anything, testEmailChannelID).Return(ch, nil)

	w := getEmailDNS(setupEmailDNSRouter(t, channelRepo, stubResolver{}), "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestEmailDNS_RejectsNonEmailChannel(t *testing.T) {
	ch := emailChannel(nil)
	ch.Type = "whatsapp"
	channelRepo := new(MockChannelRepo)
	channelRepo.On("GetByID", mock.Anything, testEmailChannelID).Return(ch, nil)

	w := getEmailDNS(setupEmailDNSRouter(t, channelRepo, stubResolver{}), "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEmailDNS_MissingFromAddressIsAConfigError(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	channelRepo.On("GetByID", mock.Anything, testEmailChannelID).
		Return(emailChannel(map[string]interface{}{"smtp_host": "smtp.example.com"}), nil)

	w := getEmailDNS(setupEmailDNSRouter(t, channelRepo, stubResolver{}), "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Unwired verifier: say so, don't report an unverified domain.
func TestEmailDNS_UnconfiguredIsNotAFailedVerification(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	channelRepo.On("GetByID", mock.Anything, testEmailChannelID).
		Return(emailChannel(map[string]interface{}{"from_address": "contato@clinica.com.br"}), nil)

	w := getEmailDNS(setupEmailDNSRouter(t, channelRepo, nil), "")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
