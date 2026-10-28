package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// AttachmentHandler serves inbound message attachments.
//
// Two endpoints and a hard rule between them: listing tells you what arrived
// and what is known about it; downloading only works for a file a scanner
// actually cleared.
type AttachmentHandler struct {
	uc   *usecases.MessageAttachmentUseCase
	repo domain.MessageAttachmentRepository
}

func NewAttachmentHandler(
	uc *usecases.MessageAttachmentUseCase,
	repo domain.MessageAttachmentRepository,
) *AttachmentHandler {
	return &AttachmentHandler{uc: uc, repo: repo}
}

// HandleListForMessage serves GET /v1/messages/:id/attachments.
//
// Returns the scan status alongside each file so the client can render the
// difference between "clean", "not scanned" and "infected" instead of
// discovering it by clicking.
//
// @Summary      List a message's attachments
// @Tags         messages
// @Produce      json
// @Param        id path string true "Message ID"
// @Success      200 {array} domain.MessageAttachment
// @Failure      401 {object} domain.AppError
// @Security     BearerAuth
// @Router       /messages/{id}/attachments [get]
func (h *AttachmentHandler) HandleListForMessage(c *gin.Context) {
	items, err := h.repo.ListByMessage(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	// Scope in Go here rather than in SQL because the query is keyed by
	// message: a message id from another account simply yields rows this
	// filter removes, and the result is an empty list rather than a leak.
	accountID := getAccountID(c)
	out := make([]*domain.MessageAttachment, 0, len(items))
	for _, a := range items {
		if a.AccountID == accountID {
			out = append(out, a)
		}
	}
	c.JSON(http.StatusOK, out)
}

// HandleDownload serves GET /v1/attachments/:id/download.
//
// Returns a short-lived signed URL rather than the bytes: the file lives in a
// private bucket and streaming it through the API would put every customer's
// documents through our egress for no benefit.
//
// @Summary      Get a signed download URL for an attachment
// @Tags         messages
// @Produce      json
// @Param        id path string true "Attachment ID"
// @Param        acknowledge_unscanned query bool false "Open a file nobody scanned. Never unlocks an infected file."
// @Success      200 {object} map[string]string
// @Failure      403 {object} domain.AppError "not scanned, or infected"
// @Failure      404 {object} domain.AppError
// @Security     BearerAuth
// @Router       /attachments/{id}/download [get]
func (h *AttachmentHandler) HandleDownload(c *gin.Context) {
	// Explicit opt-in, not a default: the client has to say it understood
	// that nobody checked this file. Never unlocks an infected one.
	acknowledged := c.Query("acknowledge_unscanned") == "true"
	url, err := h.uc.DownloadURL(c.Request.Context(), actorFrom(c), c.Param("id"), acknowledged)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}
