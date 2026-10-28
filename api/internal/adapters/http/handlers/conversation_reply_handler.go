package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/adapters/storage"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// HandleHumanReply godoc
// @Summary      Send a human reply
// @Description  Sends a message from a human agent and automatically pauses the bot (IsBotPaused = true).
//
//	Clears waiting_since, persists the message, broadcasts it via WebSocket, and dispatches the
//	reply through the appropriate channel adapter (WhatsApp, Telegram, etc.) based on the
//	conversation's channel type. Enforces the 24-hour messaging window.
//
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        id   path      string       true  "Conversation ID"
// @Param        request body ReplyRequest true "Reply payload"
// @Success      200  {object}  map[string]string  "Reply sent"
// @Failure      403  {object}  domain.AppError     "code: CONV_OUTSIDE_24H — outside 24h messaging window"
// @Failure      404  {object}  domain.AppError     "code: CONV_NOT_FOUND — conversation not found"
// @Failure      500  {object}  domain.AppError     "Internal server error"
// @Failure      502  {object}  domain.AppError     "code: MESSAGE_SEND_FAILED — reply saved but channel delivery failed"
// @Security     BearerAuth
// @Router       /conversations/{id}/reply [post]
func (h *ConversationHandler) HandleHumanReply(c *gin.Context) {
	convID := c.Param("id")
	acctID := getAccountID(c)
	if acctID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "AUTH_UNAUTHORIZED", Message: "unauthorized"})
		return
	}

	var req ReplyRequest
	if !bindJSON(c, &req) {
		return
	}

	// Default to text if not specified
	if req.MessageType == "" {
		req.MessageType = "text"
	}
	// Must have either a message body or a media URL
	if req.Message == "" && req.MediaURL == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_REQUEST", Message: "message or media_url required"})
		return
	}
	// SSRF guard: the media_url is fetched by our adapters with our network
	// identity, so it is either this account's own upload or a public https
	// URL — never an internal address (see validateOutboundMediaURL).
	if err := validateOutboundMediaURL(acctID, req.MediaURL); err != nil {
		respondError(c, err)
		return
	}

	// Fetch conversation — scoped to authenticated account
	conv, err := h.convRepo.GetByID(c.Request.Context(), convID, acctID)
	if err != nil || conv == nil {
		c.JSON(http.StatusNotFound, domain.ErrConversationNotFound)
		return
	}

	// 24h Window Enforcement.
	//
	// Meta enforces a 24h customer-service window on WhatsApp AND
	// Instagram DMs: free-form text replies are only allowed within 24h
	// of the customer's last inbound message; after that the only
	// legitimate WhatsApp outbound is an approved template. We resolve
	// the channel here (instead of at the previous spot lower in the
	// handler) so we can scope the block by capability
	// (domain.ChannelHas24hWindow):
	//   - WhatsApp/Instagram + non-template + outside window → 403, UI
	//     surfaces the OUTSIDE_24H_WINDOW Kind.
	//   - WhatsApp + template → always allowed (Meta opens a fresh
	//     billable window via the template send).
	//   - SMS / Telegram / other channels → no upstream restriction,
	//     attendant can always reply.
	channel, channelErr := h.channelRepo.GetByID(c.Request.Context(), conv.ChannelID)
	if channelErr != nil || channel == nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{
			Code: "CHANNEL_NOT_FOUND", Message: "could not resolve channel for conversation",
		})
		return
	}
	if domain.ChannelHas24hWindow(channel.Type) && req.MessageType != "template" && !conv.IsWithin24hWindow() {
		c.JSON(http.StatusForbidden, domain.ErrOutside24hWindow)
		return
	}

	// Hybrid Sync: pause the bot when a human replies.
	wasBotRunning := !conv.IsBotPaused
	if err := h.convRepo.SetBotPaused(c.Request.Context(), convID, acctID, true); err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to pause bot"})
		return
	}
	// Takeover audit: only log the first human takeover per conversation
	// (when IsBotPaused went false→true). Best-effort; errors are logged
	// but never block the reply path.
	if wasBotRunning && h.auditLogRepo != nil {
		if actorIDVal, ok := c.Get("userID"); ok {
			actorID, _ := actorIDVal.(string)
			payload, _ := json.Marshal(map[string]any{
				"conversation_id": convID,
				"channel_id":      conv.ChannelID,
				"previous_paused": false,
			})
			// Background goroutine uses a detached context so the log
			// write survives the request returning, but bounded with a
			// hard timeout so a stuck DB pool can't keep the goroutine
			// alive across graceful shutdown.
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := h.auditLogRepo.Create(ctx, &domain.AuditLog{
					ID:         ulid.Make().String(),
					AccountID:  acctID,
					ActorID:    &actorID,
					Action:     "conversation.takeover",
					EntityType: "conversation",
					EntityID:   convID,
					Payload:    payload,
					CreatedAt:  time.Now(),
				}); err != nil {
					slog.Warn("takeover audit log failed",
						"conversation_id", convID, "error", err)
				}
			}()
		}
	}

	// Clear waiting_since — this conversation has been answered
	_ = h.convRepo.ClearWaitingSince(c.Request.Context(), convID, acctID)

	// Persist the human reply message — store raw GCS object path (not a signed URL)
	userID, _ := c.Get("userID")
	userName, _ := c.Get("userName")
	now := time.Now()
	content := req.Message
	agentName := "Atendente"
	if name, ok := userName.(string); ok && name != "" {
		agentName = name
	}
	msg := &domain.Message{
		ID:             ulid.Make().String(),
		ChannelID:      conv.ChannelID,
		ConversationID: &convID,
		From:           userID.(string),
		SenderName:     &agentName,
		Type:           req.MessageType,
		Content:        &content,
		Direction:      "outbound",
		Timestamp:      now,
	}
	if req.MediaURL != "" {
		msg.MediaURL = &req.MediaURL
	}
	// Email: thread onto the customer's last message and honour its
	// Reply-To (same rule as ConversationTakeoverUseCase.SendHumanMessage).
	recipient := conv.ContactID
	if channel.Type == "email" {
		lastInbound, liErr := h.msgRepo.LastInboundInConversation(c.Request.Context(), convID)
		if liErr != nil {
			slog.Warn("last inbound lookup failed on human email reply",
				"conversation_id", convID, "error", liErr)
		}
		if meta := usecases.EmailReplyMetadata(lastInbound, ""); meta != nil {
			msg.Metadata = meta
		}
		recipient = usecases.EmailReplyRecipient(lastInbound, conv.ContactID)
	}

	if _, err := h.msgRepo.Create(c.Request.Context(), msg); err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to save reply"})
		return
	}

	// Broadcast via WebSocket — resolve a loadable media URL for dashboard
	// clients before broadcasting. This is the DASHBOARD's copy, so it gets
	// the media-proxy URL; the channel's copy below is a different question
	// and gets a different answer.
	broadcastMsg := *msg
	if msg.MediaURL != nil {
		if url, ok := h.mediaURLFor(c.Request.Context(), acctID, *msg.MediaURL); ok {
			broadcastMsg.MediaURL = &url
		}
	}
	if h.hub != nil {
		event, _ := json.Marshal(map[string]interface{}{
			"event":           "new_message",
			"conversation_id": convID,
			"message":         broadcastMsg,
		})
		h.hub.BroadcastToAccount(acctID, event)
	}

	// Dispatch via the correct channel adapter.
	// For GCS media, generate a 1-hour signed URL so WhatsApp/Telegram can
	// download it. Deliberately a GCS signed URL and NOT the media proxy: the
	// fetcher here is Meta's server, not a browser, so the CORS problem the
	// proxy exists for does not apply — and pulling the bytes through our own
	// API just to hand them to Meta would put every outbound media send on the
	// API's bandwidth for nothing.
	// `channel` was already resolved above for the 24h window check.
	if channel != nil {
		if provider, ok := h.providers[channel.Type]; ok {
			channelMsg := *msg
			if msg.MediaURL != nil && storage.IsGCSPath(*msg.MediaURL) && h.gcsAdapter != nil {
				if signedURL, err := h.gcsAdapter.GenerateReadSignedURL(c.Request.Context(), *msg.MediaURL, time.Hour); err == nil {
					channelMsg.MediaURL = &signedURL
				} else {
					slog.Error("failed to generate signed URL for channel send", "channel_id", channel.ID, "error", err)
				}
			}
			if err := provider.SendMessage(c.Request.Context(), channel, recipient, &channelMsg); err != nil {
				// US-009 (review fixes): the customer never got this reply —
				// don't answer 200 "reply_sent". Mirror the AI path: stamp
				// the row failed, broadcast so every dashboard flips the
				// bubble, and surface the failure to the attendant.
				reason := err.Error()
				slog.Error("failed to send human reply via channel", "channel_id", channel.ID, "channel_type", channel.Type, "message_id", msg.ID, "error", err)
				if h.msgRepo != nil {
					if mErr := h.msgRepo.MarkSendFailed(c.Request.Context(), msg.ID, reason); mErr != nil {
						slog.Warn("failed to mark human reply as send-failed", "message_id", msg.ID, "error", mErr)
					}
				}
				h.broadcastSendFailed(acctID, convID, msg.ID, reason)
				respondError(c, domain.ErrMessageSendFailed)
				return
			}
		} else {
			slog.Warn("no provider registered for channel type", "channel_type", channel.Type, "channel_id", channel.ID)
		}
	}

	// Kick the personality-learning trigger (if configured). The hook
	// itself is async + cooldown-gated; we just hand it enough context.
	if h.personalityHook != nil && conv.AgentID != nil {
		h.personalityHook(*conv.AgentID, acctID)
	}

	c.JSON(http.StatusOK, gin.H{"status": "reply_sent", "bot_paused": true})
}

// broadcastSendFailed tells every dashboard client that an outbound
// message they already rendered optimistically failed at the channel
// send step — same event shape as the AI-reply path in
// InboundWebhookHandler, so the app handles both with one listener.
func (h *ConversationHandler) broadcastSendFailed(accountID, conversationID, messageID, reason string) {
	if h.hub == nil {
		return
	}
	payload, err := json.Marshal(map[string]interface{}{
		"event":           "message_send_failed",
		"conversation_id": conversationID,
		"message_id":      messageID,
		"failed_reason":   reason,
	})
	if err != nil {
		slog.Warn("failed to marshal message_send_failed payload",
			"conversation_id", conversationID, "message_id", messageID, "error", err)
		return
	}
	h.hub.BroadcastToAccount(accountID, payload)
}

// HandleResumeBot godoc
// @Summary      Resume the AI bot
// @Description  Re-enables the AI bot for this conversation (sets IsBotPaused = false).
// @Tags         conversations
// @Produce      json
// @Param        id   path      string  true  "Conversation ID"
// @Success      200  {object}  map[string]string  "Bot resumed"
// @Failure      404  {object}  domain.AppError     "code: CONV_NOT_FOUND — conversation not found"
// @Failure      500  {object}  domain.AppError     "Internal server error"
// @Security     BearerAuth
// @Router       /conversations/{id}/resume-bot [patch]

// HandleResumeBot godoc
// @Summary      Resume the AI bot
// @Description  Re-enables the AI bot for this conversation (sets IsBotPaused = false).
// @Tags         conversations
// @Produce      json
// @Param        id   path      string  true  "Conversation ID"
// @Success      200  {object}  map[string]string  "Bot resumed"
// @Failure      404  {object}  domain.AppError     "code: CONV_NOT_FOUND — conversation not found"
// @Failure      500  {object}  domain.AppError     "Internal server error"
// @Security     BearerAuth
// @Router       /conversations/{id}/resume-bot [patch]
func (h *ConversationHandler) HandleResumeBot(c *gin.Context) {
	convID := c.Param("id")
	acctID := getAccountID(c)
	if acctID == "" {
		c.JSON(http.StatusUnauthorized, domain.AppError{Code: "AUTH_UNAUTHORIZED", Message: "unauthorized"})
		return
	}

	conv, err := h.convRepo.GetByID(c.Request.Context(), convID, acctID)
	if err != nil || conv == nil {
		c.JSON(http.StatusNotFound, domain.ErrConversationNotFound)
		return
	}

	if err := h.convRepo.SetBotPaused(c.Request.Context(), convID, acctID, false); err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to resume bot"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "bot_resumed", "bot_paused": false})
}

// HandleListMessages godoc
// @Summary      List messages for a conversation
// @Description  Returns messages for a conversation, ordered by created_at ASC.
//               GCS media_url fields are resolved to 30-minute signed read URLs on each request.
// @Tags         conversations
// @Produce      json
// @Param        id     path  string true  "Conversation ID"
// @Param        limit  query int    false "Maximum number of results (1-200)" default(100)
// @Param        offset query int    false "Number of results to skip" default(0)
// @Success      200  {array}   domain.Message
// @Failure      404  {object}  domain.AppError
// @Security     BearerAuth
// @Router       /conversations/{id}/messages [get]
