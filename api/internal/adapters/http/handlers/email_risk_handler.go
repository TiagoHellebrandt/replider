package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// EmailRiskHandler exposes the one action a reader can take on a warning.
type EmailRiskHandler struct {
	uc      *usecases.EmailRiskUseCase
	msgRepo domain.MessageRepository
}

func NewEmailRiskHandler(uc *usecases.EmailRiskUseCase, msgRepo domain.MessageRepository) *EmailRiskHandler {
	return &EmailRiskHandler{uc: uc, msgRepo: msgRepo}
}

// HandleMarkLegitimate serves POST /v1/messages/:id/legitimate.
//
// The sender address comes from the STORED message, never from the request
// body: accepting it from the client would let one call vouch for an address
// the caller never received mail from.
//
// @Summary      Vouch for a sender flagged as risky
// @Tags         messages
// @Produce      json
// @Param        id path string true "Message ID"
// @Success      204
// @Failure      404 {object} domain.AppError
// @Security     BearerAuth
// @Router       /messages/{id}/legitimate [post]
func (h *EmailRiskHandler) HandleMarkLegitimate(c *gin.Context) {
	if h.uc == nil || h.msgRepo == nil {
		respondError(c, &domain.AppError{
			Code: "EMAIL_RISK_UNAVAILABLE", Message: "risk checks are not configured"})
		return
	}
	actor := actorFrom(c)
	msg, err := h.msgRepo.GetByID(c.Request.Context(), c.Param("id"), actor.AccountID)
	if err != nil || msg == nil {
		respondError(c, domain.ErrMessageNotFound)
		return
	}
	// `From` carries the contact's own address on an inbound row — the same
	// key the risk checks and the payment memory use. Vouching for an
	// outbound row would allowlist ourselves.
	if msg.From == "" || msg.Direction != "inbound" {
		respondError(c, domain.ErrMessageNotFound)
		return
	}
	if err := h.uc.MarkLegitimate(c.Request.Context(), actor, msg.ID, msg.From); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type reportScamRequest struct {
	// Reason is the reporter's own words, and optional. It is the
	// highest-value thing in the report — "esse fornecedor nunca manda boleto
	// por email" is knowledge no technical check has — but demanding it would
	// cost reports from people in a hurry, and a report with no reason still
	// protects the network.
	Reason string `json:"reason"`
}

// HandleReportScam serves POST /v1/messages/:id/report-scam.
//
// Records this company's verdict and answers with what the platform now makes
// of the sender, including whether anyone else had already said the same.
//
// @Summary      Report a message as a scam
// @Tags         messages
// @Accept       json
// @Produce      json
// @Param        id path string true "Message ID"
// @Param        request body reportScamRequest false "Optional explanation"
// @Success      200 {object} domain.EmailRiskRecord
// @Failure      404 {object} domain.AppError
// @Security     BearerAuth
// @Router       /messages/{id}/report-scam [post]
func (h *EmailRiskHandler) HandleReportScam(c *gin.Context) {
	if h.uc == nil || h.msgRepo == nil {
		respondError(c, &domain.AppError{
			Code: "EMAIL_RISK_UNAVAILABLE", Message: "risk checks are not configured"})
		return
	}
	var req reportScamRequest
	// Body is optional: a report with no explanation still protects the
	// network, so a malformed or absent one must not refuse the report.
	_ = c.ShouldBindJSON(&req)

	actor := actorFrom(c)
	msg, err := h.msgRepo.GetByID(c.Request.Context(), c.Param("id"), actor.AccountID)
	if err != nil || msg == nil || msg.From == "" || msg.Direction != "inbound" {
		respondError(c, domain.ErrMessageNotFound)
		return
	}

	var body string
	if msg.Content != nil {
		body = *msg.Content
	}
	rec, err := h.uc.ReportScam(c.Request.Context(), actor, msg.ID, msg.From, body, req.Reason)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, rec)
}
