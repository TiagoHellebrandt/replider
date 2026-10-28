package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

type recordingSignalRepo struct {
	domain.SignalRepository
	upserted []*domain.Signal
}

func (r *recordingSignalRepo) Upsert(_ context.Context, s *domain.Signal) error {
	r.upserted = append(r.upserted, s)
	return nil
}

func signIngest(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func ingestRouter(repo domain.SignalRepository) *gin.Engine {
	h := NewSignalIngestHandler(repo, "global-secret")
	r := setupTestRouter()
	r.POST("/v1/webhooks/signals/:account_id", h.HandleIngest)
	return r
}

func postIngest(r *gin.Engine, accountID, signature string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/signals/"+accountID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if signature != "" {
		req.Header.Set("X-Signal-Signature", signature)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// Each account signs with its own derived key. A valid signature for
// account A, replayed against account B's URL, verifies under B's key and
// is refused — so a leaked customer secret compromises one feed, not all.
func TestSignalIngest_SignatureIsBoundToTheAccount(t *testing.T) {
	repo := &recordingSignalRepo{}
	r := ingestRouter(repo)
	body := []byte(`{"source":"pncp","external_id":"ext-1","title":"Pregão 12/2026"}`)

	secretA := AccountIngestSecret("global-secret", "acc-A")
	secretB := AccountIngestSecret("global-secret", "acc-B")
	assert.NotEqual(t, secretA, secretB)

	assert.Equal(t, http.StatusAccepted, postIngest(r, "acc-A", signIngest(secretA, body), body).Code)
	assert.Len(t, repo.upserted, 1)
	assert.Equal(t, "acc-A", repo.upserted[0].AccountID)

	// A's signature against B's feed.
	assert.Equal(t, http.StatusUnauthorized, postIngest(r, "acc-B", signIngest(secretA, body), body).Code)
	// The GLOBAL secret is never a valid signing key on its own.
	assert.Equal(t, http.StatusUnauthorized, postIngest(r, "acc-A", signIngest("global-secret", body), body).Code)
	// No signature at all.
	assert.Equal(t, http.StatusUnauthorized, postIngest(r, "acc-A", "", body).Code)
	assert.Len(t, repo.upserted, 1)
}

func TestAccountIngestSecret_IsDeterministicAndOpaque(t *testing.T) {
	a := AccountIngestSecret("global-secret", "acc-A")
	assert.Equal(t, a, AccountIngestSecret("global-secret", "acc-A"))
	assert.Len(t, a, 64)
	assert.NotEqual(t, a, AccountIngestSecret("other-global", "acc-A"))
}
