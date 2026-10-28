package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Internal CS channel endpoints (canal-interno US-002): the authenticated
// customer user's own support thread with the platform. Split out of
// me_handler.go to keep that file under the size cap.

// SetSupportThreadUC wires the internal support thread use case. Kept as a
// setter so the MeHandler constructor signature stays untouched; nil ⇒ the
// routes respond SUPPORT_NOT_AVAILABLE.
func (h *MeHandler) SetSupportThreadUC(uc *usecases.SupportThreadUseCase) {
	h.supportThreadUC = uc
}

type supportMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

// HandleSendSupportMessage receives a message from the logged-in customer
// user for the platform's internal CS channel.
//
// @Summary  Send a message to Replider support (internal CS channel)
// @Tags     me
// @Accept   json
// @Produce  json
// @Param    body  body  supportMessageRequest  true  "Message body"
// @Success  202  {object}  usecases.SupportThreadResult
// @Failure  400  {object}  domain.AppError
// @Failure  503  {object}  domain.AppError
// @Security BearerAuth
// @Router   /me/support/messages [post]
func (h *MeHandler) HandleSendSupportMessage(c *gin.Context) {
	if h.supportThreadUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{Code: "SUPPORT_NOT_AVAILABLE", Message: "internal support channel is not available"})
		return
	}
	var req supportMessageRequest
	if !bindJSON(c, &req) {
		return
	}
	result, err := h.supportThreadUC.SendMessage(c.Request.Context(), getAccountID(c), getUserID(c), req.Content)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, result)
}

// HandleListSupportMessages returns the logged-in user's own support
// thread, paginated oldest-first. A user who never wrote gets an empty
// page. Another user's thread is unreachable by construction — the thread
// is keyed by the caller's JWT user id.
//
// @Summary  List my support thread messages (internal CS channel)
// @Tags     me
// @Produce  json
// @Param    limit   query  int  false  "Page size (default 50, max 200)"
// @Param    offset  query  int  false  "Offset"
// @Success  200  {object}  usecases.SupportThreadPage
// @Failure  503  {object}  domain.AppError
// @Security BearerAuth
// @Router   /me/support/messages [get]
func (h *MeHandler) HandleListSupportMessages(c *gin.Context) {
	if h.supportThreadUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{Code: "SUPPORT_NOT_AVAILABLE", Message: "internal support channel is not available"})
		return
	}
	limit, offset := pagination(c)
	page, err := h.supportThreadUC.ListMessages(c.Request.Context(), getAccountID(c), getUserID(c), int32(limit), int32(offset)) //nolint:gosec // G115: pagination clamps to [1,200]
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}
