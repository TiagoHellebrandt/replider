package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Electronic signature (§6.11, G7).
//
// Admin-only for everything an operator does, plus ONE public route: the
// provider's callback. That route is the interesting one — see HandleCallback.
type SignatureHandler struct {
	uc *usecases.SignatureUseCase
}

func NewSignatureHandler(uc *usecases.SignatureUseCase) *SignatureHandler {
	return &SignatureHandler{uc: uc}
}

type signatureSettingsRequest struct {
	// APIToken is the customer's own provider key. Empty KEEPS the stored one, so
	// saving the sandbox switch does not wipe the credential.
	APIToken string `json:"api_token"`
	Sandbox  bool   `json:"sandbox"`
}

type signatureSettingsResponse struct {
	*domain.SignatureSettings
	// CallbackURL is what the customer pastes into their provider's webhook
	// settings. Built here because the client does not know the API's own origin.
	CallbackURL string `json:"callback_url,omitempty"`
}

// HandleGetSettings serves GET /v1/signature/settings.
//
// @Summary  The account's signature integration
// @Tags     signature
// @Produce  json
// @Success  200 {object} signatureSettingsResponse
// @Router   /signature/settings [get]
func (h *SignatureHandler) HandleGetSettings(c *gin.Context) {
	settings, err := h.uc.Settings(c.Request.Context(), getAccountID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, signatureSettingsResponse{
		SignatureSettings: settings,
		CallbackURL:       signatureCallbackURL(c, settings.WebhookToken),
	})
}

// HandleSaveSettings serves PUT /v1/signature/settings.
//
// @Summary  Store the account's own provider token
// @Tags     signature
// @Accept   json
// @Produce  json
// @Success  200 {object} signatureSettingsResponse
// @Failure  503 {object} domain.AppError
// @Router   /signature/settings [put]
func (h *SignatureHandler) HandleSaveSettings(c *gin.Context) {
	var req signatureSettingsRequest
	if !bindJSON(c, &req) {
		return
	}
	settings, err := h.uc.SaveSettings(c.Request.Context(), getAccountID(c),
		req.APIToken, req.Sandbox)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, signatureSettingsResponse{
		SignatureSettings: settings,
		CallbackURL:       signatureCallbackURL(c, settings.WebhookToken),
	})
}

// signatureCallbackURL is the inbound URL for this account.
//
// Built from the forwarded scheme and host, the same way the channel webhook URLs
// are: the API does not know its own public origin in a container behind nginx.
func signatureCallbackURL(c *gin.Context, token string) string {
	if token == "" {
		return ""
	}
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host + "/v1/webhooks/signature/" + token
}

type sendSignatureRequest struct {
	ContractID string `json:"contract_id"`
	ObjectPath string `json:"object_path"`
	LeadID     string `json:"lead_id"`
	ContactID  string `json:"contact_id"`

	Title   string                   `json:"title"`
	Signers []domain.SignatureSigner `json:"signers"`

	// ValueCents is what the signature is worth. Zero tracks the document and
	// registers NO sale — a signed document with no value is not a sale.
	ValueCents int64  `json:"value_cents"`
	DeadlineOn string `json:"deadline_on"`
}

// HandleSend serves POST /v1/signature/requests.
//
// @Summary  Send a document for signature
// @Tags     signature
// @Accept   json
// @Produce  json
// @Success  200 {object} domain.SignatureRequest
// @Failure  400 {object} domain.AppError
// @Failure  502 {object} domain.AppError
// @Failure  503 {object} domain.AppError
// @Router   /signature/requests [post]
func (h *SignatureHandler) HandleSend(c *gin.Context) {
	var req sendSignatureRequest
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.uc.Send(c.Request.Context(), getAccountID(c), getUserID(c),
		usecases.SignatureInput{
			ContractID: req.ContractID,
			ObjectPath: req.ObjectPath,
			LeadID:     req.LeadID,
			ContactID:  req.ContactID,
			Title:      req.Title,
			Signers:    req.Signers,
			ValueCents: req.ValueCents,
			DeadlineOn: parseDateOnly(req.DeadlineOn),
		})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HandleList serves GET /v1/signature/requests.
//
// @Summary  Signature requests, newest first
// @Tags     signature
// @Produce  json
// @Success  200 {array} domain.SignatureRequest
// @Router   /signature/requests [get]
func (h *SignatureHandler) HandleList(c *gin.Context) {
	limit, offset := pagination(c)
	list, err := h.uc.List(c.Request.Context(), getAccountID(c), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// HandleGet serves GET /v1/signature/requests/:id.
//
// @Summary  One signature request
// @Tags     signature
// @Param    id path string true "Request ID"
// @Produce  json
// @Success  200 {object} domain.SignatureRequest
// @Failure  404 {object} domain.AppError
// @Router   /signature/requests/{id} [get]
func (h *SignatureHandler) HandleGet(c *gin.Context) {
	out, err := h.uc.Get(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HandleRefresh serves POST /v1/signature/requests/:id/refresh.
//
// The manual half of the same read the callback triggers: asks the provider what
// the truth is. Exists because a webhook can be missed, and a salesperson staring
// at "enviado" on a document the customer signed an hour ago needs a button.
//
// @Summary  Re-read the status from the provider
// @Tags     signature
// @Produce  json
// @Success  200 {object} domain.SignatureRequest
// @Failure  404 {object} domain.AppError
// @Failure  502 {object} domain.AppError
// @Router   /signature/requests/{id}/refresh [post]
func (h *SignatureHandler) HandleRefresh(c *gin.Context) {
	out, err := h.uc.Refresh(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HandleCallback serves POST /v1/webhooks/signature/:token — PUBLIC.
//
// The provider publishes no HMAC and no shared secret, so the unguessable token in
// the path is the only credential. That is why nothing in this handler trusts the
// BODY: it reads two fields to FIND the document and then the use case asks the
// provider what actually happened, with the customer's own API token.
//
// Consequence worth stating: somebody who forges a `doc_signed` callback causes an
// authenticated read that answers "pending", and no sale is registered.
//
// Always 200 on a well-formed request, like the channel webhooks: a provider that
// receives a 4xx retries or disables the hook, and neither helps when the cause is
// a document we do not know about.
//
// @Summary  Provider status callback (public, token in path)
// @Tags     signature
// @Accept   json
// @Produce  json
// @Success  200 {object} map[string]string
// @Router   /webhooks/signature/{token} [post]
func (h *SignatureHandler) HandleCallback(c *gin.Context) {
	// Bounded read: a public endpoint must not let a caller stream something
	// enormous into memory.
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	var payload struct {
		EventType  string `json:"event_type"`
		Token      string `json:"token"`
		ExternalID string `json:"external_id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	err = h.uc.HandleCallback(c.Request.Context(), c.Param("token"),
		usecases.SignatureCallback{
			EventType:  payload.EventType,
			DocToken:   payload.Token,
			ExternalID: payload.ExternalID,
		})
	if err != nil {
		// Logged, not returned: the caller is a third party that cannot act on our
		// errors, and the operator has the refresh button plus this line.
		slog.WarnContext(c.Request.Context(), "signature callback not applied",
			"event", payload.EventType, "error", err)
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// signatureRequestSummary is the compact shape a contract card needs.
type signatureRequestSummary struct {
	ID     string                 `json:"id"`
	Title  string                 `json:"title"`
	Status domain.SignatureStatus `json:"status"`
	// SignedAt and SaleID are what make the card say "assinado e venda
	// registrada" rather than just "assinado".
	SignedAt *time.Time `json:"signed_at,omitempty"`
	SaleID   string     `json:"sale_id,omitempty"`
	Signers  int        `json:"signers"`
}

// HandleListForContract serves GET /v1/contracts/:id/signatures.
//
// @Summary  Signature requests for one contract
// @Tags     signature
// @Param    id path string true "Contract ID"
// @Produce  json
// @Success  200 {array} signatureRequestSummary
// @Router   /contracts/{id}/signatures [get]
func (h *SignatureHandler) HandleListForContract(c *gin.Context) {
	list, err := h.uc.ListByContract(c.Request.Context(), getAccountID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]signatureRequestSummary, 0, len(list))
	for _, req := range list {
		out = append(out, signatureRequestSummary{
			ID: req.ID, Title: req.Title, Status: req.Status,
			SignedAt: req.SignedAt, SaleID: strings.TrimSpace(req.SaleID),
			Signers: len(req.Signers),
		})
	}
	c.JSON(http.StatusOK, out)
}
