package handlers

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// US-009 — Tenant boundary penetration suite.
//
// Every by-ID endpoint must return 404 (never 200, never 500) when the
// caller's JWT account does not own the resource. We exercise two attack
// shapes, whichever the repository layer produces:
//
//   - "exists-but-foreign": the repo still returns the row and the
//     usecase rejects on accountID mismatch (agents, channels).
//   - "scoped-to-nil": the repo's WHERE clause already filters by
//     accountID so a foreign id is indistinguishable from a missing one,
//     returning (nil, nil) (messages US-007, contacts US-008,
//     conversations).
//
// A 200 means cross-tenant read/write; a 500 means the boundary check
// blew up instead of cleanly denying — both are failures. The suite is a
// regression gate: a future WHERE-clause refactor or a dropped ownership
// check resurfaces here, on every PR, before it ships.

const foreignAccountID = "acc-ATTACKER-VICTIM" // never equals testAccountID ("acc-1")

// assertForeign404 performs the request and fails loudly unless the
// response is exactly 404.
func assertForeign404(t *testing.T, router *gin.Engine, method, path string, body interface{}) {
	t.Helper()
	w := performRequest(router, method, path, body, testToken)
	assert.Equalf(t, http.StatusNotFound, w.Code,
		"%s %s leaked a cross-tenant resource: got %d, body=%s", method, path, w.Code, w.Body.String())
}

func TestTenantIsolation_ForeignResourcesReturn404(t *testing.T) {
	// ── Agents: repo returns a row owned by a DIFFERENT account; the
	// usecase must reject on `agent.AccountID != caller`. ──
	t.Run("agents", func(t *testing.T) {
		for _, tc := range []struct {
			name, method, path string
			body               interface{}
		}{
			{"update", http.MethodPut, "/v1/agents/agent-foreign", map[string]any{"name": "hijack"}},
			{"toggle", http.MethodPatch, "/v1/agents/agent-foreign/toggle", map[string]any{"is_active": false}},
			{"delete", http.MethodDelete, "/v1/agents/agent-foreign", nil},
		} {
			t.Run(tc.name, func(t *testing.T) {
				agentRepo := new(MockAgentRepo)
				jwtMock := new(MockJWTService)
				expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
				agentRepo.On("GetByID", mock.Anything, "agent-foreign").Return(&domain.Agent{
					ID:        "agent-foreign",
					AccountID: foreignAccountID,
					Name:      "victim's bot",
				}, nil)

				handler := setupAgentTestRouter(agentRepo, jwtMock)
				router := setupTestRouter()
				registerAgentRoutes(router, handler, jwtMock)

				assertForeign404(t, router, tc.method, tc.path, tc.body)
				// The mutation/delete must NOT have been performed.
				agentRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
				agentRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
			})
		}
	})

	// ── Channels: same exists-but-foreign shape. ──
	t.Run("channels", func(t *testing.T) {
		for _, tc := range []struct {
			name, method, path string
			body               interface{}
		}{
			{"update", http.MethodPut, "/v1/channels/ch-foreign", map[string]any{"name": "hijack"}},
			{"toggle", http.MethodPatch, "/v1/channels/ch-foreign/toggle", nil},
			{"delete", http.MethodDelete, "/v1/channels/ch-foreign", nil},
		} {
			t.Run(tc.name, func(t *testing.T) {
				channelRepo := new(MockChannelRepo)
				agentRepo := new(MockAgentRepo)
				jwtMock := new(MockJWTService)
				expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
				channelRepo.On("GetByID", mock.Anything, "ch-foreign").Return(&domain.Channel{
					ID:        "ch-foreign",
					AccountID: foreignAccountID,
					Name:      "victim's channel",
					Type:      "whatsapp",
				}, nil)

				_, router := setupChannelTestRouter(channelRepo, agentRepo, jwtMock)

				assertForeign404(t, router, tc.method, tc.path, tc.body)
				channelRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
				channelRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
			})
		}
	})

	// ── Conversations: repo scopes by accountID in SQL → (nil, nil). ──
	t.Run("conversation_messages", func(t *testing.T) {
		convRepo := new(MockConversationRepo)
		msgRepo := new(MockMessageRepo)
		channelRepo := new(MockChannelRepo)
		jwtMock := new(MockJWTService)
		expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
		// Foreign/missing conversation: WHERE account_id filters it out.
		convRepo.On("GetByID", mock.Anything, "conv-foreign", testAccountID).Return(nil, nil)

		router := setupConversationTestRouter(convRepo, msgRepo, channelRepo, jwtMock)

		assertForeign404(t, router, http.MethodGet, "/v1/conversations/conv-foreign/messages", nil)
		// Must never have listed another tenant's messages.
		msgRepo.AssertNotCalled(t, "ListByConversationID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	// ── Contacts (US-008): repo scopes by accountID → (nil, nil). ──
	t.Run("contacts", func(t *testing.T) {
		for _, tc := range []struct {
			name, method, path string
			body               interface{}
		}{
			{"get", http.MethodGet, "/v1/contacts/contact-foreign", nil},
			{"update", http.MethodPut, "/v1/contacts/contact-foreign", map[string]any{"name": "hijack"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				contactRepo := new(MockContactRepo)
				jwtMock := new(MockJWTService)
				expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
				contactRepo.On("GetByID", mock.Anything, "contact-foreign", testAccountID).Return(nil, nil)

				handler := NewContactHandler(
					usecases.NewListContactsUseCase(contactRepo),
					usecases.NewGetContactUseCase(contactRepo),
					usecases.NewUpdateContactUseCase(contactRepo),
					contactRepo,
				)
				router := setupTestRouter()
				g := router.Group("/v1/contacts", setupAuthMiddleware(jwtMock), middleware.RequireRole("admin"))
				g.GET("/:id", handler.HandleGetContact)
				g.PUT("/:id", handler.HandleUpdateContact)

				assertForeign404(t, router, tc.method, tc.path, tc.body)
				contactRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
			})
		}
	})

	// ── Messages transcribe (US-007): repo scopes by accountID. ──
	t.Run("transcribe", func(t *testing.T) {
		msgRepo := new(MockMessageRepo)
		jwtMock := new(MockJWTService)
		expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")
		msgRepo.On("GetByID", mock.Anything, "msg-foreign", testAccountID).Return(nil, nil)

		handler := NewTranscribeHandler(msgRepo, nil, stubTranscriber{}, nil)
		router := setupTestRouter()
		g := router.Group("/v1/messages", setupAuthMiddleware(jwtMock), middleware.RequireRole("admin"))
		g.POST("/:id/transcribe", handler.HandleTranscribe)

		assertForeign404(t, router, http.MethodPost, "/v1/messages/msg-foreign/transcribe", nil)
	})
}
