package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
)

// Volume sending setup (hybrid model, 2026-08-25).
//
// One-by-one approvals always leave through the customer's own mailbox; the
// endpoints here exist so an account that wants BATCH approvals can register
// its domain with the platform's bulk provider and publish the records the
// provider asks for. Until that is done, batch approvals refuse with
// BULK_SENDING_NOT_READY — the per-message provider cost only exists for
// accounts that opted in.

// SetBulkSending wires the bulk provider. Optional setter, same pattern as
// SetEmailDNSVerifier: nil (or an unconfigured provider) answers 503.
func (h *ChannelHandler) SetBulkSending(p domain.BulkDomainProvider) { h.bulkProvider = p }

type bulkSendingResponse struct {
	// Enabled: the domain is registered with the provider.
	Enabled bool `json:"enabled"`
	// Ready: registered AND verified — batch approvals will go through.
	Ready      bool       `json:"ready"`
	Status     string     `json:"status,omitempty"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
	// Records the customer must publish; empty once verified.
	Records []domain.BulkDNSRecord `json:"records,omitempty"`
	Domain  string                 `json:"domain,omitempty"`
}

// HandleEnableBulkSending serves POST /v1/channels/:id/bulk-sending/enable.
//
// Registers the channel's sending domain with the bulk provider (idempotent:
// an already-registered domain is reused) and returns the DNS records to
// publish.
//
// @Summary  Enable volume sending for an email channel's domain
// @Tags     channels
// @Param    id path string true "Channel ID"
// @Success  200 {object} bulkSendingResponse
// @Failure  404 {object} domain.AppError
// @Failure  503 {object} domain.AppError
// @Router   /channels/{id}/bulk-sending/enable [post]
func (h *ChannelHandler) HandleEnableBulkSending(c *gin.Context) {
	ch, address, sendingDomain, appErr := h.bulkSendingChannel(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	dom, err := h.bulkProvider.CreateDomain(c.Request.Context(), sendingDomain)
	if err != nil {
		respondError(c, err)
		return
	}

	// The registration has to LAND on the mailbox or the approval gate can
	// never open (the dns_verified_at lesson). Failing the request on a
	// failed stamp is right here — unlike the BYO poll, the whole point of
	// this call is the stamp.
	if _, err := h.mailboxRepo.EnsureForChannel(c.Request.Context(), ch.AccountID, ch.ID, address); err != nil {
		respondError(c, err)
		return
	}
	if err := h.mailboxRepo.SetBulkDomain(c.Request.Context(), ch.AccountID, address, dom.ID); err != nil {
		respondError(c, err)
		return
	}
	h.stampBulkVerified(c, ch.AccountID, address, dom)

	c.JSON(http.StatusOK, bulkResponseFor(dom, nil))
}

// HandleBulkSendingStatus serves GET /v1/channels/:id/bulk-sending.
//
// Reports registration + verification state. Each poll also asks the provider
// to re-check the records (their verify is asynchronous), so the customer
// publishes the records, keeps the screen open, and the state flips on its
// own — no separate "verify now" button to know about.
//
// @Summary  Volume sending status for an email channel
// @Tags     channels
// @Param    id path string true "Channel ID"
// @Success  200 {object} bulkSendingResponse
// @Failure  404 {object} domain.AppError
// @Failure  503 {object} domain.AppError
// @Router   /channels/{id}/bulk-sending [get]
func (h *ChannelHandler) HandleBulkSendingStatus(c *gin.Context) {
	ch, address, _, appErr := h.bulkSendingChannel(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	mailbox, err := h.mailboxRepo.EnsureForChannel(c.Request.Context(), ch.AccountID, ch.ID, address)
	if err != nil {
		respondError(c, err)
		return
	}
	if mailbox.BulkDomainID == nil || *mailbox.BulkDomainID == "" {
		c.JSON(http.StatusOK, bulkSendingResponse{Enabled: false})
		return
	}

	// Already verified: answer from our own stamp, no provider round trip.
	// Forward-only, same posture as dns_verified_at.
	if mailbox.BulkVerifiedAt != nil {
		c.JSON(http.StatusOK, bulkSendingResponse{
			Enabled: true, Ready: true, Status: "verified",
			VerifiedAt: mailbox.BulkVerifiedAt,
		})
		return
	}

	if err := h.bulkProvider.VerifyDomain(c.Request.Context(), *mailbox.BulkDomainID); err != nil {
		// Best-effort: the read below still reports the truth.
		slog.WarnContext(c.Request.Context(), "bulk sending: verify request failed",
			"channel_id", ch.ID, "error", err)
	}
	dom, err := h.bulkProvider.GetDomain(c.Request.Context(), *mailbox.BulkDomainID)
	if err != nil {
		respondError(c, err)
		return
	}
	h.stampBulkVerified(c, ch.AccountID, address, dom)

	c.JSON(http.StatusOK, bulkResponseFor(dom, mailbox.BulkVerifiedAt))
}

func bulkResponseFor(dom *domain.BulkDomain, verifiedAt *time.Time) bulkSendingResponse {
	resp := bulkSendingResponse{
		Enabled:    true,
		Ready:      dom.Verified(),
		Status:     dom.Status,
		Domain:     dom.Name,
		VerifiedAt: verifiedAt,
	}
	if !dom.Verified() {
		resp.Records = dom.Records
	}
	return resp
}

func (h *ChannelHandler) stampBulkVerified(c *gin.Context, accountID, address string, dom *domain.BulkDomain) {
	if !dom.Verified() {
		return
	}
	if err := h.mailboxRepo.MarkBulkVerified(c.Request.Context(), accountID, address, time.Now()); err != nil {
		slog.WarnContext(c.Request.Context(), "bulk sending: could not stamp verification",
			"error", err)
	}
}

// bulkSendingChannel resolves and gates both routes identically: provider
// wired+configured, mailbox repo wired, channel owned, email, with a
// from_address.
func (h *ChannelHandler) bulkSendingChannel(c *gin.Context) (*domain.Channel, string, string, error) {
	if h.bulkProvider == nil || !h.bulkProvider.Configured() || h.mailboxRepo == nil {
		return nil, "", "", domain.ErrBulkSendingUnavailable
	}
	ch, err := h.channelRepo.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		return nil, "", "", err
	}
	if ch == nil || ch.AccountID != getAccountID(c) {
		return nil, "", "", domain.ErrChannelNotFound
	}
	if ch.Type != "email" {
		return nil, "", "", &domain.AppError{
			Code:    "CHANNEL_NOT_EMAIL",
			Message: "este canal não é de e-mail",
		}
	}
	address := emailFromAddressOf(ch)
	sendingDomain := emailDomainOf(ch)
	if address == "" || sendingDomain == "" {
		return nil, "", "", &domain.AppError{
			Code:    "CHANNEL_MISSING_CONFIG",
			Message: "o canal não tem from_address configurado",
		}
	}
	return ch, address, sendingDomain, nil
}
