package handlers

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/adapters/observability"
	"github.com/replider/api/internal/adapters/payment"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

type PaymentHandler struct {
	checkoutUC      *usecases.CreateCheckoutSessionUseCase
	webhookUC       *usecases.ProcessStripeWebhookUseCase
	purchaseAddonUC *usecases.PurchaseAddonUseCase
	billingPortalUC *usecases.CreateBillingPortalSessionUseCase
	stripeAdpt      *payment.StripeAdapter
	appBaseURL      string

	// AbacatePay (BRL PIX + card) — wired via SetAbacatePay, so the
	// constructor signature and its test call sites stay untouched. Nil
	// when AbacatePay is not configured (transparent checkout then 503s
	// only its Abacate-specific routes; the Stripe path is unaffected).
	transparentUC    *usecases.CreateTransparentCheckoutUseCase
	abacateWebhookUC *usecases.ProcessAbacatePayWebhookUseCase
	abacateAdpt      domain.AbacatePayAdapter
	// eventRepo dedups deliveries. The table is shared with Stripe and the
	// ids are namespaced ("abacate:<id>"), so no second table exists to
	// drift from the one that already has crash recovery and cleanup.
	eventRepo domain.StripeEventRepository
}

// SetAbacatePay wires the AbacatePay transparent-checkout + webhook path.
// Optional — a nil transparentUC makes the transparent routes 503.
func (h *PaymentHandler) SetAbacatePay(
	transparentUC *usecases.CreateTransparentCheckoutUseCase,
	abacateWebhookUC *usecases.ProcessAbacatePayWebhookUseCase,
	abacateAdpt domain.AbacatePayAdapter,
	eventRepo domain.StripeEventRepository,
) {
	h.transparentUC = transparentUC
	h.abacateWebhookUC = abacateWebhookUC
	h.abacateAdpt = abacateAdpt
	h.eventRepo = eventRepo
}

func NewPaymentHandler(
	checkoutUC *usecases.CreateCheckoutSessionUseCase,
	webhookUC *usecases.ProcessStripeWebhookUseCase,
	purchaseAddonUC *usecases.PurchaseAddonUseCase,
	billingPortalUC *usecases.CreateBillingPortalSessionUseCase,
	stripeAdpt *payment.StripeAdapter,
	appBaseURL string,
) *PaymentHandler {
	return &PaymentHandler{
		checkoutUC:      checkoutUC,
		webhookUC:       webhookUC,
		purchaseAddonUC: purchaseAddonUC,
		billingPortalUC: billingPortalUC,
		stripeAdpt:      stripeAdpt,
		appBaseURL:      appBaseURL,
	}
}

// validateRedirectURL checks that a redirect URL is safe to use with Stripe.
// It must use https (or http for localhost), match the configured AppBaseURL domain,
// and not contain userinfo (which can be used for open redirect attacks).
func (h *PaymentHandler) validateRedirectURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	// Block userinfo in URL (e.g. https://app.replider.com@attacker.com)
	if parsed.User != nil {
		return false
	}

	// Must have a valid scheme
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return false
	}

	// http is only allowed for localhost
	if parsed.Scheme == "http" && !strings.HasPrefix(parsed.Hostname(), "localhost") {
		return false
	}

	// Allow localhost always (for development)
	if strings.HasPrefix(parsed.Hostname(), "localhost") {
		return true
	}

	// Parse the configured app base URL to extract the allowed root domain.
	// APP_BASE_URL may be api.replider.com while the redirect targets
	// app.replider.com — both share the same root domain and are trusted.
	appURL, err := url.Parse(h.appBaseURL)
	if err != nil {
		return false
	}

	return sameRootDomain(parsed.Hostname(), appURL.Hostname())
}

// sameRootDomain returns true when two hostnames share the same
// registrable domain (e.g. api.replider.com and app.replider.com both
// share replider.com). For single-label hosts (localhost, bare IPs)
// it falls back to exact match.
func sameRootDomain(a, b string) bool {
	if a == b {
		return true
	}
	rootA := rootDomain(a)
	rootB := rootDomain(b)
	return rootA == rootB && rootA != ""
}

// rootDomain extracts the last two labels of a hostname (e.g.
// "api.replider.com" → "replider.com"). Returns "" for single-label
// hosts so they fall through to exact match.
func rootDomain(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-2] + "." + parts[len(parts)-1]
}

type CheckoutRequest struct {
	PlanID     string `json:"plan_id" binding:"required"`
	SuccessURL string `json:"success_url" binding:"required"`
	CancelURL  string `json:"cancel_url" binding:"required"`
	// CouponCode is optional. The use case silently drops invalid /
	// inactive / expired codes so a stale invite link doesn't block
	// the upgrade path.
	CouponCode string `json:"coupon_code,omitempty"`
	// Interval selects monthly vs annual billing (US-169). Optional —
	// empty defaults to monthly. Validated against the supported set so
	// a garbage value is a 400, not a silent monthly charge.
	Interval string `json:"interval,omitempty"`
}

func (h *PaymentHandler) Checkout(c *gin.Context) {
	accountID, exists := c.Get("accountID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req CheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if !h.validateRedirectURL(req.SuccessURL) || !h.validateRedirectURL(req.CancelURL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid redirect URL: must use HTTPS and match the allowed domain"})
		return
	}

	interval, ok := domain.NormalizeBillingInterval(req.Interval)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid interval: must be 'monthly' or 'annual'"})
		return
	}

	dto := usecases.CreateCheckoutSessionDTO{
		AccountID:  accountID.(string),
		PlanID:     req.PlanID,
		SuccessURL: req.SuccessURL,
		CancelURL:  req.CancelURL,
		CouponCode: req.CouponCode,
		Interval:   interval,
	}

	checkoutURL, err := h.checkoutUC.Execute(c.Request.Context(), dto)
	if err != nil {
		// Contract plans (Partner / Enterprise) intentionally have no
		// Stripe price. Surface a structured payload with both contact
		// channels so the Flutter billing screen can render the right
		// CTAs (mailto + wa.me) instead of a generic 500.
		if err == domain.ErrPlanNotPurchasable {
			c.JSON(http.StatusConflict, gin.H{
				"error":   domain.ErrPlanNotPurchasable.Code,
				"message": "Este plano é vendido sob contrato. Fale com um especialista para contratar.",
				"contact": domain.DefaultContactSales(),
			})
			return
		}
		slog.Error("failed to create checkout session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create checkout session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": checkoutURL})
}

// BillingPortalRequest opens a short-lived Stripe-hosted portal where
// the customer manages their card / downloads invoices / cancels.
type BillingPortalRequest struct {
	ReturnURL string `json:"return_url" binding:"required"`
}

func (h *PaymentHandler) BillingPortal(c *gin.Context) {
	accountID, exists := c.Get("accountID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var req BillingPortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if !h.validateRedirectURL(req.ReturnURL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid return URL: must use HTTPS and match the allowed domain"})
		return
	}
	url, err := h.billingPortalUC.Execute(c.Request.Context(), usecases.CreateBillingPortalSessionDTO{
		AccountID: accountID.(string),
		ReturnURL: req.ReturnURL,
	})
	if err != nil {
		slog.Error("failed to create billing portal session", "error", err)
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}

type AddonCheckoutRequest struct {
	PackID     string `json:"pack_id" binding:"required"`
	SuccessURL string `json:"success_url" binding:"required"`
	CancelURL  string `json:"cancel_url" binding:"required"`
}

func (h *PaymentHandler) AddonCheckout(c *gin.Context) {
	accountID, exists := c.Get("accountID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req AddonCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if !h.validateRedirectURL(req.SuccessURL) || !h.validateRedirectURL(req.CancelURL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid redirect URL: must use HTTPS and match the allowed domain"})
		return
	}

	dto := usecases.PurchaseAddonDTO{
		AccountID:  accountID.(string),
		PackID:     req.PackID,
		SuccessURL: req.SuccessURL,
		CancelURL:  req.CancelURL,
	}

	addonURL, err := h.purchaseAddonUC.Execute(c.Request.Context(), dto)
	if err != nil {
		slog.Error("failed to create addon checkout session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create checkout session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": addonURL})
}

// RetryPayment opens the Stripe billing portal so the customer can
// update their card after a failed invoice. Functionally identical to
// BillingPortal but exposed as a dedicated endpoint for clarity.
func (h *PaymentHandler) RetryPayment(c *gin.Context) {
	h.BillingPortal(c)
}

// ListAddonPacks returns the one-time credit packs for the requested
// currency. The route is public (no account context), so — like
// GET /plans — the currency comes from the `?currency=` query param
// (default BRL). The actual charge is still priced from the account's
// own currency server-side in PurchaseAddonUseCase, so a spoofed query
// only changes the displayed list, never the amount billed.
func (h *PaymentHandler) ListAddonPacks(c *gin.Context) {
	currency := strings.ToLower(strings.TrimSpace(c.Query("currency")))
	c.JSON(http.StatusOK, usecases.AddonPacksForCurrency(currency))
}

// TransparentCheckoutRequest drives the country-routed checkout: BRL →
// AbacatePay (PIX embedded or card hosted), non-BRL → Stripe (card).
type TransparentCheckoutRequest struct {
	PlanID     string `json:"plan_id" binding:"required"`
	SuccessURL string `json:"success_url" binding:"required"`
	CancelURL  string `json:"cancel_url" binding:"required"`
	CouponCode string `json:"coupon_code,omitempty"`
	Interval   string `json:"interval,omitempty"`
	// Method selects the BRL rail: "pix" (default) or "card". Ignored on
	// the Stripe (non-BRL) path, which is always card.
	Method string `json:"method,omitempty"`
}

// TransparentCheckout creates a checkout routed by the account's billing
// currency. Returns a discriminated payload: {provider, method, url?, pix?}.
func (h *PaymentHandler) TransparentCheckout(c *gin.Context) {
	if h.transparentUC == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ABACATEPAY_DISABLED"})
		return
	}
	accountID, exists := c.Get("accountID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req TransparentCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if !h.validateRedirectURL(req.SuccessURL) || !h.validateRedirectURL(req.CancelURL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid redirect URL: must use HTTPS and match the allowed domain"})
		return
	}
	interval, ok := domain.NormalizeBillingInterval(req.Interval)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid interval: must be 'monthly' or 'annual'"})
		return
	}
	method := strings.ToLower(strings.TrimSpace(req.Method))
	if method != "" && method != usecases.CheckoutMethodPix && method != usecases.CheckoutMethodCard {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid method: must be 'pix' or 'card'"})
		return
	}

	result, err := h.transparentUC.Execute(c.Request.Context(), usecases.CreateTransparentCheckoutDTO{
		AccountID:  accountID.(string),
		PlanID:     req.PlanID,
		Interval:   interval,
		Method:     method,
		CouponCode: req.CouponCode,
		SuccessURL: req.SuccessURL,
		CancelURL:  req.CancelURL,
	})
	if err != nil {
		if err == domain.ErrPlanNotPurchasable {
			c.JSON(http.StatusConflict, gin.H{
				"error":   domain.ErrPlanNotPurchasable.Code,
				"message": "Este plano é vendido sob contrato. Fale com um especialista para contratar.",
				"contact": domain.DefaultContactSales(),
			})
			return
		}
		slog.Error("failed to create transparent checkout", "error", err)
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// ChargeStatus polls a transparent PIX charge so the app can detect
// payment without waiting on the webhook.
func (h *PaymentHandler) ChargeStatus(c *gin.Context) {
	if h.transparentUC == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ABACATEPAY_DISABLED"})
		return
	}
	chargeID := c.Param("id")
	if chargeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing charge id"})
		return
	}
	status, paid, err := h.transparentUC.GetChargeStatus(c.Request.Context(), chargeID)
	if err != nil {
		slog.Error("failed to check abacatepay charge status", "error", err, "charge_id", chargeID)
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status, "paid": paid})
}

// AbacatePayWebhook receives payment events from AbacatePay.
//
// Three gates, in this order (US-M014):
//
//  1. `?webhookSecret=` — the per-merchant shared secret, compared in
//     constant time. This is the AUTHENTICATION. AbacatePay's HMAC key is
//     published in their docs, so the signature alone would let anyone
//     activate a paid plan on any account.
//  2. `X-Webhook-Signature` — integrity: the body arrived unaltered.
//  3. Event-id claim — AbacatePay retries deliveries, and processing
//     `billing.paid` twice re-activates the plan and resets usage twice.
//
// A failure in 1 or 2 is 401: the caller is not who they claim to be.
func (h *PaymentHandler) AbacatePayWebhook(c *gin.Context) {
	if h.abacateAdpt == nil || h.abacateWebhookUC == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ABACATEPAY_DISABLED"})
		return
	}
	// The secret is read from the X-Webhook-Secret header first; the
	// `?webhookSecret=` query stays accepted because that is where
	// AbacatePay's dashboard puts it, and nginx masks it in the access log
	// (2026-09-01 hardening).
	if !h.abacateAdpt.VerifyWebhookSecret(abacateWebhookSecret(c)) {
		slog.Warn("abacatepay webhook rejected: bad or missing webhookSecret",
			"client_ip", c.ClientIP())
		c.JSON(http.StatusUnauthorized, domain.AppError{
			Code:    "INVALID_SIGNATURE",
			Message: "webhook authentication failed",
		})
		return
	}
	payload, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error reading request body"})
		return
	}
	if !h.abacateAdpt.VerifySignature(payload, c.GetHeader("X-Webhook-Signature")) {
		slog.Warn("abacatepay webhook rejected: signature mismatch",
			"client_ip", c.ClientIP())
		c.JSON(http.StatusUnauthorized, domain.AppError{
			Code:    "INVALID_SIGNATURE",
			Message: "webhook signature verification failed",
		})
		return
	}
	event, err := h.abacateAdpt.ParseWebhook(payload)
	if err != nil {
		slog.Error("abacatepay webhook body could not be decoded", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook body"})
		return
	}

	// Replay protection. Namespaced so an AbacatePay delivery id can never
	// collide with a Stripe event id in the shared idempotency table.
	if h.eventRepo != nil && event.ID != "" {
		claimed, claimErr := h.eventRepo.ClaimEvent(
			c.Request.Context(), "abacate:"+event.ID, event.Event, payload)
		if claimErr != nil {
			slog.Error("abacatepay webhook: claim failed", "error", claimErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process webhook"})
			return
		}
		if !claimed {
			// Already handled. 200 so AbacatePay stops retrying.
			c.Status(http.StatusOK)
			return
		}
	}

	if err := h.abacateWebhookUC.Execute(c.Request.Context(), event); err != nil {
		slog.Error("failed to process abacatepay webhook", "error", err)
		observability.CaptureError(err, map[string]string{
			"component":  "abacatepay_webhook",
			"event_type": event.Event,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process webhook"})
		return
	}
	if h.eventRepo != nil && event.ID != "" {
		if err := h.eventRepo.MarkProcessed(
			c.Request.Context(), "abacate:"+event.ID, event.Event); err != nil {
			slog.Warn("abacatepay webhook: failed to mark processed", "error", err)
		}
	}
	c.Status(http.StatusOK)
}

// Webhook receives events directly from Stripe asynchronously.
func (h *PaymentHandler) Webhook(c *gin.Context) {
	// Stripe requires the raw body to verify the signature
	payload, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)) // 1 MB max
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error reading request body"})
		return
	}

	signature := c.GetHeader("Stripe-Signature")
	if signature == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing Stripe-Signature header"})
		return
	}

	event, err := h.stripeAdpt.ConstructEvent(payload, signature)
	if err != nil {
		// US-015: a failed ConstructEvent means the payload was not signed
		// with our webhook secret — treat it as a hard 400 with an explicit
		// INVALID_SIGNATURE code (not a silent log), so a forged/replayed
		// or misconfigured delivery is unambiguously rejected and Stripe
		// retries against a corrected endpoint.
		slog.Error("webhook signature verification failed", "error", err)
		c.JSON(http.StatusBadRequest, domain.AppError{
			Code:    "INVALID_SIGNATURE",
			Message: "webhook signature verification failed",
		})
		return
	}

	err = h.webhookUC.Execute(c.Request.Context(), event)
	if err != nil {
		slog.Error("failed to process webhook", "error", err)
		observability.CaptureError(err, map[string]string{
			"component":  "stripe_webhook",
			"event_type": string(event.Type),
			"event_id":   event.ID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process webhook"})
		return
	}

	c.Status(http.StatusOK)
}

// abacateWebhookSecret prefers the X-Webhook-Secret header over the legacy
// `?webhookSecret=` query parameter.
func abacateWebhookSecret(c *gin.Context) string {
	if v := strings.TrimSpace(c.GetHeader("X-Webhook-Secret")); v != "" {
		return v
	}
	return c.Query("webhookSecret")
}
