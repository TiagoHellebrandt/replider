package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/http/middleware"
	"github.com/replider/api/internal/adapters/httpsafe"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

const (
	testWebhookToken     = "test-token-webhook-handler"
	testWebhookAccountID = "acc-wh"
	testWebhookUserID    = "u-wh"
)

// inMemoryAccountWebhookRepo is the smallest impl that satisfies the
// AccountWebhookRepository contract for handler tests. We don't need
// the failure-counter machinery — just CRUD scoped by account_id.
type inMemoryAccountWebhookRepo struct {
	mu    sync.Mutex
	hooks []*domain.AccountWebhook
}

func (r *inMemoryAccountWebhookRepo) Create(_ context.Context, w *domain.AccountWebhook) (*domain.AccountWebhook, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks = append(r.hooks, w)
	return w, nil
}
func (r *inMemoryAccountWebhookRepo) GetByID(_ context.Context, accountID, id string) (*domain.AccountWebhook, error) {
	for _, w := range r.hooks {
		if w.ID == id && w.AccountID == accountID {
			return w, nil
		}
	}
	return nil, nil
}
func (r *inMemoryAccountWebhookRepo) ListByAccount(_ context.Context, accountID string) ([]*domain.AccountWebhook, error) {
	out := []*domain.AccountWebhook{}
	for _, w := range r.hooks {
		if w.AccountID == accountID {
			out = append(out, w)
		}
	}
	return out, nil
}
func (r *inMemoryAccountWebhookRepo) ListActiveForEvent(_ context.Context, _ string, _ string) ([]*domain.AccountWebhook, error) {
	return nil, nil
}
func (r *inMemoryAccountWebhookRepo) Update(_ context.Context, w *domain.AccountWebhook) (*domain.AccountWebhook, error) {
	for i, ex := range r.hooks {
		if ex.ID == w.ID && ex.AccountID == w.AccountID {
			r.hooks[i] = w
			return w, nil
		}
	}
	return w, nil
}
func (r *inMemoryAccountWebhookRepo) Delete(_ context.Context, accountID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.hooks[:0]
	for _, w := range r.hooks {
		if w.ID == id && w.AccountID == accountID {
			continue
		}
		out = append(out, w)
	}
	r.hooks = out
	return nil
}
func (r *inMemoryAccountWebhookRepo) RecordSuccess(_ context.Context, _ string) error { return nil }
func (r *inMemoryAccountWebhookRepo) RecordFailure(_ context.Context, _ string, _ string) error {
	return nil
}
func (r *inMemoryAccountWebhookRepo) Disable(_ context.Context, _ string) error { return nil }

func setupWebhookRouter() (*inMemoryAccountWebhookRepo, *gin.Engine, *MockJWTService) {
	repo := &inMemoryAccountWebhookRepo{}
	handler := NewAccountWebhookHandler(repo)
	jwtMock := new(MockJWTService)
	expectAuth(jwtMock, testWebhookToken, testWebhookUserID, testWebhookAccountID, "admin")
	authMw := setupAuthMiddleware(jwtMock)
	router := setupTestRouter()
	g := router.Group("/v1/account-webhooks", authMw, middleware.RequireRole("admin"))
	g.GET("", handler.HandleList)
	g.POST("", handler.HandleCreate)
	g.PUT("/:id", handler.HandleUpdate)
	g.DELETE("/:id", handler.HandleDelete)
	return repo, router, jwtMock
}

// TestWebhook_CreateReturnsSecretOnce asserts the create endpoint
// hands back the plaintext secret + warning ONCE; subsequent
// list/get must never echo it.
func TestWebhook_CreateReturnsSecretOnce(t *testing.T) {
	repo, router, _ := setupWebhookRouter()
	body := map[string]any{
		"url":         "https://example.com/hook",
		"events":      []string{"message.received"},
		"description": "test",
	}
	w := performRequest(router, http.MethodPost, "/v1/account-webhooks", body, testWebhookToken)
	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	secret, ok := resp["secret"].(string)
	assert.True(t, ok)
	assert.Len(t, secret, 64) // 32 bytes hex-encoded
	assert.NotEmpty(t, resp["secret_warning"])
	assert.Len(t, repo.hooks, 1)

	// List must not echo the secret again.
	w2 := performRequest(router, http.MethodGet, "/v1/account-webhooks", nil, testWebhookToken)
	assert.Equal(t, http.StatusOK, w2.Code)
	var list []map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &list)
	assert.Len(t, list, 1)
	_, leaked := list[0]["secret"]
	assert.False(t, leaked, "secret must never be echoed by list endpoint")
}

// TestWebhook_RejectsHTTP asserts plain-HTTP URLs (outside localhost)
// are rejected before they ever reach the dispatcher.
func TestWebhook_RejectsHTTP(t *testing.T) {
	_, router, _ := setupWebhookRouter()
	body := map[string]any{
		"url":    "http://example.com/hook",
		"events": []string{"message.received"},
	}
	w := performRequest(router, http.MethodPost, "/v1/account-webhooks", body, testWebhookToken)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp domain.AppError
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_URL", resp.Code)
}

// TestWebhook_RejectsUnknownEvent guards the event whitelist — a typo
// in `events` must fail-fast at the API layer instead of going into the
// DB and silently never firing.
func TestWebhook_RejectsUnknownEvent(t *testing.T) {
	_, router, _ := setupWebhookRouter()
	body := map[string]any{
		"url":    "https://example.com/hook",
		"events": []string{"message.received", "totally_made_up"},
	}
	w := performRequest(router, http.MethodPost, "/v1/account-webhooks", body, testWebhookToken)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp domain.AppError
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "INVALID_EVENTS", resp.Code)
}

// TestWebhook_DeleteScopesByAccount asserts you can't delete another
// account's webhook even if you guess the id — multi-tenant safety.
func TestWebhook_DeleteScopesByAccount(t *testing.T) {
	repo, router, _ := setupWebhookRouter()
	repo.hooks = append(repo.hooks, &domain.AccountWebhook{
		ID: "w-foreign", AccountID: "other-acct",
		URL: "https://x.example.com", Secret: "x",
		Events: []string{"message.received"}, IsActive: true,
	})
	w := performRequest(router, http.MethodDelete, "/v1/account-webhooks/w-foreign", nil, testWebhookToken)
	// Delete is best-effort idempotent — returns 204 even if the row
	// belonged to another account, but the row must NOT be removed.
	_ = w
	if len(repo.hooks) != 1 {
		t.Fatalf("foreign-account row should not be deletable; got %d rows", len(repo.hooks))
	}
}

// US-F011: a subscription URL pointing at the internal network (loopback,
// RFC1918, cloud metadata) is an SSRF primitive — reject it at create AND
// update, before the row can ever reach the dispatcher.
func TestWebhook_RejectsInternalTargets(t *testing.T) {
	prev := httpsafe.AllowsPrivateTargets()
	httpsafe.SetAllowPrivateTargets(false)
	t.Cleanup(func() { httpsafe.SetAllowPrivateTargets(prev) })

	_, router, _ := setupWebhookRouter()
	for _, raw := range []string{
		"https://169.254.169.254/latest/meta-data/",
		"https://10.0.0.7/hook",
		"https://[::1]/hook",
	} {
		body := map[string]any{"url": raw, "events": []string{"message.received"}}
		w := performRequest(router, http.MethodPost, "/v1/account-webhooks", body, testWebhookToken)
		assert.Equal(t, http.StatusBadRequest, w.Code, raw)

		var resp domain.AppError
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "INVALID_URL", resp.Code, raw)
	}
}

// A public HTTPS endpoint still passes — the guard must not break the
// legitimate case.
func TestWebhook_AllowsPublicHTTPSTarget(t *testing.T) {
	prev := httpsafe.AllowsPrivateTargets()
	httpsafe.SetAllowPrivateTargets(false)
	t.Cleanup(func() { httpsafe.SetAllowPrivateTargets(prev) })

	_, router, _ := setupWebhookRouter()
	body := map[string]any{
		"url":    "https://93.184.216.34/hook",
		"events": []string{"message.received"},
	}
	w := performRequest(router, http.MethodPost, "/v1/account-webhooks", body, testWebhookToken)
	assert.Equal(t, http.StatusCreated, w.Code)
}
