package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// WebChatHandler exposes the public webchat widget endpoints. These are the
// only public (no JWT) endpoints in the channel space — they're called
// directly from the browser running on the customer's website.
//
// Security model:
//   - channel_token in the URL path identifies the tenant; it's a ULID and
//     therefore unguessable
//   - CORS is enforced PER-CHANNEL via the channel.config.allowed_origins list
//   - Rate limiting is applied at the router level (configured by the global
//     rate limiter — see middleware.RateLimit). For the webchat we recommend
//     a stricter per-IP+token limit
//   - Visitor IDs are opaque strings chosen by the widget; they are tied to
//     the channel, so a visitor on channel A cannot read messages of channel B
type WebChatHandler struct {
	channelRepo domain.ChannelRepository
	convRepo    domain.ConversationRepository
	msgRepo     domain.MessageRepository
	inboundUC   *usecases.InboundWebhookUseCase
	hub         *Hub
	csatRepo    domain.CSATRepository             // optional — nil disables the CSAT endpoint
	pushUC      *usecases.PushNotificationUseCase // optional — nil skips push fan-out (tests, no-VAPID dev)
	accountRepo domain.AccountRepository          // optional — used to resolve the plan for the branding badge
	planRepo    domain.PlanRepository             // optional — paired with accountRepo
}

// SetPlanResolver wires the account/plan lookups used to decide whether the
// bootstrap response asks the widget to render the "Powered by Replider"
// badge (US-172). Optional — when unset (or either lookup fails) the badge
// is shown, the safe free-tier default.
func (h *WebChatHandler) SetPlanResolver(accountRepo domain.AccountRepository, planRepo domain.PlanRepository) {
	h.accountRepo = accountRepo
	h.planRepo = planRepo
}

// SetPushUseCase wires the push fan-out for inbound webchat messages so
// attendants get OS notifications when a visitor writes in. Optional —
// silently skipped when unset.
func (h *WebChatHandler) SetPushUseCase(uc *usecases.PushNotificationUseCase) {
	h.pushUC = uc
}

func NewWebChatHandler(
	channelRepo domain.ChannelRepository,
	convRepo domain.ConversationRepository,
	msgRepo domain.MessageRepository,
	inboundUC *usecases.InboundWebhookUseCase,
	hub *Hub,
) *WebChatHandler {
	return &WebChatHandler{
		channelRepo: channelRepo,
		convRepo:    convRepo,
		msgRepo:     msgRepo,
		inboundUC:   inboundUC,
		hub:         hub,
	}
}

// SetCSATRepo enables the anonymous CSAT submission endpoint for the
// webchat. When nil, POST /widget/:token/csat responds 503 so an older
// deployment without the CSAT schema still boots.
func (h *WebChatHandler) SetCSATRepo(r domain.CSATRepository) {
	h.csatRepo = r
}

// webchatConfig is the parsed shape of channel.config for type=webchat.
type webchatConfig struct {
	AllowedOrigins []string `json:"allowed_origins"`
	WelcomeMessage string   `json:"welcome_message"`
	PrimaryColor   string   `json:"primary_color"`
	Title          string   `json:"title"`
}

// visitorIDRegex enforces a safe format on caller-supplied visitor IDs to
// prevent injection (the visitor_id is used as the conversation contact_id).
var visitorIDRegex = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// resolveWebChatChannel loads the channel by token, validates type/active,
// and applies CORS. Returns the channel if everything's OK; otherwise writes
// the error response and returns nil.
func (h *WebChatHandler) resolveWebChatChannel(c *gin.Context) (*domain.Channel, *webchatConfig) {
	token := c.Param("channel_token")
	ch, err := h.channelRepo.GetByWebhookToken(c.Request.Context(), token)
	if err != nil || ch == nil {
		c.JSON(http.StatusNotFound, domain.AppError{Code: "CHANNEL_NOT_FOUND", Message: "channel not found"})
		return nil, nil
	}
	if ch.Type != "webchat" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_CHANNEL_TYPE", Message: "endpoint only supports webchat channels"})
		return nil, nil
	}
	if !ch.IsActive {
		c.JSON(http.StatusForbidden, domain.AppError{Code: "CHANNEL_DISABLED", Message: "channel is disabled"})
		return nil, nil
	}

	cfg := parseWebchatConfig(ch.Config)

	// CORS: if allowed_origins is configured, enforce it. If empty, allow any
	// origin (but only return the wildcard during local dev — production
	// channels should always have allowed_origins set).
	origin := c.GetHeader("Origin")
	if len(cfg.AllowedOrigins) > 0 {
		if !originAllowed(origin, cfg.AllowedOrigins) {
			c.JSON(http.StatusForbidden, domain.AppError{Code: "ORIGIN_NOT_ALLOWED", Message: "this origin is not allowed for this channel"})
			return nil, nil
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Vary", "Origin")
	} else {
		c.Header("Access-Control-Allow-Origin", "*")
	}
	c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Content-Type")

	return ch, &cfg
}

func parseWebchatConfig(raw interface{}) webchatConfig {
	var cfg webchatConfig
	b, err := json.Marshal(raw)
	if err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	return cfg
}

func originAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return false
	}
	for _, a := range allowed {
		if strings.EqualFold(strings.TrimRight(a, "/"), strings.TrimRight(origin, "/")) {
			return true
		}
	}
	return false
}

// HandlePreflight responds to CORS preflight (OPTIONS) requests.
//
// @Summary  Webchat CORS preflight
// @Tags     webchat
// @Param    channel_token  path  string  true  "Channel webhook token"
// @Success  204
// @Router   /widget/{channel_token}/messages [options]
func (h *WebChatHandler) HandlePreflight(c *gin.Context) {
	if _, _ = h.resolveWebChatChannel(c); c.IsAborted() || c.Writer.Status() >= 400 {
		return
	}
	c.Status(http.StatusNoContent)
}

// ──────────────────────────────────────────────────────────────────────────
// Public meta endpoint — visitor's first interaction (widget bootstrap).
// ──────────────────────────────────────────────────────────────────────────

type webchatBootstrap struct {
	Title          string `json:"title"`
	WelcomeMessage string `json:"welcome_message"`
	PrimaryColor   string `json:"primary_color"`
	// ShowBranding tells the widget to render the discreet "Powered by
	// Replider" badge. It's on for free/trial accounts (a distribution
	// loop) and removable on any paid plan (US-172).
	ShowBranding bool `json:"show_branding"`
}

// HandleBootstrap returns the visible widget settings (no secrets).
//
// @Summary  Get webchat bootstrap config
// @Tags     webchat
// @Produce  json
// @Param    channel_token  path  string  true  "Channel webhook token"
// @Success  200  {object}  webchatBootstrap
// @Router   /widget/{channel_token}/bootstrap [get]
func (h *WebChatHandler) HandleBootstrap(c *gin.Context) {
	channel, cfg := h.resolveWebChatChannel(c)
	if cfg == nil {
		return
	}
	c.JSON(http.StatusOK, webchatBootstrap{
		Title:          cfg.Title,
		WelcomeMessage: cfg.WelcomeMessage,
		PrimaryColor:   cfg.PrimaryColor,
		ShowBranding:   h.showBranding(c.Request.Context(), channel),
	})
}

// showBranding decides whether the widget should render the "Powered by
// Replider" badge for this channel's account (US-172). The badge shows for
// free/trial accounts and is removed on any paid plan. Fails open to "show"
// — if the resolvers are unwired (tests) or a lookup errors, we keep the
// badge rather than silently stripping free-tier attribution.
func (h *WebChatHandler) showBranding(ctx context.Context, channel *domain.Channel) bool {
	if h.accountRepo == nil || h.planRepo == nil || channel == nil {
		return true
	}
	acc, err := h.accountRepo.GetByID(ctx, channel.AccountID)
	if err != nil || acc == nil || acc.PlanID == nil {
		return true
	}
	plan, err := h.planRepo.GetByID(ctx, *acc.PlanID)
	if err != nil || plan == nil {
		return true
	}
	return !plan.AllowsBrandingRemoval()
}

// ──────────────────────────────────────────────────────────────────────────
// POST /widget/:channel_token/messages — visitor sends a message
// ──────────────────────────────────────────────────────────────────────────

type webchatMessageRequest struct {
	VisitorID string `json:"visitor_id" binding:"required"`
	Text      string `json:"text" binding:"required"`
	Name      string `json:"name"` // optional display name from visitor (e.g. captured form)
}

type webchatMessageResponse struct {
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
}

// HandleSendMessage receives a message from a visitor and processes it
// through the same inbound pipeline used by WhatsApp/Telegram.
//
// @Summary  Visitor sends a message via the webchat widget
// @Tags     webchat
// @Accept   json
// @Produce  json
// @Param    channel_token  path  string                 true   "Channel webhook token"
// @Param    body           body  webchatMessageRequest  true   "Message body"
// @Success  202  {object}  webchatMessageResponse
// @Failure  400  {object}  domain.AppError
// @Failure  404  {object}  domain.AppError
// @Router   /widget/{channel_token}/messages [post]
func (h *WebChatHandler) HandleSendMessage(c *gin.Context) {
	channel, _ := h.resolveWebChatChannel(c)
	if channel == nil {
		return
	}

	var req webchatMessageRequest
	if !bindJSON(c, &req) {
		return
	}

	// Hard limits: visitor IDs are opaque but constrained; messages are size-capped.
	if !visitorIDRegex.MatchString(req.VisitorID) {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_VISITOR_ID", Message: "visitor_id must match ^[A-Za-z0-9._-]{1,128}$"})
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "EMPTY_MESSAGE", Message: "text is required"})
		return
	}
	if len(text) > 4000 {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "MESSAGE_TOO_LONG", Message: "text must be <= 4000 chars"})
		return
	}

	partial, err := h.inboundUC.SaveInbound(c.Request.Context(), channel, usecases.InboundMessageDTO{
		ChannelToken: channel.WebhookToken,
		SenderPhone:  req.VisitorID,
		SenderName:   req.Name,
		MessageType:  "text",
		Content:      text,
	})
	if err != nil {
		slog.Error("webchat: failed to save inbound", "channel_id", channel.ID, "error", err)
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "PROCESSING_ERROR", Message: "failed to save message"})
		return
	}

	// Broadcast inbound to dashboard immediately
	if h.hub != nil {
		payload, _ := json.Marshal(map[string]interface{}{
			"event":           "new_message",
			"conversation_id": partial.ConversationID,
			"message":         partial.Message,
		})
		h.hub.BroadcastToAccount(partial.AccountID, payload)
	}

	// Push so attendants get OS notifications for webchat visitors too
	// (the only channel where the visitor is browser-side and the
	// support team is the only audience that benefits from a ping).
	dispatchInboundPushAsync(h.pushUC, partial)

	// Run AI in background so the visitor doesn't wait. The AI reply is
	// delivered via the long-polling GET endpoint (and via WS to the dashboard).
	if !partial.IsBotPaused {
		go func(p *usecases.InboundPartialResult, content string, accountID string) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			result := h.inboundUC.ProcessAI(ctx, p, content)
			if result == nil || result.Message == nil {
				return
			}
			if h.hub != nil {
				payload, _ := json.Marshal(map[string]interface{}{
					"event":           "new_message",
					"conversation_id": p.ConversationID,
					"message":         result.Message,
				})
				h.hub.BroadcastToAccount(accountID, payload)
			}
		}(partial, text, partial.AccountID)
	}

	c.JSON(http.StatusAccepted, webchatMessageResponse{
		ConversationID: partial.ConversationID,
		MessageID:      partial.Message.ID,
	})
}

// ──────────────────────────────────────────────────────────────────────────
// GET /widget/:channel_token/messages — visitor polls for new messages
// ──────────────────────────────────────────────────────────────────────────

type webchatPollResponse struct {
	Messages []webchatMessageDTO `json:"messages"`
	Now      string              `json:"now"` // server time, used as next "since" cursor
}

type webchatMessageDTO struct {
	ID        string `json:"id"`
	Direction string `json:"direction"` // "inbound" (from visitor) | "outbound" (from agent/AI)
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

// HandlePollMessages returns the messages of a visitor's conversation since a
// given timestamp. Designed for long-poll: clients call repeatedly with the
// last-seen timestamp to receive new messages (including AI replies that came
// after the visitor's POST returned).
//
// @Summary  Poll for new webchat messages
// @Tags     webchat
// @Produce  json
// @Param    channel_token  path   string  true   "Channel webhook token"
// @Param    visitor_id     query  string  true   "Visitor ID"
// @Param    since          query  string  false  "RFC3339 timestamp; only messages created after this are returned"
// @Success  200  {object}  webchatPollResponse
// @Router   /widget/{channel_token}/messages [get]
func (h *WebChatHandler) HandlePollMessages(c *gin.Context) {
	channel, _ := h.resolveWebChatChannel(c)
	if channel == nil {
		return
	}

	visitorID := c.Query("visitor_id")
	if !visitorIDRegex.MatchString(visitorID) {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_VISITOR_ID", Message: "visitor_id is required and must be valid"})
		return
	}

	var since time.Time
	if s := c.Query("since"); s != "" {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_SINCE", Message: "since must be RFC3339"})
			return
		}
		since = t
	}

	// Find the conversation for this (channel, visitor) — without leaking
	// existence to outsiders. If none exists yet, return empty list.
	// FindByContact is the same lookup the inbound pipeline uses.
	conv, err := h.findConv(c.Request.Context(), channel.ID, visitorID)
	if err != nil {
		slog.Error("webchat: failed to find conversation", "error", err)
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "internal error"})
		return
	}
	now := time.Now().UTC()
	if conv == nil {
		c.JSON(http.StatusOK, webchatPollResponse{Messages: []webchatMessageDTO{}, Now: now.Format(time.RFC3339Nano)})
		return
	}

	msgs, err := h.msgRepo.ListByConversationSince(c.Request.Context(), conv.ID, since)
	if err != nil {
		slog.Error("webchat: failed to list messages", "error", err)
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "internal error"})
		return
	}

	out := make([]webchatMessageDTO, 0, len(msgs))
	for _, m := range msgs {
		text := ""
		if m.Content != nil {
			text = *m.Content
		}
		out = append(out, webchatMessageDTO{
			ID:        m.ID,
			Direction: m.Direction,
			Text:      text,
			CreatedAt: m.Timestamp.UTC().Format(time.RFC3339Nano),
		})
	}
	c.JSON(http.StatusOK, webchatPollResponse{Messages: out, Now: now.Format(time.RFC3339Nano)})
}

// findConv looks up the conversation for a (channel, visitor) pair using the
// same scoping as the inbound pipeline (channel.id + contact_id).
func (h *WebChatHandler) findConv(ctx context.Context, channelID, visitorID string) (*domain.Conversation, error) {
	return h.convRepo.FindByContact(ctx, channelID, visitorID)
}

type webchatCSATRequest struct {
	VisitorID string  `json:"visitor_id" binding:"required"`
	Rating    int     `json:"rating" binding:"required,min=1,max=5"`
	Feedback  *string `json:"feedback,omitempty"`
}

// HandleRecordCSAT lets the webchat visitor submit a 1-5 satisfaction
// rating for their own conversation. Unauthenticated but scoped:
//   - channel_token in the URL identifies the tenant
//   - visitor_id must match the existing conversation's contact
//
// If the conversation doesn't exist (no messages exchanged yet) we
// return 404 instead of silently succeeding — that's the only way to
// catch a bug in the widget on the next deploy.
//
// @Summary  Submit a webchat CSAT rating
// @Tags     webchat
// @Accept   json
// @Produce  json
// @Param    channel_token  path  string               true  "Channel webhook token"
// @Param    request        body  webchatCSATRequest   true  "Rating payload"
// @Success  200  {object}  map[string]string  "recorded"
// @Failure  400  {object}  domain.AppError    "code: INVALID_REQUEST"
// @Failure  404  {object}  domain.AppError    "code: CONV_NOT_FOUND"
// @Failure  503  {object}  domain.AppError    "code: CSAT_DISABLED"
// @Router   /widget/{channel_token}/csat [post]
func (h *WebChatHandler) HandleRecordCSAT(c *gin.Context) {
	if h.csatRepo == nil {
		c.JSON(http.StatusServiceUnavailable, domain.AppError{Code: "CSAT_DISABLED", Message: "csat is not configured"})
		return
	}
	channel, _ := h.resolveWebChatChannel(c)
	if channel == nil {
		return
	}

	var req webchatCSATRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_REQUEST", Message: err.Error()})
		return
	}
	if !visitorIDRegex.MatchString(req.VisitorID) {
		c.JSON(http.StatusBadRequest, domain.AppError{Code: "INVALID_VISITOR_ID", Message: "visitor_id is required and must be valid"})
		return
	}

	conv, err := h.findConv(c.Request.Context(), channel.ID, req.VisitorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "internal error"})
		return
	}
	if conv == nil {
		c.JSON(http.StatusNotFound, domain.ErrConversationNotFound)
		return
	}

	if err := h.csatRepo.RecordAnswer(c.Request.Context(), conv.ID, req.Rating, req.Feedback); err != nil {
		c.JSON(http.StatusInternalServerError, domain.AppError{Code: "INTERNAL_ERROR", Message: "failed to record rating"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}
