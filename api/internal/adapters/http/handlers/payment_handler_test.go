package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/payment"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestWebhook_InvalidSignatureReturns400 locks in US-015: a payload that
// is not signed with our webhook secret must be rejected with a hard 400
// and an explicit INVALID_SIGNATURE code — never silently accepted or
// logged-and-200'd. We use a real StripeAdapter (with a webhook secret)
// and a bogus Stripe-Signature so ConstructEvent genuinely fails.
func TestWebhook_InvalidSignatureReturns400(t *testing.T) {
	adapter := payment.NewStripeAdapter("", "whsec_test_secret")
	handler := NewPaymentHandler(nil, nil, nil, nil, adapter, "http://localhost:8080")

	router := setupTestRouter()
	router.POST("/v1/payments/webhook", handler.Webhook)

	req := httptest.NewRequest(http.MethodPost, "/v1/payments/webhook",
		bytes.NewReader([]byte(`{"id":"evt_forged","type":"invoice.paid"}`)))
	req.Header.Set("Stripe-Signature", "t=123,v1=deadbeefnotarealsignature")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body domain.AppError
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "INVALID_SIGNATURE", body.Code)
}

// TestWebhook_MissingSignatureReturns400 covers the no-header case.
func TestWebhook_MissingSignatureReturns400(t *testing.T) {
	adapter := payment.NewStripeAdapter("", "whsec_test_secret")
	handler := NewPaymentHandler(nil, nil, nil, nil, adapter, "http://localhost:8080")

	router := setupTestRouter()
	router.POST("/v1/payments/webhook", handler.Webhook)

	req := httptest.NewRequest(http.MethodPost, "/v1/payments/webhook",
		bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListAddonPacks_Success(t *testing.T) {
	handler := NewPaymentHandler(nil, nil, nil, nil, nil, "http://localhost:8080")

	router := setupTestRouter()
	router.GET("/v1/payments/addon-packs", handler.ListAddonPacks)

	w := performRequest(router, http.MethodGet, "/v1/payments/addon-packs", nil, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var packs []usecases.AddonPack
	err := json.Unmarshal(w.Body.Bytes(), &packs)
	assert.NoError(t, err)
	assert.Len(t, packs, 3)
	assert.Equal(t, "addon_p", packs[0].ID)
	assert.Equal(t, "addon_m", packs[1].ID)
	assert.Equal(t, "addon_g", packs[2].ID)
	// Default (no ?currency=) is the BRL ladder.
	assert.Equal(t, "brl", packs[0].Currency)
}

func TestListAddonPacks_USD(t *testing.T) {
	handler := NewPaymentHandler(nil, nil, nil, nil, nil, "http://localhost:8080")

	router := setupTestRouter()
	router.GET("/v1/payments/addon-packs", handler.ListAddonPacks)

	w := performRequest(router, http.MethodGet, "/v1/payments/addon-packs?currency=usd", nil, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var packs []usecases.AddonPack
	err := json.Unmarshal(w.Body.Bytes(), &packs)
	assert.NoError(t, err)
	assert.Len(t, packs, 3)
	for _, p := range packs {
		assert.Equal(t, "usd", p.Currency)
		assert.Positive(t, p.Price)
	}
	assert.Equal(t, "addon_p", packs[0].ID)
}

func TestListPlans_Success(t *testing.T) {
	planRepo := new(MockPlanRepo)

	plans := []*domain.Plan{
		{ID: "plan-starter", Name: "Starter", Price: 9900, Currency: "BRL", IsListed: true},
		{ID: "plan-pro", Name: "Pro", Price: 29900, Currency: "BRL", IsListed: true},
	}
	planRepo.On("ListAll", mock.Anything).Return(plans, nil)

	plansHandler := NewPlansHandler(planRepo, nil)

	router := setupTestRouter()
	router.GET("/v1/plans", plansHandler.HandleListPlans)

	w := performRequest(router, http.MethodGet, "/v1/plans", nil, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []domain.Plan
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, "Starter", resp[0].Name)
	assert.Equal(t, "Pro", resp[1].Name)

	planRepo.AssertExpectations(t)
}

// TestBillingPortalSession_Unauthorized locks in US-175: the canonical
// self-serve endpoint POST /v1/billing/portal-session rejects requests
// with no authenticated account before touching Stripe.
func TestBillingPortalSession_Unauthorized(t *testing.T) {
	handler := NewPaymentHandler(nil, nil, nil, nil, nil, "https://app.replider.com")

	router := setupTestRouter()
	// No auth middleware — handler itself checks for accountID.
	router.POST("/v1/billing/portal-session", handler.BillingPortal)

	body := map[string]string{"return_url": "https://app.replider.com/billing"}
	w := performRequest(router, http.MethodPost, "/v1/billing/portal-session", body, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "Unauthorized", resp["error"])
}

// TestBillingPortalSession_RejectsForeignReturnURL proves the canonical
// route inherits the open-redirect guard: a return_url pointing at an
// attacker domain is rejected with 400 before any Stripe call.
func TestBillingPortalSession_RejectsForeignReturnURL(t *testing.T) {
	handler := NewPaymentHandler(nil, nil, nil, nil, nil, "https://app.replider.com")

	router := setupTestRouter()
	router.POST("/v1/billing/portal-session", func(c *gin.Context) {
		c.Set("accountID", "acc_123")
		handler.BillingPortal(c)
	})

	body := map[string]string{"return_url": "https://evil.example.com/steal"}
	w := performRequest(router, http.MethodPost, "/v1/billing/portal-session", body, "")

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCheckout_Unauthorized(t *testing.T) {
	handler := NewPaymentHandler(nil, nil, nil, nil, nil, "http://localhost:8080")

	router := setupTestRouter()
	// No auth middleware — handler itself checks for accountID
	router.POST("/v1/payments/checkout", handler.Checkout)

	body := map[string]string{
		"success_url": "https://example.com/success",
		"cancel_url":  "https://example.com/cancel",
	}
	w := performRequest(router, http.MethodPost, "/v1/payments/checkout", body, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "Unauthorized", resp["error"])
}
