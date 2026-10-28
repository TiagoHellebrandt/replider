package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// HandleAssumeConversation godoc
// @Summary      Assume a conversation (Human Takeover)
// @Description  Marks the conversation as taken over by the authenticated user.
// @Description  Sets assigned_to, stamps assigned_at=NOW() and pauses the bot atomically.
// @Description  Idempotent; safe to call repeatedly.
// @Tags         conversations
// @Produce      json
// @Param        id path string true "Conversation ID"
// @Success      200 {object} domain.ConversationAssignment
// @Failure      404 {object} domain.AppError "code: CONV_NOT_FOUND"
// @Failure      500 {object} domain.AppError
// @Security     BearerAuth
// @Router       /conversations/{id}/assign [post]
func (h *ConversationHandler) HandleAssumeConversation(c *gin.Context) {
	if h.takeoverUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "TAKEOVER_DISABLED", Message: "takeover is not configured",
		})
		return
	}
	convID := c.Param("id")
	accountID := c.GetString("accountID")
	userID := c.GetString("userID")

	res, err := h.takeoverUC.Assume(c.Request.Context(), convID, accountID, userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res.Assignment)
}

// HandleReleaseConversation godoc
// @Summary      Release a conversation back to the bot
// @Description  Clears assigned_to, assigned_at and un-pauses the bot.
// @Description  Idempotent; safe to call on an already-released conversation.
// @Tags         conversations
// @Produce      json
// @Param        id path string true "Conversation ID"
// @Success      200 {object} domain.ConversationAssignment
// @Failure      404 {object} domain.AppError "code: CONV_NOT_FOUND"
// @Security     BearerAuth
// @Router       /conversations/{id}/release [post]
func (h *ConversationHandler) HandleReleaseConversation(c *gin.Context) {
	if h.takeoverUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "TAKEOVER_DISABLED", Message: "takeover is not configured",
		})
		return
	}
	convID := c.Param("id")
	accountID := c.GetString("accountID")

	res, err := h.takeoverUC.Release(c.Request.Context(), convID, accountID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res.Assignment)
}

// SendHumanMessageRequest carries the minimal text-message shape for
// the takeover reply path. Media attachments still flow through
// POST /conversations/{id}/reply.
type SendHumanMessageRequest struct {
	Content string `json:"content" binding:"required"`
	Type    string `json:"type"`
	// Subject is honoured on EMAIL conversations only.
	Subject string `json:"subject"`
}

// HandleSendHumanMessage godoc
// @Summary      Send a message as the human attendant (takeover mode)
// @Description  Persists the message with role=attendant (user_id as sender),
// @Description  dispatches it via the channel adapter and broadcasts via WebSocket.
// @Description  Fails with CONV_NOT_ASSUMED if the bot is not paused (i.e. nobody
// @Description  has assumed the conversation yet).
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        id path string true "Conversation ID"
// @Param        request body SendHumanMessageRequest true "Message"
// @Success      200 {object} domain.Message
// @Failure      403 {object} domain.AppError "code: CONV_OUTSIDE_24H | CONV_NOT_ASSUMED"
// @Failure      404 {object} domain.AppError "code: CONV_NOT_FOUND"
// @Failure      502 {object} domain.AppError "code: MESSAGE_SEND_FAILED — message saved but channel delivery failed"
// @Security     BearerAuth
// @Router       /conversations/{id}/send-message [post]
func (h *ConversationHandler) HandleSendHumanMessage(c *gin.Context) {
	if h.takeoverUC == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "TAKEOVER_DISABLED", Message: "takeover is not configured",
		})
		return
	}

	var req SendHumanMessageRequest
	if !bindJSON(c, &req) {
		return
	}

	msg, err := h.takeoverUC.SendHumanMessage(c.Request.Context(), usecases.SendHumanMessageDTO{
		ConversationID: c.Param("id"),
		AccountID:      c.GetString("accountID"),
		UserID:         c.GetString("userID"),
		UserName:       c.GetString("userName"),
		Content:        req.Content,
		Type:           req.Type,
		Subject:        req.Subject,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, msg)
}
