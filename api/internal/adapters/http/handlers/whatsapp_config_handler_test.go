package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	waadapter "github.com/replider/api/internal/adapters/channels/whatsapp"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// setupWAConfigRouter wires the WhatsApp config handler routes for tests.
func setupWAConfigRouter(channelRepo *MockChannelRepo, jwtMock *MockJWTService) *gin.Engine {
	// In-process Meta response cache is package-global; without resetting,
	// a prior test's success leaks into the next test that expects a
	// fresh upstream call (different httptest server, same channel ID).
	metaCacheResetForTesting()
	h := NewWhatsAppConfigHandler(channelRepo, "test-app-id")
	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	g := router.Group("/v1/channels", authMw, middleware.RequireRole("admin"))
	{
		g.GET("/:id/whatsapp/profile", h.HandleGetBusinessProfile)
		g.PUT("/:id/whatsapp/profile", h.HandleUpdateBusinessProfile)
		g.POST("/:id/whatsapp/profile/picture", h.HandleUploadProfilePicture)
		g.DELETE("/:id/whatsapp/profile/picture", h.HandleDeleteProfilePicture)
		g.POST("/:id/templates", h.HandleCreateTemplate)
		g.DELETE("/:id/templates/:name", h.HandleDeleteTemplate)
	}
	return router
}

// waChannel builds a valid WhatsApp channel with populated config JSON.
func waChannel(id, accountID string) *domain.Channel {
	cfg, _ := json.Marshal(map[string]string{
		"phone_number_id": "pn-123",
		"waba_id":         "waba-456",
		"access_token":    "EAAsometoken",
	})
	return &domain.Channel{
		ID:        id,
		AccountID: accountID,
		Type:      "whatsapp",
		Name:      "WA",
		Config:    json.RawMessage(cfg),
		IsActive:  true,
	}
}

// ──────────────────────────────────────────────────────────────────
// SECURITY: cross-tenant access — account B cannot touch account A's channel
// ──────────────────────────────────────────────────────────────────

func TestWAConfig_GetProfile_CrossTenant_404(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	// Channel belongs to a different account
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", "other-acc"), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/channels/ch-1/whatsapp/profile", nil, testToken)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "CHANNEL_NOT_FOUND", body["code"])
	channelRepo.AssertExpectations(t)
}

func TestWAConfig_UpdateProfile_CrossTenant_404(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", "other-acc"), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodPut, "/v1/channels/ch-1/whatsapp/profile",
		map[string]interface{}{"about": "hi"}, testToken)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWAConfig_CreateTemplate_CrossTenant_404(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", "other-acc"), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/channels/ch-1/templates", map[string]interface{}{
		"name":       "hello",
		"category":   "UTILITY",
		"language":   "pt_BR",
		"components": []map[string]interface{}{{"type": "BODY", "text": "oi"}},
	}, testToken)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWAConfig_DeleteTemplate_CrossTenant_404(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", "other-acc"), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodDelete, "/v1/channels/ch-1/templates/hello", nil, testToken)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ──────────────────────────────────────────────────────────────────
// SECURITY: wrong channel type rejected
// ──────────────────────────────────────────────────────────────────

func TestWAConfig_GetProfile_NonWhatsAppChannel_400(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	// Telegram channel
	cfg, _ := json.Marshal(map[string]string{"bot_token": "tg"})
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(&domain.Channel{
		ID:        "ch-1",
		AccountID: testAccountID,
		Type:      "telegram",
		Config:    json.RawMessage(cfg),
	}, nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/channels/ch-1/whatsapp/profile", nil, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "INVALID_CHANNEL_TYPE", body["code"])
}

// ──────────────────────────────────────────────────────────────────
// SECURITY: missing config rejected
// ──────────────────────────────────────────────────────────────────

func TestWAConfig_GetProfile_MissingConfig_400(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	// WhatsApp channel with empty config
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(&domain.Channel{
		ID:        "ch-1",
		AccountID: testAccountID,
		Type:      "whatsapp",
		Config:    json.RawMessage(`{}`),
	}, nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/channels/ch-1/whatsapp/profile", nil, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "CHANNEL_MISSING_CONFIG", body["code"])
}

// ──────────────────────────────────────────────────────────────────
// SECURITY: unauthenticated request blocked
// ──────────────────────────────────────────────────────────────────

func TestWAConfig_GetProfile_NoAuth_401(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	// No token
	w := performRequest(router, http.MethodGet, "/v1/channels/ch-1/whatsapp/profile", nil, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ──────────────────────────────────────────────────────────────────
// SECURITY: non-admin role blocked
// ──────────────────────────────────────────────────────────────────

func TestWAConfig_GetProfile_NonAdmin_403(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "attendant")

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/channels/ch-1/whatsapp/profile", nil, testToken)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ──────────────────────────────────────────────────────────────────
// SECURITY: template name validation (prevents injection via URL path)
// ──────────────────────────────────────────────────────────────────

func TestWAConfig_DeleteTemplate_InvalidName_400(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)

	// All encoded via url.PathEscape so httptest.NewRequest accepts them;
	// the handler must still reject them via the regex check.
	cases := []struct{ name, encoded string }{
		{"Upper", "Upper"},
		{"has space", "has%20space"},
		{"with-dash", "with-dash"},
		{"foo.bar", "foo.bar"},
		{"ampersand", "evil%26name%3Dattacker"},
	}
	for _, c := range cases {
		t.Run("name="+c.name, func(t *testing.T) {
			path := "/v1/channels/ch-1/templates/" + c.encoded
			w := performRequest(router, http.MethodDelete, path, nil, testToken)
			assert.Equal(t, http.StatusBadRequest, w.Code,
				"expected 400 for name=%q, got %d", c.name, w.Code)
			var body map[string]interface{}
			_ = json.Unmarshal(w.Body.Bytes(), &body)
			assert.Equal(t, "INVALID_TEMPLATE_NAME", body["code"])
		})
	}
}

func TestWAConfig_CreateTemplate_InvalidName_400(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/channels/ch-1/templates", map[string]interface{}{
		"name":       "Invalid-Name!",
		"category":   "UTILITY",
		"language":   "pt_BR",
		"components": []map[string]interface{}{{"type": "BODY", "text": "oi"}},
	}, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "INVALID_TEMPLATE_NAME", body["code"])
}

// ──────────────────────────────────────────────────────────────────
// SECURITY: profile picture file validation
// ──────────────────────────────────────────────────────────────────

func uploadMultipart(t *testing.T, router *gin.Engine, path, token string, fileBytes []byte, fileName, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	h := make(map[string][]string)
	h["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="file"; filename="%s"`, fileName)}
	h["Content-Type"] = []string{contentType}
	part, err := w.CreatePart(h)
	assert.NoError(t, err)
	_, _ = part.Write(fileBytes)
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestWAConfig_UploadPicture_FakeImage_Rejected(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	// Send a plain-text file with a lying Content-Type — magic-byte check must reject
	rec := uploadMultipart(t, router, "/v1/channels/ch-1/whatsapp/profile/picture", testToken,
		[]byte("not an image, just text"), "fake.jpg", "image/jpeg")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	assert.Equal(t, "INVALID_IMAGE_FORMAT", body["code"])
}

func TestWAConfig_UploadPicture_TooLarge_400(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	big := make([]byte, 6*1024*1024) // 6MB > 5MB limit
	big[0], big[1], big[2] = 0xFF, 0xD8, 0xFF
	rec := uploadMultipart(t, router, "/v1/channels/ch-1/whatsapp/profile/picture", testToken,
		big, "big.jpg", "image/jpeg")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	assert.Equal(t, "FILE_TOO_LARGE", body["code"])
}

func TestWAConfig_UploadPicture_MissingFile_400(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)

	// Multipart form without a "file" field
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("other", "value")
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/channels/ch-1/whatsapp/profile/picture", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	assert.Equal(t, "FILE_REQUIRED", body["code"])
}

// ──────────────────────────────────────────────────────────────────
// Bad JSON bodies are rejected
// ──────────────────────────────────────────────────────────────────

// ──────────────────────────────────────────────────────────────────
// Success paths (integration with Meta via httptest.Server)
// ──────────────────────────────────────────────────────────────────

func TestWAConfig_GetProfile_Success_ProxiesMeta(t *testing.T) {
	// Spin up a fake Meta that returns a profile payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"about":"Nossa loja","email":"e@e.com"}]}`))
	}))
	t.Cleanup(srv.Close)

	// Shadow the base URL used by the wrappers
	origURL := waadapter.GetBaseURLForTesting()
	waadapter.SetBaseURLForTesting(srv.URL)
	t.Cleanup(func() { waadapter.SetBaseURLForTesting(origURL) })

	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/channels/ch-1/whatsapp/profile", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "Nossa loja", body["about"])
	assert.Equal(t, "e@e.com", body["email"])
}

func TestWAConfig_GetProfile_MetaFailure_502(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	origURL := waadapter.GetBaseURLForTesting()
	waadapter.SetBaseURLForTesting(srv.URL)
	t.Cleanup(func() { waadapter.SetBaseURLForTesting(origURL) })

	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/channels/ch-1/whatsapp/profile", nil, testToken)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "WA_PROFILE_FETCH_FAILED", body["code"])
}

func TestWAConfig_UpdateProfile_Success_204(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	origURL := waadapter.GetBaseURLForTesting()
	waadapter.SetBaseURLForTesting(srv.URL)
	t.Cleanup(func() { waadapter.SetBaseURLForTesting(origURL) })

	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodPut, "/v1/channels/ch-1/whatsapp/profile",
		map[string]interface{}{"about": "new"}, testToken)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestWAConfig_UploadPicture_Success(t *testing.T) {
	step := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step++
		if step == 1 {
			_, _ = w.Write([]byte(`{"id":"up:1"}`))
		} else {
			_, _ = w.Write([]byte(`{"h":"H_PIC"}`))
		}
	}))
	t.Cleanup(srv.Close)
	origURL := waadapter.GetBaseURLForTesting()
	waadapter.SetBaseURLForTesting(srv.URL)
	t.Cleanup(func() { waadapter.SetBaseURLForTesting(origURL) })

	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	// Real JPEG magic bytes + some padding
	jpegBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	rec := uploadMultipart(t, router, "/v1/channels/ch-1/whatsapp/profile/picture", testToken,
		jpegBytes, "x.jpg", "image/jpeg")

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	assert.Equal(t, "H_PIC", body["handle"])
}

func TestWAConfig_UploadPicture_PNG_Accepted(t *testing.T) {
	step := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step++
		if step == 1 {
			_, _ = w.Write([]byte(`{"id":"up:2"}`))
		} else {
			// Verify MIME was set to image/png in step 1 URL (was seen last request in step 2 scope — skip)
			_, _ = w.Write([]byte(`{"h":"H_PNG"}`))
		}
	}))
	t.Cleanup(srv.Close)
	origURL := waadapter.GetBaseURLForTesting()
	waadapter.SetBaseURLForTesting(srv.URL)
	t.Cleanup(func() { waadapter.SetBaseURLForTesting(origURL) })

	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	pngBytes := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	rec := uploadMultipart(t, router, "/v1/channels/ch-1/whatsapp/profile/picture", testToken,
		pngBytes, "x.png", "image/png")

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestWAConfig_CreateTemplate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"tpl"}`))
	}))
	t.Cleanup(srv.Close)
	origURL := waadapter.GetBaseURLForTesting()
	waadapter.SetBaseURLForTesting(srv.URL)
	t.Cleanup(func() { waadapter.SetBaseURLForTesting(origURL) })

	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/channels/ch-1/templates", map[string]interface{}{
		"name":       "boas_vindas",
		"category":   "UTILITY",
		"language":   "pt_BR",
		"components": []map[string]interface{}{{"type": "BODY", "text": "oi"}},
	}, testToken)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestWAConfig_DeleteTemplate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	t.Cleanup(srv.Close)
	origURL := waadapter.GetBaseURLForTesting()
	waadapter.SetBaseURLForTesting(srv.URL)
	t.Cleanup(func() { waadapter.SetBaseURLForTesting(origURL) })

	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodDelete, "/v1/channels/ch-1/templates/boas_vindas", nil, testToken)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

// ──────────────────────────────────────────────────────────────────
// Missing waba_id / Meta-side failures coverage
// ──────────────────────────────────────────────────────────────────

func waChannelNoWABA(id, accountID string) *domain.Channel {
	cfg, _ := json.Marshal(map[string]string{
		"phone_number_id": "pn-1",
		"access_token":    "tok",
		// waba_id intentionally missing
	})
	return &domain.Channel{ID: id, AccountID: accountID, Type: "whatsapp",
		Name: "WA", Config: json.RawMessage(cfg)}
}

func TestWAConfig_CreateTemplate_MissingWABA_400(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannelNoWABA("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/channels/ch-1/templates", map[string]interface{}{
		"name":       "ok_name",
		"category":   "UTILITY",
		"language":   "pt_BR",
		"components": []map[string]interface{}{{"type": "BODY", "text": "oi"}},
	}, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "MISSING_WABA_ID", body["code"])
}

func TestWAConfig_DeleteTemplate_MissingWABA_400(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannelNoWABA("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodDelete, "/v1/channels/ch-1/templates/hello", nil, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "MISSING_WABA_ID", body["code"])
}

func TestWAConfig_CreateTemplate_MetaFailure_502(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad"}`))
	}))
	t.Cleanup(srv.Close)
	origURL := waadapter.GetBaseURLForTesting()
	waadapter.SetBaseURLForTesting(srv.URL)
	t.Cleanup(func() { waadapter.SetBaseURLForTesting(origURL) })

	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/channels/ch-1/templates", map[string]interface{}{
		"name":       "ok_name",
		"category":   "UTILITY",
		"language":   "pt_BR",
		"components": []map[string]interface{}{{"type": "BODY", "text": "oi"}},
	}, testToken)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "WA_TEMPLATE_CREATE_FAILED", body["code"])
}

func TestWAConfig_CreateTemplate_MetaValidationError_422(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid template name","type":"OAuthException","code":100,"fbtrace_id":"abc"}}`))
	}))
	t.Cleanup(srv.Close)
	origURL := waadapter.GetBaseURLForTesting()
	waadapter.SetBaseURLForTesting(srv.URL)
	t.Cleanup(func() { waadapter.SetBaseURLForTesting(origURL) })

	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/channels/ch-1/templates", map[string]interface{}{
		"name":       "ok_name",
		"category":   "UTILITY",
		"language":   "pt_BR",
		"components": []map[string]interface{}{{"type": "BODY", "text": "oi"}},
	}, testToken)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "INVALID_PARAM", body["code"])
}

func TestWAConfig_DeleteTemplate_MetaFailure_502(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	t.Cleanup(srv.Close)
	origURL := waadapter.GetBaseURLForTesting()
	waadapter.SetBaseURLForTesting(srv.URL)
	t.Cleanup(func() { waadapter.SetBaseURLForTesting(origURL) })

	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodDelete, "/v1/channels/ch-1/templates/hello", nil, testToken)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "WA_TEMPLATE_DELETE_FAILED", body["code"])
}

func TestWAConfig_UpdateProfile_MetaFailure_502(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	origURL := waadapter.GetBaseURLForTesting()
	waadapter.SetBaseURLForTesting(srv.URL)
	t.Cleanup(func() { waadapter.SetBaseURLForTesting(origURL) })

	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodPut, "/v1/channels/ch-1/whatsapp/profile",
		map[string]interface{}{"about": "new"}, testToken)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "WA_PROFILE_UPDATE_FAILED", body["code"])
}

func TestWAConfig_UploadPicture_MetaFailure_502(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	origURL := waadapter.GetBaseURLForTesting()
	waadapter.SetBaseURLForTesting(srv.URL)
	t.Cleanup(func() { waadapter.SetBaseURLForTesting(origURL) })

	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	rec := uploadMultipart(t, router, "/v1/channels/ch-1/whatsapp/profile/picture", testToken,
		jpeg, "x.jpg", "image/jpeg")

	assert.Equal(t, http.StatusBadGateway, rec.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	assert.Equal(t, "WA_PICTURE_UPLOAD_FAILED", body["code"])
}

func TestWAConfig_CreateTemplate_BadJSON_400(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	req := httptest.NewRequest(http.MethodPost, "/v1/channels/ch-1/templates",
		strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWAConfig_UpdateProfile_BadJSON_400(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)

	req := httptest.NewRequest(http.MethodPut, "/v1/channels/ch-1/whatsapp/profile",
		strings.NewReader("{not valid json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestMetaPictureFingerprint_StripsQuery(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"https://pps.whatsapp.net/v/t61.24694-24/abc_n.jpg", "https://pps.whatsapp.net/v/t61.24694-24/abc_n.jpg"},
		{"https://pps.whatsapp.net/v/t61.24694-24/abc_n.jpg?ccb=11&oh=def&oe=123", "https://pps.whatsapp.net/v/t61.24694-24/abc_n.jpg"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, metaPictureFingerprint(c.in))
	}
}

// HandleDeleteProfilePicture clears the cfg markers and emits 204 even
// when the channel had nothing cached — DELETE is idempotent.
func TestWAConfig_DeleteProfilePicture_NoCache_204(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", testAccountID), nil)
	channelRepo.On("MergeCredentials", mock.Anything, "ch-1", mock.MatchedBy(func(fields map[string]string) bool {
		return fields["wa_profile_picture_path"] == "" && fields["wa_profile_picture_url_fp"] == ""
	})).Return(nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodDelete, "/v1/channels/ch-1/whatsapp/profile/picture", nil, testToken)

	assert.Equal(t, http.StatusNoContent, w.Code)
	channelRepo.AssertCalled(t, "MergeCredentials", mock.Anything, "ch-1", mock.Anything)
}

// Cross-tenant DELETE is rejected — account B cannot purge account A's
// cached picture even by guessing the channel ID.
func TestWAConfig_DeleteProfilePicture_CrossTenant_404(t *testing.T) {
	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(waChannel("ch-1", "other-account"), nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodDelete, "/v1/channels/ch-1/whatsapp/profile/picture", nil, testToken)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// When Meta returns a fresh URL whose fingerprint matches what the
// channel config already pinned, the GET must NOT spawn a re-cache
// goroutine — otherwise every poll would re-download the same image.
// We verify by asserting MergeCredentials was never invoked.
func TestWAConfig_GetProfile_FingerprintMatch_SkipsRecache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"about":"x","profile_picture_url":"https://cdn.meta/pic.jpg?ccb=1&oe=2"}]}`))
	}))
	t.Cleanup(srv.Close)
	origURL := waadapter.GetBaseURLForTesting()
	waadapter.SetBaseURLForTesting(srv.URL)
	t.Cleanup(func() { waadapter.SetBaseURLForTesting(origURL) })

	cfg, _ := json.Marshal(map[string]string{
		"phone_number_id":           "pn-123",
		"waba_id":                   "waba-456",
		"access_token":              "EAAtok",
		"wa_profile_picture_url_fp": "https://cdn.meta/pic.jpg",
	})
	ch := &domain.Channel{ID: "ch-1", AccountID: testAccountID, Type: "whatsapp", Name: "WA", Config: json.RawMessage(cfg)}

	channelRepo := new(MockChannelRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	channelRepo.On("GetByID", mock.Anything, "ch-1").Return(ch, nil)

	router := setupWAConfigRouter(channelRepo, jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/channels/ch-1/whatsapp/profile", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)
	channelRepo.AssertNotCalled(t, "MergeCredentials", mock.Anything, mock.Anything, mock.Anything)
}
