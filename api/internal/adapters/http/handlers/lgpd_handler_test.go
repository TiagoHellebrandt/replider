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
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLGPDRepo is a minimal local mock sufficient for the handler tests.
type MockLGPDRepo struct{ mock.Mock }

func (m *MockLGPDRepo) ListConversationsByContact(ctx context.Context, accountID, externalID string) ([]*domain.LGPDExportConversation, error) {
	args := m.Called(ctx, accountID, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.LGPDExportConversation), args.Error(1)
}
func (m *MockLGPDRepo) ListMessagesByContact(ctx context.Context, accountID, externalID string) ([]*domain.LGPDExportMessage, error) {
	args := m.Called(ctx, accountID, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.LGPDExportMessage), args.Error(1)
}
func (m *MockLGPDRepo) ListCampaignParticipations(ctx context.Context, accountID, externalID string) ([]*domain.LGPDCampaignParticipation, error) {
	args := m.Called(ctx, accountID, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.LGPDCampaignParticipation), args.Error(1)
}
func (m *MockLGPDRepo) AnonymizeContact(ctx context.Context, accountID, externalID, anonID string) error {
	return m.Called(ctx, accountID, externalID, anonID).Error(0)
}

// MockTxManagerLGPD is a passthrough so the usecase runs the closure
// synchronously — we don't need real transactional semantics in handler
// tests (the repo methods are mocked).
type MockTxManagerLGPD struct{ mock.Mock }

func (m *MockTxManagerLGPD) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	_ = m.Called(ctx, fn)
	return fn(ctx)
}

// ──────────────────────────────────────────────────────────────────
// Router helper
// ──────────────────────────────────────────────────────────────────

func setupLGPDRouter(
	contactRepo *MockContactRepo,
	lgpdRepo *MockLGPDRepo,
	jwtMock *MockJWTService,
) *gin.Engine {
	txm := &MockTxManagerLGPD{}
	txm.On("WithinTransaction", mock.Anything, mock.Anything).Return(nil).Maybe()
	exportUC := usecases.NewExportContactDataUseCase(contactRepo, lgpdRepo)
	anonUC := usecases.NewAnonymizeContactUseCase(contactRepo, lgpdRepo, txm)
	h := NewLGPDHandler(exportUC, anonUC)

	router := setupTestRouter()
	authMw := setupAuthMiddleware(jwtMock)
	g := router.Group("/v1/lgpd", authMw, middleware.RequireRole("admin"))
	{
		// Mirrors setupLGPDRoutes: POST on both, because the data subject's
		// phone number must not travel in a query string.
		g.POST("/export", h.HandleExport)
		g.POST("/anonymize", h.HandleAnonymize)
	}
	return router
}

// ──────────────────────────────────────────────────────────────────
// Auth + RBAC
// ──────────────────────────────────────────────────────────────────

func TestLGPD_NoAuth_401(t *testing.T) {
	router := setupLGPDRouter(new(MockContactRepo), new(MockLGPDRepo), new(MockJWTService))
	w := performRequest(router, http.MethodPost, "/v1/lgpd/export", map[string]any{"external_id": "5511"}, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLGPD_NonAdmin_403(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "attendant")
	router := setupLGPDRouter(new(MockContactRepo), new(MockLGPDRepo), jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/lgpd/export", map[string]any{"external_id": "5511"}, testToken)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ──────────────────────────────────────────────────────────────────
// Validation
// ──────────────────────────────────────────────────────────────────

func TestLGPD_Export_MissingExternalID_400(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	router := setupLGPDRouter(new(MockContactRepo), new(MockLGPDRepo), jwtMock)

	w := performRequest(router, http.MethodPost, "/v1/lgpd/export", map[string]any{}, testToken)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "EXTERNAL_ID_REQUIRED", body["code"])
}

func TestLGPD_Anonymize_MissingExternalID_400(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	router := setupLGPDRouter(new(MockContactRepo), new(MockLGPDRepo), jwtMock)

	w := performRequest(router, http.MethodPost, "/v1/lgpd/anonymize", map[string]any{}, testToken)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ──────────────────────────────────────────────────────────────────
// Export happy path
// ──────────────────────────────────────────────────────────────────

func TestLGPD_Export_Success_ReturnsBundle(t *testing.T) {
	contactRepo := new(MockContactRepo)
	lgpdRepo := new(MockLGPDRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	contact := &domain.Contact{
		ID:         "c-1",
		AccountID:  testAccountID,
		ExternalID: "5511",
		Name:       strp("João"),
	}
	contactRepo.On("GetByExternalID", mock.Anything, testAccountID, "5511").Return(contact, nil)
	lgpdRepo.On("ListConversationsByContact", mock.Anything, testAccountID, "5511").Return(
		[]*domain.LGPDExportConversation{{ID: "conv-1", ChannelID: "ch", ContactID: "5511", Status: "open", CreatedAt: time.Now()}}, nil)
	lgpdRepo.On("ListMessagesByContact", mock.Anything, testAccountID, "5511").Return(
		[]*domain.LGPDExportMessage{{ID: "m-1", Sender: "5511", Type: "text", Direction: "inbound", CreatedAt: time.Now()}}, nil)
	lgpdRepo.On("ListCampaignParticipations", mock.Anything, testAccountID, "5511").Return(
		[]*domain.LGPDCampaignParticipation{}, nil)

	router := setupLGPDRouter(contactRepo, lgpdRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/lgpd/export", map[string]any{"external_id": "5511"}, testToken)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Disposition"), "lgpd-export-5511.json")

	var bundle domain.LGPDExportBundle
	_ = json.Unmarshal(w.Body.Bytes(), &bundle)
	assert.Equal(t, testAccountID, bundle.AccountID)
	assert.NotNil(t, bundle.Contact)
	assert.Len(t, bundle.Conversations, 1)
	assert.Len(t, bundle.Messages, 1)
	assert.False(t, bundle.GeneratedAt.IsZero())
}

func TestLGPD_Export_NoData_404Mapped(t *testing.T) {
	contactRepo := new(MockContactRepo)
	lgpdRepo := new(MockLGPDRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	// Nothing on file anywhere.
	contactRepo.On("GetByExternalID", mock.Anything, testAccountID, "nobody").Return(nil, nil)
	lgpdRepo.On("ListConversationsByContact", mock.Anything, testAccountID, "nobody").Return([]*domain.LGPDExportConversation{}, nil)
	lgpdRepo.On("ListMessagesByContact", mock.Anything, testAccountID, "nobody").Return([]*domain.LGPDExportMessage{}, nil)
	lgpdRepo.On("ListCampaignParticipations", mock.Anything, testAccountID, "nobody").Return([]*domain.LGPDCampaignParticipation{}, nil)

	router := setupLGPDRouter(contactRepo, lgpdRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/lgpd/export", map[string]any{"external_id": "nobody"}, testToken)

	// AppError with no explicit Status defaults to 400 in respondError; we
	// just assert the code name so we catch renames in one place.
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "LGPD_NO_DATA", body["code"])
}

// ──────────────────────────────────────────────────────────────────
// Anonymize happy path + tenant safety
// ──────────────────────────────────────────────────────────────────

func TestLGPD_Anonymize_Success_204(t *testing.T) {
	contactRepo := new(MockContactRepo)
	lgpdRepo := new(MockLGPDRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	contact := &domain.Contact{ID: "c-1", AccountID: testAccountID, ExternalID: "5511"}
	contactRepo.On("GetByExternalID", mock.Anything, testAccountID, "5511").Return(contact, nil)
	// The externalID passed to AnonymizeContact must match the victim;
	// the anonID is generated internally — assert via .MatchedBy.
	lgpdRepo.On("AnonymizeContact", mock.Anything, testAccountID, "5511",
		mock.MatchedBy(func(anonID string) bool {
			// anonID starts with "anon-" and is ≥ 30 chars (ulid = 26 chars).
			return len(anonID) >= 30 && anonID[:5] == "anon-"
		})).Return(nil)

	router := setupLGPDRouter(contactRepo, lgpdRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/lgpd/anonymize", map[string]any{"external_id": "5511"}, testToken)

	assert.Equal(t, http.StatusNoContent, w.Code)
	lgpdRepo.AssertExpectations(t)
}

func TestLGPD_Anonymize_NoContact_404Code(t *testing.T) {
	contactRepo := new(MockContactRepo)
	lgpdRepo := new(MockLGPDRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	contactRepo.On("GetByExternalID", mock.Anything, testAccountID, "nobody").Return(nil, nil)

	router := setupLGPDRouter(contactRepo, lgpdRepo, jwtMock)
	w := performRequest(router, http.MethodPost, "/v1/lgpd/anonymize", map[string]any{"external_id": "nobody"}, testToken)

	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "CONTACT_NOT_FOUND", body["code"])
	lgpdRepo.AssertNotCalled(t, "AnonymizeContact")
}

// ──────────────────────────────────────────────────────────────────
// The identifier must not travel in the URL
// ──────────────────────────────────────────────────────────────────

// The data subject's identifier is a phone number. In a query string it is
// written to the nginx access log, the edge provider's log, the browser
// history and any Referer sent onward — the LGPD endpoint itself leaking the
// personal data of the person exercising a privacy right.
//
// This pins the shape, not just the wiring: a body-less call carrying the id
// in the URL must be refused rather than quietly honoured.
func TestLGPD_Export_RejectsIdentifierInQueryString(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	router := setupLGPDRouter(new(MockContactRepo), new(MockLGPDRepo), jwtMock)

	w := performRequest(router, http.MethodPost,
		"/v1/lgpd/export?external_id=5511999999999", nil, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "EXTERNAL_ID_REQUIRED")
}

func TestLGPD_Anonymize_RejectsIdentifierInQueryString(t *testing.T) {
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
	router := setupLGPDRouter(new(MockContactRepo), new(MockLGPDRepo), jwtMock)

	w := performRequest(router, http.MethodPost,
		"/v1/lgpd/anonymize?external_id=5511999999999", nil, testToken)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "EXTERNAL_ID_REQUIRED")
}
