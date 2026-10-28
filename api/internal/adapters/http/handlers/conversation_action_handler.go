package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// HandleAssignConversation assigns a conversation to a team member.
func (h *ConversationHandler) HandleAssignConversation(c *gin.Context) {
	accountID := c.GetString("accountID")
	convID := c.Param("id")

	var req struct {
		AssignedTo *string `json:"assigned_to"` // nil = unassign
	}
	if !bindJSON(c, &req) {
		return
	}

	if err := h.convRepo.AssignConversation(c.Request.Context(), convID, accountID, req.AssignedTo); err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to assign conversation"})
		return
	}

	// Broadcast assignment change
	if h.hub != nil {
		event, _ := json.Marshal(map[string]interface{}{
			"event":           "conversation_updated",
			"conversation_id": convID,
			"assigned_to":     req.AssignedTo,
		})
		h.hub.BroadcastToAccount(accountID, event)
	}
	// Public webhook event so tenant integrations can sync the
	// assignment to a CRM ticket owner, Slack channel, etc.
	if h.events != nil {
		h.events.Emit(accountID, "conversation.assigned", map[string]any{
			"conversation_id": convID,
			"assigned_to":     req.AssignedTo,
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "conversation assigned"})
}

// HandleSetPriority sets the priority of a conversation.
func (h *ConversationHandler) HandleSetPriority(c *gin.Context) {
	accountID := c.GetString("accountID")
	convID := c.Param("id")

	var req struct {
		Priority string `json:"priority" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}

	// Validate priority value
	switch req.Priority {
	case "low", "normal", "high", "urgent":
	default:
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_PRIORITY", Message: "priority must be low, normal, high, or urgent"})
		return
	}

	conv, err := h.convRepo.GetByID(c.Request.Context(), convID, accountID)
	if err != nil || conv == nil {
		c.JSON(http.StatusNotFound, domain.ErrConversationNotFound)
		return
	}

	conv.Priority = req.Priority
	if err := h.convRepo.Update(c.Request.Context(), conv); err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to update priority"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "priority updated"})
}

// HandleResolveConversation marks a conversation as resolved.
func (h *ConversationHandler) HandleResolveConversation(c *gin.Context) {
	accountID := c.GetString("accountID")
	convID := c.Param("id")

	conv, err := h.convRepo.GetByID(c.Request.Context(), convID, accountID)
	if err != nil || conv == nil {
		c.JSON(http.StatusNotFound, domain.ErrConversationNotFound)
		return
	}

	now := time.Now()
	conv.Status = domain.ConversationClosed
	conv.ResolvedAt = &now
	if err := h.convRepo.Update(c.Request.Context(), conv); err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to resolve conversation"})
		return
	}

	// Broadcast resolved event
	if h.hub != nil {
		event, _ := json.Marshal(map[string]interface{}{
			"event":           "conversation_updated",
			"conversation_id": convID,
			"status":          "closed",
		})
		h.hub.BroadcastToAccount(accountID, event)
	}
	if h.events != nil {
		h.events.Emit(accountID, "conversation.closed", map[string]any{
			"conversation_id": convID,
			"resolved_at":     now,
		})
	}

	// CSAT: reserve a pending survey slot so the response-rate denominator
	// is correct even if the customer never answers. The actual survey
	// message is shipped by the caller (template/webchat overlay); this
	// row just tracks "we asked". Best-effort — never fail the close on
	// this side-effect.
	if h.csatRepo != nil {
		pending := &domain.CSATRating{
			ID:             ulid.Make().String(),
			AccountID:      accountID,
			ConversationID: convID,
			ContactID:      conv.ContactID,
		}
		if err := h.csatRepo.CreatePending(c.Request.Context(), pending); err != nil {
			// log only — closing the conversation is the main intent
			_ = err
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "conversation resolved"})
}

// HandleReopenConversation godoc
// @Summary      Reopen a resolved conversation
// @Description  Resolve's inverse: status back to open, resolved_at cleared, waiting_since restarted. Idempotent on an already-open thread.
// @Tags         conversations
// @Produce      json
// @Param        id   path      string  true  "Conversation ID"
// @Success      200  {object}  map[string]string
// @Failure      404  {object}  domain.AppError  "code: CONV_NOT_FOUND"
// @Failure      409  {object}  domain.AppError  "code: CONV_ALREADY_OPEN — another open thread exists for this sender"
// @Failure      503  {object}  domain.AppError  "code: CONV_REOPEN_UNAVAILABLE"
// @Security     BearerAuth
// @Router       /conversations/{id}/reopen [patch]
func (h *ConversationHandler) HandleReopenConversation(c *gin.Context) {
	accountID := c.GetString("accountID")
	convID := c.Param("id")

	reopener, ok := h.convRepo.(domain.ConversationReopener)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{Code: "CONV_REOPEN_UNAVAILABLE", Message: "reopen not configured"})
		return
	}
	conv, err := h.convRepo.GetByID(c.Request.Context(), convID, accountID)
	if err != nil || conv == nil {
		c.JSON(http.StatusNotFound, domain.ErrConversationNotFound)
		return
	}
	if conv.Status != domain.ConversationClosed {
		c.JSON(http.StatusOK, gin.H{"message": "conversation already open", "status": string(conv.Status)})
		return
	}
	reopened, err := reopener.Reopen(c.Request.Context(), convID, accountID)
	if err != nil {
		if errors.Is(err, domain.ErrConversationAlreadyOpen) {
			// Explicit 409: the code is not in httpStatusForCode, and
			// respondError would default it to a 400 the app reads as "bad input".
			c.JSON(http.StatusConflict, domain.ErrConversationAlreadyOpen)
			return
		}
		respondError(c, err)
		return
	}
	if !reopened {
		// Lost a race with a concurrent reopen — the thread is open either way.
		c.JSON(http.StatusOK, gin.H{"message": "conversation already open", "status": "open"})
		return
	}
	if h.hub != nil {
		event, _ := json.Marshal(map[string]interface{}{
			"event":           "conversation_updated",
			"conversation_id": convID,
			"status":          "open",
		})
		h.hub.BroadcastToAccount(accountID, event)
	}
	// No tenant webhook: "conversation.reopened" is not in
	// domain.WebhookEventCatalogue, and a name outside the catalogue never
	// fires — adding one is a catalogue change, not a handler change.
	c.JSON(http.StatusOK, gin.H{"message": "conversation reopened", "status": "open"})
}

// CSATAnswerRequest carries a customer's satisfaction rating.
type CSATAnswerRequest struct {
	Rating   int     `json:"rating" binding:"required,min=1,max=5"`
	Feedback *string `json:"feedback,omitempty"`
}

// HandleRecordCSAT stores a customer's satisfaction rating for a given
// conversation. Typically called by the webchat frontend after the user
// picks a star rating, or by a channel-specific reply parser that maps
// "1"-"5" buttons to this endpoint. Authenticated: only the owning
// account can record ratings on its own conversations.
func (h *ConversationHandler) HandleRecordCSAT(c *gin.Context) {
	if h.csatRepo == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{Code: "CSAT_DISABLED", Message: "csat is not configured"})
		return
	}
	accountID := c.GetString("accountID")
	convID := c.Param("id")

	var req CSATAnswerRequest
	if !bindJSON(c, &req) {
		return
	}
	conv, err := h.convRepo.GetByID(c.Request.Context(), convID, accountID)
	if err != nil || conv == nil {
		c.JSON(http.StatusNotFound, domain.ErrConversationNotFound)
		return
	}
	if err := h.csatRepo.RecordAnswer(c.Request.Context(), convID, req.Rating, req.Feedback); err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to record rating"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}

// readReceiptSender is an optional interface for channel adapters that support
// sending read receipts (e.g. WhatsApp blue ticks).
type readReceiptSender interface {
	MarkAsRead(ctx context.Context, channel *domain.Channel, messageID string) error
}

// HandleMarkAsRead marks all unread inbound messages in a conversation as read.
// For WhatsApp channels, also sends read receipts (blue ticks) to the sender.
func (h *ConversationHandler) HandleMarkAsRead(c *gin.Context) {
	accountID := c.GetString("accountID")
	convID := c.Param("id")

	conv, err := h.convRepo.GetByID(c.Request.Context(), convID, accountID)
	if err != nil || conv == nil {
		c.JSON(http.StatusNotFound, domain.ErrConversationNotFound)
		return
	}

	// Get unread inbound messages BEFORE marking them — we need their IDs for read receipts
	unreadMsgs, _ := h.msgRepo.ListUnreadInbound(c.Request.Context(), convID)

	now := time.Now()
	if err := h.msgRepo.MarkConversationRead(c.Request.Context(), convID, now); err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to mark as read"})
		return
	}

	// Best-effort: send read receipts to WhatsApp (blue ticks)
	if len(unreadMsgs) > 0 {
		channel, chErr := h.channelRepo.GetByID(c.Request.Context(), conv.ChannelID)
		if chErr == nil && channel != nil {
			if provider, ok := h.providers[channel.Type]; ok {
				if sender, ok := provider.(readReceiptSender); ok {
					// Send read receipt for the last message only (WhatsApp auto-marks earlier)
					lastMsg := unreadMsgs[len(unreadMsgs)-1]
					if lastMsg.Metadata != nil {
						// Try to extract the original WhatsApp message ID from metadata
						raw, _ := json.Marshal(lastMsg.Metadata)
						var meta map[string]string
						if json.Unmarshal(raw, &meta) == nil && meta["wa_message_id"] != "" {
							_ = sender.MarkAsRead(c.Request.Context(), channel, meta["wa_message_id"])
						}
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "marked as read"})
}

// --- Conversation Notes ---

// SetInboundDebounceRepo wires the "responder agora" action and the pending-run
// read (K1). Optional: nil answers 503 rather than panicking.
func (h *ConversationHandler) SetInboundDebounceRepo(r domain.InboundDebounceRepository) {
	h.debounceRepo = r
}

// HandleAnswerNow serves POST /v1/conversations/:id/ai-now.
//
// The attendant's escape from the silence window: the customer is clearly done
// typing, so stop waiting. It pulls the pending run forward instead of generating
// inline — one code path answers a burst, and it is the worker's.
//
// @Summary     Answer a debounced burst now
// @Description Pulls a waiting AI reply forward. 404 when nothing is waiting (including when the reply is already being generated, which is not something that can be hurried).
// @Tags        Conversations
// @Produce     json
// @Param       id path string true "Conversation ID"
// @Success     202
// @Failure     404 {object} domain.AppError
// @Failure     503 {object} domain.AppError
// @Security    BearerAuth
// @Router      /conversations/{id}/ai-now [post]
func (h *ConversationHandler) HandleAnswerNow(c *gin.Context) {
	if h.debounceRepo == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{
			Code: "DEBOUNCE_UNAVAILABLE", Message: "inbound debounce not configured"})
		return
	}
	// Account-scoped inside the UPDATE: another tenant's conversation id matches
	// zero rows rather than relying on a caller to have checked.
	pulled, err := h.debounceRepo.AnswerNow(c.Request.Context(),
		getAccountID(c), c.Param("id"), time.Now())
	if err != nil {
		respondError(c, err)
		return
	}
	if !pulled {
		c.JSON(http.StatusNotFound, domain.AppError{
			Code:    "DEBOUNCE_NOTHING_PENDING",
			Message: "não há resposta aguardando para esta conversa",
		})
		return
	}
	// 202, not 200: the worker answers on its next tick, so the reply has been
	// scheduled rather than sent. Saying 200 would invite the client to expect a
	// message in the response body.
	c.Status(http.StatusAccepted)
}
