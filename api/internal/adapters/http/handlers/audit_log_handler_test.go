package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAuditLogRepo is local to handler tests (no shared mock yet).
type MockAuditLogRepo struct{ mock.Mock }

func (m *MockAuditLogRepo) Create(ctx context.Context, log *domain.AuditLog) error {
	return m.Called(ctx, log).Error(0)
}
func (m *MockAuditLogRepo) ListByAccount(ctx context.Context, accountID string, limit, offset int32) ([]*domain.AuditLog, error) {
	args := m.Called(ctx, accountID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}
func (m *MockAuditLogRepo) CountByAccount(ctx context.Context, accountID string) (int64, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockAuditLogRepo) ListFiltered(ctx context.Context, f domain.AuditLogFilter) ([]*domain.AuditLog, error) {
	args := m.Called(ctx, f)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}
func (m *MockAuditLogRepo) CountFiltered(ctx context.Context, f domain.AuditLogFilter) (int64, error) {
	args := m.Called(ctx, f)
	return args.Get(0).(int64), args.Error(1)
}

func setupAuditLogRouter(repo *MockAuditLogRepo, jwtMock *MockJWTService) *gin.Engine {
	h := NewAuditLogHandler(repo)
	router := setupTestRouter()
	authMw := setupAuditAuthMw(jwtMock)
	g := router.Group("/v1/audit", authMw, middleware.RequireRole("admin"))
	g.GET("", h.HandleList)
	g.GET("/export.csv", h.HandleExportCSV)
	return router
}

// setupAuditAuthMw is just a thin wrapper around the existing helper —
// kept as a shim so that future audit-specific auth middleware (e.g.
// a role scoped narrower than "admin") can slot in without rewriting
// every test.
func setupAuditAuthMw(jwtMock *MockJWTService) gin.HandlerFunc {
	return setupAuthMiddleware(jwtMock)
}

func TestAudit_NoAuth_401(t *testing.T) {
	router := setupAuditLogRouter(new(MockAuditLogRepo), new(MockJWTService))
	w := performRequest(router, http.MethodGet, "/v1/audit", nil, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAudit_NonAdmin_403(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "attendant")
	router := setupAuditLogRouter(new(MockAuditLogRepo), jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/audit", nil, testToken)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAudit_List_Success(t *testing.T) {
	repo := new(MockAuditLogRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	entries := []*domain.AuditLog{
		{ID: "a1", AccountID: testAccountID, Action: "lgpd.export",
			EntityType: "contact", EntityID: "5511", CreatedAt: time.Now()},
		{ID: "a2", AccountID: testAccountID, Action: "lgpd.anonymize",
			EntityType: "contact", EntityID: "5522", CreatedAt: time.Now()},
	}
	repo.On("ListFiltered", mock.Anything, mock.MatchedBy(func(f domain.AuditLogFilter) bool {
		return f.AccountID == testAccountID && f.Limit == 50 && f.Offset == 0 &&
			f.Action == nil && f.EntityType == nil && f.ActorID == nil
	})).Return(entries, nil)
	repo.On("CountFiltered", mock.Anything, mock.Anything).Return(int64(2), nil)

	router := setupAuditLogRouter(repo, jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/audit", nil, testToken)
	assert.Equal(t, http.StatusOK, w.Code)

	var body listAuditLogsResponse
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, int64(2), body.Total)
	assert.Len(t, body.Entries, 2)
	assert.Equal(t, "lgpd.export", body.Entries[0].Action)
}

func TestAudit_List_PaginationClamped(t *testing.T) {
	repo := new(MockAuditLogRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	// Request limit=1000 → clamped to 50.
	repo.On("ListFiltered", mock.Anything, mock.MatchedBy(func(f domain.AuditLogFilter) bool {
		return f.Limit == 50 && f.Offset == 10
	})).Return([]*domain.AuditLog{}, nil)
	repo.On("CountFiltered", mock.Anything, mock.Anything).Return(int64(0), nil)

	router := setupAuditLogRouter(repo, jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/audit?limit=1000&offset=10", nil, testToken)
	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestAudit_List_Filters(t *testing.T) {
	repo := new(MockAuditLogRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	repo.On("ListFiltered", mock.Anything, mock.MatchedBy(func(f domain.AuditLogFilter) bool {
		return f.AccountID == testAccountID &&
			f.Action != nil && *f.Action == "lgpd.export" &&
			f.EntityType != nil && *f.EntityType == "contact" &&
			f.ActorID != nil && *f.ActorID == "user-1"
	})).Return([]*domain.AuditLog{}, nil)
	repo.On("CountFiltered", mock.Anything, mock.Anything).Return(int64(0), nil)

	router := setupAuditLogRouter(repo, jwtMock)
	w := performRequest(router, http.MethodGet,
		"/v1/audit?action=lgpd.export&entity_type=contact&actor_id=user-1", nil, testToken)
	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestAudit_ExportCSV_Success(t *testing.T) {
	repo := new(MockAuditLogRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	actor := "user-1"
	entries := []*domain.AuditLog{
		{ID: "a1", AccountID: testAccountID, ActorID: &actor, Action: "lgpd.export",
			EntityType: "contact", EntityID: "5511", CreatedAt: now},
	}
	repo.On("ListFiltered", mock.Anything, mock.MatchedBy(func(f domain.AuditLogFilter) bool {
		return f.Limit == 10000
	})).Return(entries, nil)

	router := setupAuditLogRouter(repo, jwtMock)
	w := performRequest(router, http.MethodGet, "/v1/audit/export.csv", nil, testToken)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	body := w.Body.String()
	assert.Contains(t, body, "id,created_at,actor_id,action,entity_type,entity_id,payload")
	assert.Contains(t, body, "a1,")
	assert.Contains(t, body, "lgpd.export")
	assert.Contains(t, body, "user-1")
}
