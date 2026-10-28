package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-M014. This endpoint activates paid plans from an unauthenticated
// origin, so the tests are about what it REFUSES.
//
// The central fact: AbacatePay publishes its HMAC key. A valid signature
// is therefore something anyone can produce — the `?webhookSecret=` query
// parameter is the only thing that authenticates a delivery.

const testAbacateSecret = "whsec_test_secret"
const testAbacateHMACKey = "public_key_from_the_docs"

// fakeAbacateAdapter implements the verification half of the port with the
// documented scheme, so the handler is exercised against real signatures.
type fakeAbacateAdapter struct {
	parsed *domain.AbacateWebhookEvent
}

func (f *fakeAbacateAdapter) Enabled() bool { return true }

func (f *fakeAbacateAdapter) CreatePixCharge(ctx context.Context, in domain.PixChargeInput) (*domain.PixCharge, error) {
	return nil, nil
}

func (f *fakeAbacateAdapter) CreateCardCheckout(ctx context.Context, in domain.CardCheckoutInput) (string, error) {
	return "", nil
}

func (f *fakeAbacateAdapter) GetChargeStatus(ctx context.Context, id string) (string, error) {
	return "", nil
}

func (f *fakeAbacateAdapter) VerifySignature(payload []byte, header string) bool {
	mac := hmac.New(sha256.New, []byte(testAbacateHMACKey))
	mac.Write(payload)
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(strings.TrimSpace(header)))
}

func (f *fakeAbacateAdapter) VerifyWebhookSecret(provided string) bool {
	return provided != "" && provided == testAbacateSecret
}

func (f *fakeAbacateAdapter) ParseWebhook(payload []byte) (*domain.AbacateWebhookEvent, error) {
	if f.parsed != nil {
		return f.parsed, nil
	}
	// An event the use case ignores, so these tests exercise the gates and
	// the dedup without needing the whole activation stack.
	return &domain.AbacateWebhookEvent{ID: "log_1", Event: "transparent.completed"}, nil
}

func signAbacate(body string) string {
	mac := hmac.New(sha256.New, []byte(testAbacateHMACKey))
	mac.Write([]byte(body))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func abacateRouter(h *PaymentHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/v1/payments/abacatepay-webhook", h.AbacatePayWebhook)
	return r
}

func postAbacate(r *gin.Engine, query, body, signature string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/payments/abacatepay-webhook"+query,
		strings.NewReader(body))
	if signature != "" {
		req.Header.Set("X-Webhook-Signature", signature)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// A correctly SIGNED delivery with no shared secret is still an intruder:
// the signing key is public.
func TestAbacateWebhook_ValidSignatureWithoutTheSecretIs401(t *testing.T) {
	h := &PaymentHandler{}
	h.SetAbacatePay(nil, ignoredEventUC(), &fakeAbacateAdapter{}, nil)
	body := `{"id":"log_1","event":"billing.paid"}`

	w := postAbacate(abacateRouter(h), "", body, signAbacate(body))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_SIGNATURE")
}

func TestAbacateWebhook_WrongSecretIs401(t *testing.T) {
	h := &PaymentHandler{}
	h.SetAbacatePay(nil, ignoredEventUC(), &fakeAbacateAdapter{}, nil)
	body := `{"id":"log_1","event":"billing.paid"}`

	w := postAbacate(abacateRouter(h), "?webhookSecret=nope", body, signAbacate(body))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// Right secret, tampered body: the integrity gate catches it.
func TestAbacateWebhook_TamperedBodyIs401(t *testing.T) {
	h := &PaymentHandler{}
	h.SetAbacatePay(nil, ignoredEventUC(), &fakeAbacateAdapter{}, nil)
	signed := `{"id":"log_1","event":"billing.paid"}`
	tampered := `{"id":"log_1","event":"billing.paid","data":{"metadata":{"account_id":"attacker"}}}`

	w := postAbacate(abacateRouter(h), "?webhookSecret="+testAbacateSecret, tampered, signAbacate(signed))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAbacateWebhook_MissingSignatureIs401(t *testing.T) {
	h := &PaymentHandler{}
	h.SetAbacatePay(nil, ignoredEventUC(), &fakeAbacateAdapter{}, nil)
	body := `{"id":"log_1","event":"billing.paid"}`

	w := postAbacate(abacateRouter(h), "?webhookSecret="+testAbacateSecret, body, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// The happy path, and the retry after it: AbacatePay redelivers, and
// activating a plan twice resets the customer's usage twice.
func TestAbacateWebhook_RetriedDeliveryIsProcessedOnce(t *testing.T) {
	events := &fakeEventClaimer{}
	h := &PaymentHandler{}
	h.SetAbacatePay(nil, ignoredEventUC(), &fakeAbacateAdapter{}, events)
	body := `{"id":"log_1","event":"billing.paid"}`
	r := abacateRouter(h)

	first := postAbacate(r, "?webhookSecret="+testAbacateSecret, body, signAbacate(body))
	require.Equal(t, http.StatusOK, first.Code)

	second := postAbacate(r, "?webhookSecret="+testAbacateSecret, body, signAbacate(body))
	// 200 so AbacatePay stops retrying — but not processed again.
	assert.Equal(t, http.StatusOK, second.Code)
	assert.Equal(t, 1, events.claims, "the second delivery must not re-run the side effects")
	assert.Equal(t, []string{"abacate:log_1"}, events.claimedIDs,
		"ids are namespaced so an AbacatePay id can never collide with a Stripe one")
}

// ignoredEventUC is the real use case with no repos: the fake adapter
// parses an event type it ignores, so nothing is dereferenced.
func ignoredEventUC() *usecases.ProcessAbacatePayWebhookUseCase {
	return usecases.NewProcessAbacatePayWebhookUseCase(nil, nil, nil)
}

// fakeEventClaimer records claims so a retry can be told from a first
// delivery.
type fakeEventClaimer struct {
	claims     int
	claimedIDs []string
	seen       map[string]bool
}

func (f *fakeEventClaimer) ClaimEvent(_ context.Context, id, _ string, _ []byte) (bool, error) {
	if f.seen == nil {
		f.seen = map[string]bool{}
	}
	if f.seen[id] {
		return false, nil
	}
	f.seen[id] = true
	f.claims++
	f.claimedIDs = append(f.claimedIDs, id)
	return true, nil
}

func (f *fakeEventClaimer) IsProcessed(context.Context, string) (bool, error)   { return false, nil }
func (f *fakeEventClaimer) MarkProcessed(context.Context, string, string) error { return nil }
func (f *fakeEventClaimer) ListStuckEvents(context.Context) ([]domain.StripeEventRecord, error) {
	return nil, nil
}
func (f *fakeEventClaimer) CleanupOldEvents(context.Context) (int64, error) { return 0, nil }
