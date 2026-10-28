package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeAuditLog captures Create calls for assertions; the read methods are
// unused by the trial-bypass path so they return zero values.
type fakeAuditLog struct{ created []*domain.AuditLog }

func (f *fakeAuditLog) Create(_ context.Context, l *domain.AuditLog) error {
	f.created = append(f.created, l)
	return nil
}
func (f *fakeAuditLog) ListByAccount(context.Context, string, int32, int32) ([]*domain.AuditLog, error) {
	return nil, nil
}
func (f *fakeAuditLog) CountByAccount(context.Context, string) (int64, error) { return 0, nil }
func (f *fakeAuditLog) ListFiltered(context.Context, domain.AuditLogFilter) ([]*domain.AuditLog, error) {
	return nil, nil
}
func (f *fakeAuditLog) CountFiltered(context.Context, domain.AuditLogFilter) (int64, error) {
	return 0, nil
}

// registerAdminRoutes mirrors the production wiring in
// internal/adapters/http/router/router.go so the RBAC test exercises the
// exact middleware chain (authMw + RequireRole("admin")) that gates every
// /admin/* endpoint. If you add or rename an admin route in router.go,
// add it here too — otherwise the 403 guarantee will silently slip.
func registerAdminRoutes(router *gin.Engine, handler *AdminHandler, jwtMock *MockJWTService) {
	authMw := setupAuthMiddleware(jwtMock)
	admin := router.Group("/v1/admin")
	admin.Use(authMw)
	admin.Use(middleware.RequireRole("admin"))
	admin.POST("/smtp/test", handler.HandleSendSMTPTest)
	admin.GET("/wa-webhook-events", handler.HandleListWAWebhookEvents)
	admin.POST("/wa-webhook-events/:id/retry", handler.HandleRetryWAWebhookEvent)
	admin.GET("/wa-meter", handler.HandleListWABillingMeter)
	admin.GET("/agents/invalid-timezones", handler.HandleListInvalidTimezoneAgents)
	admin.POST("/accounts/:id/trial-bypass", handler.HandleSetTrialBypass)
	admin.GET("/billing/metrics", handler.HandleGetBillingMetrics)
	admin.GET("/accounts/health", handler.HandleListAccountHealth)
	admin.GET("/nps", handler.HandleGetNPSMetrics)
	admin.GET("/cancellation-survey", handler.HandleGetCancellationSurvey)
}

// adminRoutes returns the full list of /admin/* endpoints that ship today.
// Keep in lock-step with registerAdminRoutes / router.go.
func adminRoutes() []struct {
	method string
	path   string
} {
	return []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/admin/smtp/test"},
		{http.MethodGet, "/v1/admin/wa-webhook-events"},
		{http.MethodPost, "/v1/admin/wa-webhook-events/some-id/retry"},
		{http.MethodGet, "/v1/admin/wa-meter"},
		{http.MethodGet, "/v1/admin/agents/invalid-timezones"},
		{http.MethodPost, "/v1/admin/accounts/some-account/trial-bypass"},
		{http.MethodGet, "/v1/admin/billing/metrics"},
		{http.MethodGet, "/v1/admin/accounts/health"},
		{http.MethodGet, "/v1/admin/nps"},
		{http.MethodGet, "/v1/admin/cancellation-survey"},
	}
}

// TestHandleSetTrialBypass_WritesAuditWithBeforeAfter locks in US-014: a
// trial-bypass flip records an audit row capturing the actor and the full
// before→after transition (plus reason), so the toggle can't be abused
// silently.
func TestHandleSetTrialBypass_WritesAuditWithBeforeAfter(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	accountRepo := new(MockAccountRepo)
	// Prior state: bypass currently OFF.
	accountRepo.On("GetByID", mock.Anything, "acc-x").
		Return(&domain.Account{ID: "acc-x", TrialBypass: false}, nil)
	accountRepo.On("SetTrialBypass", mock.Anything, "acc-x", true, "vip onboarding").Return(nil)

	audit := &fakeAuditLog{}
	handler := NewAdminHandler(nil)
	handler.SetAccountRepo(accountRepo)
	handler.SetAuditLog(audit)

	router := setupTestRouter()
	registerAdminRoutes(router, handler, jwtMock)

	w := performRequest(router, http.MethodPost, "/v1/admin/accounts/acc-x/trial-bypass",
		map[string]any{"bypass": true, "reason": "vip onboarding"}, testToken)

	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, audit.created, 1)

	entry := audit.created[0]
	assert.Equal(t, "account.trial_bypass.set", entry.Action)
	assert.Equal(t, "account", entry.EntityType)
	assert.Equal(t, "acc-x", entry.EntityID)
	assert.Equal(t, "acc-x", entry.AccountID)
	require.NotNil(t, entry.ActorID)
	assert.Equal(t, testUserID, *entry.ActorID)

	var p map[string]any
	require.NoError(t, json.Unmarshal(entry.Payload, &p))
	assert.Equal(t, false, p["before"])
	assert.Equal(t, true, p["after"])
	assert.Equal(t, "vip onboarding", p["reason"])

	accountRepo.AssertExpectations(t)
}

// TestAdminEndpoints_AttendantForbidden locks in US-002: every /admin/*
// endpoint must return 403 for a caller whose JWT role is "attendant".
// Handlers are stubbed via NewAdminHandler(nil) because RequireRole
// aborts before the handler runs — so use-case wiring isn't needed.
func TestAdminEndpoints_AttendantForbidden(t *testing.T) {
	for _, rt := range adminRoutes() {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			jwtMock := new(MockJWTService)
			expectAuth(jwtMock, testToken, testUserID, testAccountID, "attendant")

			handler := NewAdminHandler(nil)

			router := setupTestRouter()
			registerAdminRoutes(router, handler, jwtMock)

			w := performRequest(router, rt.method, rt.path, nil, testToken)

			assert.Equal(t, http.StatusForbidden, w.Code, "non-admin must be forbidden")
			var resp map[string]interface{}
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			assert.Equal(t, "FORBIDDEN", resp["code"])
		})
	}
}

// TestAdminEndpoints_UnknownRoleForbidden guards against drift where a
// non-admin role string (e.g. someone hand-rolls a "viewer" role in a
// future epic) accidentally satisfies the gate.
func TestAdminEndpoints_UnknownRoleForbidden(t *testing.T) {
	for _, rt := range adminRoutes() {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			jwtMock := new(MockJWTService)
			expectAuth(jwtMock, testToken, testUserID, testAccountID, "viewer")

			handler := NewAdminHandler(nil)

			router := setupTestRouter()
			registerAdminRoutes(router, handler, jwtMock)

			w := performRequest(router, rt.method, rt.path, nil, testToken)
			assert.Equal(t, http.StatusForbidden, w.Code)
		})
	}
}

// TestAdminEndpoints_AdminPassesRBAC confirms the gate is permissive for
// the right role — admin must get past the middleware. We only check the
// endpoints that won't NPE when their optional deps are nil (those return
// 503 instead). HandleSendSMTPTest dereferences a non-nil use case so
// it's excluded here; the 403 tests above already prove the gate runs
// before the handler.
func TestAdminEndpoints_AdminPassesRBAC(t *testing.T) {
	rbacSafe := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v1/admin/wa-webhook-events"},
		{http.MethodPost, "/v1/admin/wa-webhook-events/some-id/retry"},
		{http.MethodGet, "/v1/admin/wa-meter"},
		{http.MethodGet, "/v1/admin/agents/invalid-timezones"},
		{http.MethodPost, "/v1/admin/accounts/some-account/trial-bypass"},
		{http.MethodGet, "/v1/admin/billing/metrics"},
		{http.MethodGet, "/v1/admin/accounts/health"},
		{http.MethodGet, "/v1/admin/nps"},
		{http.MethodGet, "/v1/admin/cancellation-survey"},
	}
	for _, rt := range rbacSafe {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			jwtMock := new(MockJWTService)
			expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

			handler := NewAdminHandler(nil)

			router := setupTestRouter()
			registerAdminRoutes(router, handler, jwtMock)

			w := performRequest(router, rt.method, rt.path, nil, testToken)
			assert.NotEqual(t, http.StatusUnauthorized, w.Code)
			assert.NotEqual(t, http.StatusForbidden, w.Code)
		})
	}
}

// TestHandleListInvalidTimezoneAgents locks in US-048: the admin audit
// endpoint surfaces only agents whose Timezone fails time.LoadLocation
// (AgentRunner silently falls back to UTC for these, breaking booking math).
func TestHandleListInvalidTimezoneAgents(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	agentRepo := new(MockAgentRepo)
	agentRepo.On("ListAllTimezones", mock.Anything).Return([]*domain.AgentTimezone{
		{ID: "a-good", AccountID: "acc-1", Name: "Good", Timezone: "America/Sao_Paulo"},
		{ID: "a-bad", AccountID: "acc-1", Name: "Bad", Timezone: "Mars/Phobos"},
		{ID: "a-empty", AccountID: "acc-2", Name: "Empty", Timezone: ""},
	}, nil)

	handler := NewAdminHandler(nil)
	handler.SetAgentRepo(agentRepo)

	router := setupTestRouter()
	registerAdminRoutes(router, handler, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/admin/agents/invalid-timezones", nil, testToken)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Items []domain.AgentTimezone `json:"items"`
		Count int                    `json:"count"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	// Only the invalid "Mars/Phobos" tz is flagged; "" resolves to UTC via
	// LoadLocation so it is NOT considered invalid.
	require.Equal(t, 1, resp.Count)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "a-bad", resp.Items[0].ID)
}

// TestHandleListInvalidTimezoneAgents_RepoDisabled returns 503 when the agent
// repo is not wired (nil-safe parity with the other admin infra endpoints).
func TestHandleListInvalidTimezoneAgents_RepoDisabled(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	handler := NewAdminHandler(nil) // no SetAgentRepo

	router := setupTestRouter()
	registerAdminRoutes(router, handler, jwtMock)

	w := performRequest(router, http.MethodGet, "/v1/admin/agents/invalid-timezones", nil, testToken)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
