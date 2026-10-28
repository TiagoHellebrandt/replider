package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestHandleTranscribe_CrossAccountReturns404 locks in US-007: a message
// that belongs to a foreign account must not be retrievable via the
// transcription endpoint. The repository's GetByID returns (nil, nil)
// because the conversations JOIN filters out rows whose account_id !=
// the caller's accountID claim, and the handler maps that to 404 —
// without leaking whether the messageID existed at all.
func TestHandleTranscribe_CrossAccountReturns404(t *testing.T) {
	msgRepo := new(MockMessageRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	const foreignMsgID = "01HXFOREIGNMSGABCDEFGHIJK0"

	// Repo enforces the account boundary via the conversations JOIN —
	// a foreign messageID looks identical to a missing one from the
	// caller's perspective: (nil, nil). Handler must map this to 404.
	msgRepo.On("GetByID", mock.Anything, foreignMsgID, testAccountID).Return(nil, nil)

	// A non-nil transcriber is required, otherwise the handler short-
	// circuits with 503 before the repo is even consulted.
	handler := NewTranscribeHandler(msgRepo, nil, stubTranscriber{}, nil)

	router := setupTestRouter()
	router.Use(setupAuthMiddleware(jwtMock))
	router.POST("/v1/messages/:id/transcribe", handler.HandleTranscribe)

	w := performRequest(router, http.MethodPost, "/v1/messages/"+foreignMsgID+"/transcribe", nil, testToken)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var body domain.AppError
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "MESSAGE_NOT_FOUND", body.Code)

	// Critical assertion: the repo was called with the caller's
	// accountID, not the message's true owner. This proves the
	// signature change (accountID is now a required parameter, not
	// re-derived inside the repo) is the gate.
	msgRepo.AssertCalled(t, "GetByID", mock.Anything, foreignMsgID, testAccountID)
}

// TestHandleTranscribe_PassesAccountIDFromContext verifies the handler
// reads accountID from the JWT context and forwards it to the repo, so
// a forged route param cannot bypass the boundary.
func TestHandleTranscribe_PassesAccountIDFromContext(t *testing.T) {
	msgRepo := new(MockMessageRepo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testToken, testUserID, testAccountID, "admin")

	const msgID = "01HXMSGOWNEDBYCALLER000000"
	msgRepo.On("GetByID", mock.Anything, msgID, testAccountID).Return(nil, nil)

	handler := NewTranscribeHandler(msgRepo, nil, stubTranscriber{}, nil)

	router := setupTestRouter()
	router.Use(setupAuthMiddleware(jwtMock))
	router.POST("/v1/messages/:id/transcribe", handler.HandleTranscribe)

	performRequest(router, http.MethodPost, "/v1/messages/"+msgID+"/transcribe", nil, testToken)

	msgRepo.AssertCalled(t, "GetByID", mock.Anything, msgID, testAccountID)
	// Negative assertion: must NOT have been called with any other
	// accountID — including the empty string. Catches a regression
	// where the handler drops the JWT claim.
	msgRepo.AssertNotCalled(t, "GetByID", mock.Anything, msgID, "")
}

// stubTranscriber is a minimal domain.Transcriber so the handler does
// not short-circuit with 503 before reaching the repo guard.
type stubTranscriber struct{}

func (stubTranscriber) Transcribe(_ context.Context, _ io.Reader, _ string) (string, error) {
	return "", nil
}
