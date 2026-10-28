package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/httpsafe"
	"github.com/replider/api/internal/core/domain"
)

// WebhookTester is the synchronous test-ping contract. Defined here
// (instead of importing from services) so handlers don't reverse the
// clean-arch direction.
type WebhookTester interface {
	DeliverTest(ctx context.Context, sub *domain.AccountWebhook) (int, error)
}

// AccountWebhookHandler manages tenant-managed outbound webhook
// subscriptions. Lets each account wire integrations (Zapier, n8n,
// CRMs) directly without a per-vendor adapter on our side.
//
// The secret is generated server-side on Create, returned ONCE in
// the response, and never echoed again — same pattern as Stripe API
// keys. Consumers store it on their end; if lost, the only recovery
// is to delete + recreate the webhook.
type AccountWebhookHandler struct {
	repo   domain.AccountWebhookRepository
	tester WebhookTester
}

func NewAccountWebhookHandler(repo domain.AccountWebhookRepository) *AccountWebhookHandler {
	return &AccountWebhookHandler{repo: repo}
}

// SetTester wires the synchronous tester used by HandleTest. Without
// it the test endpoint returns 503 — the rest of CRUD keeps working
// so dev wiring without the dispatcher still functions.
func (h *AccountWebhookHandler) SetTester(t WebhookTester) {
	h.tester = t
}

type webhookCreateRequest struct {
	URL         string   `json:"url" binding:"required"`
	Events      []string `json:"events" binding:"required"`
	Description string   `json:"description"`
}

type webhookUpdateRequest struct {
	URL         string   `json:"url"`
	Events      []string `json:"events"`
	IsActive    *bool    `json:"is_active"`
	Description string   `json:"description"`
}

// HandleList returns every webhook for the calling account, including
// auto-disabled rows (admin needs visibility to triage).
// @Summary  List outbound webhooks
// @Tags     webhooks
// @Produce  json
// @Security BearerAuth
// @Success  200  {array}  domain.AccountWebhook
// @Router   /webhooks [get]
func (h *AccountWebhookHandler) HandleList(c *gin.Context) {
	accountID := c.GetString("accountID")
	rows, err := h.repo.ListByAccount(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, err)
		return
	}
	// Mask secrets in list responses — they were already returned at
	// create time, this endpoint must never leak them again.
	out := make([]map[string]any, 0, len(rows))
	for _, w := range rows {
		out = append(out, webhookDigest(w))
	}
	c.JSON(http.StatusOK, out)
}

// HandleCreate provisions a new webhook + returns the secret ONCE.
// @Summary  Create outbound webhook
// @Tags     webhooks
// @Accept   json
// @Produce  json
// @Security BearerAuth
// @Param    body  body  webhookCreateRequest  true  "URL + events"
// @Success  201
// @Router   /webhooks [post]
func (h *AccountWebhookHandler) HandleCreate(c *gin.Context) {
	accountID := c.GetString("accountID")
	var req webhookCreateRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := validateWebhookURL(req.URL); err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_URL", Message: err.Error()})
		return
	}
	events, err := validateWebhookEvents(req.Events)
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_EVENTS", Message: err.Error()})
		return
	}
	secret, err := mintWebhookSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL", Message: "failed to mint secret"})
		return
	}
	var desc *string
	if d := strings.TrimSpace(req.Description); d != "" {
		desc = &d
	}
	w := &domain.AccountWebhook{
		ID:          ulid.Make().String(),
		AccountID:   accountID,
		URL:         req.URL,
		Secret:      secret,
		Events:      events,
		IsActive:    true,
		Description: desc,
		CreatedAt:   time.Now(),
	}
	saved, err := h.repo.Create(c.Request.Context(), w)
	if err != nil {
		respondError(c, err)
		return
	}
	// Return secret in PLAIN this one time. Mark explicit so callers
	// notice and persist it.
	resp := webhookDigest(saved)
	resp["secret"] = secret
	resp["secret_warning"] = "this is the only time the secret is returned in plaintext — store it securely now"
	c.JSON(http.StatusCreated, resp)
}

// HandleUpdate edits an existing webhook. Cannot rotate the secret
// here — to rotate, delete + recreate.
// @Summary  Update outbound webhook
// @Tags     webhooks
// @Router   /webhooks/{id} [put]
func (h *AccountWebhookHandler) HandleUpdate(c *gin.Context) {
	accountID := c.GetString("accountID")
	id := c.Param("id")
	existing, err := h.repo.GetByID(c.Request.Context(), accountID, id)
	if err != nil {
		respondError(c, err)
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "NOT_FOUND", Message: "webhook not found"})
		return
	}
	var req webhookUpdateRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.URL != "" {
		if err := validateWebhookURL(req.URL); err != nil {
			c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_URL", Message: err.Error()})
			return
		}
		existing.URL = req.URL
	}
	if len(req.Events) > 0 {
		events, err := validateWebhookEvents(req.Events)
		if err != nil {
			c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_EVENTS", Message: err.Error()})
			return
		}
		existing.Events = events
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}
	if req.Description != "" {
		d := req.Description
		existing.Description = &d
	}
	saved, err := h.repo.Update(c.Request.Context(), existing)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, webhookDigest(saved))
}

// HandleDelete removes a webhook permanently.
// @Summary  Delete outbound webhook
// @Tags     webhooks
// @Router   /webhooks/{id} [delete]
func (h *AccountWebhookHandler) HandleDelete(c *gin.Context) {
	accountID := c.GetString("accountID")
	id := c.Param("id")
	if err := h.repo.Delete(c.Request.Context(), accountID, id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// HandleTest fires a synthetic `webhook.test` event at the
// subscription synchronously. Lets the operator confirm reachability
// + signature handling before flipping the row on for real traffic.
// Returns the consumer's HTTP status so the dashboard can show
// success / failure inline.
// @Summary  Send synthetic test event
// @Tags     webhooks
// @Router   /webhooks/{id}/test [post]
func (h *AccountWebhookHandler) HandleTest(c *gin.Context) {
	if h.tester == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code:    "WEBHOOK_TEST_DISABLED",
			Message: "test ping not configured on this deployment",
		})
		return
	}
	accountID := c.GetString("accountID")
	id := c.Param("id")
	w, err := h.repo.GetByID(c.Request.Context(), accountID, id)
	if err != nil {
		respondError(c, err)
		return
	}
	if w == nil {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "NOT_FOUND", Message: "webhook not found"})
		return
	}
	status, err := h.tester.DeliverTest(c.Request.Context(), w)
	if err != nil {
		c.JSON(http.StatusBadGateway, domain.AppError{
			Code:    "WEBHOOK_TEST_FAILED",
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status_code": status,
		"ok":          status >= 200 && status < 300,
	})
}

// webhookDigest returns the dashboard view of a webhook — every field
// EXCEPT the secret. Keep this centralised so we never leak the
// secret by accident in a list or update response.
func webhookDigest(w *domain.AccountWebhook) map[string]any {
	return map[string]any{
		"id":                w.ID,
		"url":               w.URL,
		"events":            w.Events,
		"is_active":         w.IsActive,
		"description":       w.Description,
		"created_at":        w.CreatedAt,
		"updated_at":        w.UpdatedAt,
		"last_fired_at":     w.LastFiredAt,
		"last_succeeded_at": w.LastSucceededAt,
		"last_error":        w.LastError,
		"failure_count":     w.FailureCount,
	}
}

func validateWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	// SSRF guard (US-F011): the transport-level check happens again at dial
	// time, but rejecting here gives the user an actionable error instead of
	// a webhook row that can only ever fail.
	if err := httpsafe.ValidateURL(raw); err != nil {
		return errInvalidURL("URL não pode apontar para a rede interna")
	}
	// HTTPS-only outside dev. localhost is allowed for development; in
	// prod the env validator panics with HTTP, so we don't double-
	// enforce here — but we DO reject obvious shenanigans like file://
	// or javascript: URLs that would otherwise reach Dispatcher.
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		// Allow http only on localhost / 127.0.0.1 — anywhere else
		// and a transient interception risks leaking customer messages.
		host := u.Hostname()
		if host == "localhost" || host == "127.0.0.1" {
			return nil
		}
		return errURLNotHTTPS
	default:
		return errURLNotHTTPS
	}
}

var errURLNotHTTPS = errInvalidURL("URL deve usar HTTPS")

type errInvalidURL string

func (e errInvalidURL) Error() string { return string(e) }

func validateWebhookEvents(events []string) ([]string, error) {
	// The catalogue in the domain is the single source of truth (J2). It used
	// to be a map here AND a set of constants in the dispatcher, with a comment
	// asking whoever added an event to update both — which is exactly the kind
	// of instruction that gets missed, and the failure it produces is a webhook
	// that looks configured and never fires.
	trimmed := make([]string, 0, len(events))
	for _, e := range events {
		trimmed = append(trimmed, strings.TrimSpace(e))
	}
	if err := domain.ValidateWebhookEvents(trimmed); err != nil {
		var appErr *domain.AppError
		if errors.As(err, &appErr) {
			return nil, errInvalidURL(appErr.Message)
		}
		return nil, err
	}

	out := make([]string, 0, len(trimmed))
	seen := map[string]struct{}{}
	for _, e := range trimmed {
		if _, dup := seen[e]; dup {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	return out, nil
}

// mintWebhookSecret generates 32 bytes of randomness and hex-encodes
// to 64 chars. Long enough for HMAC-SHA256 + comfortably above
// brute-force thresholds; hex (not base64) to keep it copy-paste safe
// across shells and config files.
func mintWebhookSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// HandleListEvents serves GET /v1/webhooks/events.
//
// The catalogue with descriptions, so somebody wiring an integration can read
// what each event means rather than guessing from its name — "sale.registered"
// alone does not say whether it fires on an AI-extracted sale nobody confirmed.
//
// @Summary     Subscribable webhook events
// @Description Every event name a subscription may name, with what it means.
// @Tags        Webhooks
// @Produce     json
// @Success     200 {array} domain.WebhookEventInfo
// @Security    BearerAuth
// @Router      /account-webhooks/events [get]
func (h *AccountWebhookHandler) HandleListEvents(c *gin.Context) {
	c.JSON(http.StatusOK, domain.WebhookEventCatalogue())
}
