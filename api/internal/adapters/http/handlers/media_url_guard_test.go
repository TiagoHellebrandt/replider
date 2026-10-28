package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/adapters/httpsafe"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// The SSRF guard is process-wide; run these under the production policy and
// restore whatever the previous test left.
func withProductionSSRFPolicy(t *testing.T) {
	t.Helper()
	prev := httpsafe.AllowsPrivateTargets()
	httpsafe.SetAllowPrivateTargets(false)
	t.Cleanup(func() { httpsafe.SetAllowPrivateTargets(prev) })
}

func TestValidateOutboundMediaURL(t *testing.T) {
	withProductionSSRFPolicy(t)

	rejected := []string{
		"http://db:5432",                                // internal service name
		"http://127.0.0.1:8080/health",                  // loopback
		"https://169.254.169.254/latest/meta-data/",     // cloud metadata
		"https://10.0.0.5/secret.ogg",                   // RFC1918
		"http://storage.googleapis.com/bucket/x.ogg",    // plain http, even to a public host
		"accounts/other-account/voice.ogg",              // GCS path outside the caller's prefix
		"accounts/" + testAccountID + "/../x/voice.ogg", // traversal
		"file:///etc/passwd",
		"ftp://example.com/x",
	}
	for _, raw := range rejected {
		assert.ErrorIs(t, validateOutboundMediaURL(testAccountID, raw), ErrInvalidMediaURL, raw)
	}

	accepted := []string{
		"",
		"accounts/" + testAccountID + "/voice.ogg", // the caller's own upload
		"https://8.8.8.8/voice.ogg",                // public https
	}
	for _, raw := range accepted {
		assert.NoError(t, validateOutboundMediaURL(testAccountID, raw), raw)
	}
}

// An attendant who can point media_url at the backend network turns the
// WhatsApp adapter (and the transcriber) into an SSRF read primitive. The
// reply endpoint must refuse before anything is persisted or sent.
func TestHumanReply_RejectsInternalMediaURL(t *testing.T) {
	withProductionSSRFPolicy(t)

	for _, raw := range []string{
		"http://10.0.0.5:5432/",
		"https://169.254.169.254/latest/meta-data/",
		"http://127.0.0.1/",
	} {
		t.Run(raw, func(t *testing.T) {
			convRepo := new(MockConversationRepo)
			msgRepo := new(MockMessageRepo)
			channelRepo := new(MockChannelRepo)
			mockProvider := new(MockMessageProvider)
			jwtMock := new(MockJWTService)
			expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

			now := time.Now()
			convRepo.On("GetByID", mock.Anything, "conv-1", testAccountID).Return(&domain.Conversation{
				ID: "conv-1", AccountID: testAccountID, ChannelID: "ch-1", ContactID: "contact-1",
				Status: domain.ConversationOpen, LastCustomerMessageAt: &now, CreatedAt: now,
			}, nil).Maybe()

			handler := NewConversationHandler(convRepo, msgRepo, channelRepo, nil,
				map[string]domain.MessageProvider{"whatsapp": mockProvider}, nil, nil)

			router := setupTestRouter()
			conversations := router.Group("/v1/conversations")
			conversations.Use(setupAuthMiddleware(jwtMock), middleware.RequireRole("admin", "attendant"))
			conversations.POST("/:id/reply", handler.HandleHumanReply)

			body := gin.H{"message": "audio", "message_type": "audio", "media_url": raw}
			w := performRequest(router, http.MethodPost, "/v1/conversations/conv-1/reply", body, testToken)

			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			var resp domain.AppError
			assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, "INVALID_MEDIA_URL", resp.Code)
			// Nothing was saved and nothing was sent.
			msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
			mockProvider.AssertNotCalled(t, "SendMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// The transcriber fetches through the guarded client, so even a stored
// media_url that predates the write-time guard cannot reach an internal
// address.
func TestTranscribeFetchAudio_RefusesInternalAddress(t *testing.T) {
	withProductionSSRFPolicy(t)

	h := NewTranscribeHandler(new(MockMessageRepo), nil, stubTranscriber{}, nil)
	for _, raw := range []string{"http://127.0.0.1:1/voice.ogg", "http://169.254.169.254/latest/meta-data/"} {
		_, err := h.fetchAudio(t.Context(), raw)
		assert.ErrorIs(t, err, httpsafe.ErrBlockedTarget, raw)
	}
}
