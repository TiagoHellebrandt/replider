package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
)

// ConversationTakeoverUseCase groups the three operations that make up
// Human Takeover: attendant assumes a conversation (pauses the bot),
// attendant sends a reply while in takeover mode, attendant releases
// the conversation back to the bot.
//
// The existing HandleHumanReply / HandleResumeBot handlers stay around
// for backward compat, but all new logic for takeover lives here so the
// Clean Architecture boundary (adapters -> usecases -> domain) is
// respected and the behaviour is unit-testable without Gin.
type ConversationTakeoverUseCase struct {
	convRepo    domain.ConversationRepository
	msgRepo     domain.MessageRepository
	channelRepo domain.ChannelRepository
	userRepo    domain.UserRepository
	providers   map[string]domain.MessageProvider
	broadcaster domain.EventBroadcaster
	// inflight lets Assume cancel a bot reply that's already streaming for
	// this conversation the instant a human takes over (US-044). Optional;
	// nil = the pause is only honored by the next inbound message.
	inflight *services.InflightAIRegistry
}

// SetInflightRegistry wires the shared in-flight AI registry so that
// assuming a conversation cancels any mid-stream bot reply (US-044). The
// same instance must be wired into InboundWebhookUseCase.
func (uc *ConversationTakeoverUseCase) SetInflightRegistry(r *services.InflightAIRegistry) {
	uc.inflight = r
}

func NewConversationTakeoverUseCase(
	convRepo domain.ConversationRepository,
	msgRepo domain.MessageRepository,
	channelRepo domain.ChannelRepository,
	userRepo domain.UserRepository,
	providers map[string]domain.MessageProvider,
	broadcaster domain.EventBroadcaster,
) *ConversationTakeoverUseCase {
	return &ConversationTakeoverUseCase{
		convRepo:    convRepo,
		msgRepo:     msgRepo,
		channelRepo: channelRepo,
		userRepo:    userRepo,
		providers:   providers,
		broadcaster: broadcaster,
	}
}

// AssumeResult carries the post-takeover state so the handler can
// return it to the client without a second roundtrip.
type AssumeResult struct {
	Assignment *domain.ConversationAssignment
}

// Assume marks the conversation as taken over by userID. Idempotent:
// calling it twice with the same userID leaves the state identical
// (assigned_at gets a fresh timestamp, but that's acceptable and
// simpler than branching on "already yours"). Pauses the bot
// atomically so the next inbound message sees is_bot_paused=TRUE.
func (uc *ConversationTakeoverUseCase) Assume(ctx context.Context, conversationID, accountID, userID string) (*AssumeResult, error) {
	// Ownership gate: GetByID scopes by account_id, so a user from
	// another account hitting this endpoint gets CONV_NOT_FOUND
	// rather than a generic 500 or, worse, a successful write.
	conv, err := uc.convRepo.GetByID(ctx, conversationID, accountID)
	if err != nil {
		return nil, fmt.Errorf("lookup conversation: %w", err)
	}
	if conv == nil {
		return nil, domain.ErrConversationNotFound
	}

	if err := uc.convRepo.AssumeConversation(ctx, conversationID, accountID, userID); err != nil {
		return nil, fmt.Errorf("assume conversation: %w", err)
	}

	// US-044: bot_paused is now committed; cancel any AI reply that's
	// already mid-stream for this conversation so the customer doesn't get
	// a bot message on a thread a human just took over. No-op if nothing is
	// in flight.
	if uc.inflight.Cancel(conversationID) {
		slog.Info("canceled in-flight AI reply on takeover", "conversation_id", conversationID)
	}

	assignment, err := uc.convRepo.GetAssignment(ctx, conversationID, accountID)
	if err != nil {
		return nil, fmt.Errorf("load assignment: %w", err)
	}
	uc.broadcastAssignment(accountID, assignment)
	return &AssumeResult{Assignment: assignment}, nil
}

// Release returns the conversation to the bot. Always sets bot_paused
// back to FALSE and clears the assignment, even if the caller wasn't
// the one who assumed it (admins can take over a teammate's stuck
// conversation to resume the bot). Idempotent.
func (uc *ConversationTakeoverUseCase) Release(ctx context.Context, conversationID, accountID string) (*AssumeResult, error) {
	conv, err := uc.convRepo.GetByID(ctx, conversationID, accountID)
	if err != nil {
		return nil, fmt.Errorf("lookup conversation: %w", err)
	}
	if conv == nil {
		return nil, domain.ErrConversationNotFound
	}

	if err := uc.convRepo.ReleaseConversation(ctx, conversationID, accountID); err != nil {
		return nil, fmt.Errorf("release conversation: %w", err)
	}

	assignment, err := uc.convRepo.GetAssignment(ctx, conversationID, accountID)
	if err != nil {
		return nil, fmt.Errorf("load assignment: %w", err)
	}
	uc.broadcastAssignment(accountID, assignment)
	return &AssumeResult{Assignment: assignment}, nil
}

// SendHumanMessageDTO is the input to SendHumanMessage. Keeps the
// contract tiny on purpose — media uploads and the full legacy reply
// shape still flow through HandleHumanReply.
type SendHumanMessageDTO struct {
	ConversationID string
	AccountID      string
	UserID         string
	UserName       string
	Content        string
	Type           string // default "text"
	// Subject rides only on EMAIL sends: the composer offers it, and the
	// adapter threads it (empty falls back to the server's Re: derivation).
	Subject string
}

// SendHumanMessage persists the attendant reply, dispatches it through
// the channel adapter (WhatsApp, Telegram…) and broadcasts the new
// message via WebSocket. Requires the conversation to already be in
// takeover mode — the bot-pause gate is enforced both client-side
// (disabled input) and here so stale UI state can't bypass it.
func (uc *ConversationTakeoverUseCase) SendHumanMessage(ctx context.Context, dto SendHumanMessageDTO) (*domain.Message, error) {
	conv, err := uc.convRepo.GetByID(ctx, dto.ConversationID, dto.AccountID)
	if err != nil {
		return nil, fmt.Errorf("lookup conversation: %w", err)
	}
	if conv == nil {
		return nil, domain.ErrConversationNotFound
	}
	msgType := dto.Type
	if msgType == "" {
		msgType = "text"
	}

	// Resolve the channel BEFORE the 24h gate so the window is enforced
	// only where the platform actually imposes it (WhatsApp/Instagram —
	// domain.ChannelHas24hWindow). SMS and Telegram have no upstream
	// window, so a >24h human reply on them must go through. A failed
	// lookup keeps the old fail-open behaviour: no gate, and the
	// dispatch step below skips the send with a warning.
	var channel *domain.Channel
	if uc.channelRepo != nil {
		var chErr error
		channel, chErr = uc.channelRepo.GetByID(ctx, conv.ChannelID)
		if chErr != nil || channel == nil {
			slog.Warn("channel lookup failed on human send",
				"channel_id", conv.ChannelID, "error", chErr)
			channel = nil
		}
	}
	// The assume-first gate exists so a human and the bot never both answer
	// a live chat. EMAIL has no auto-reply at all (the AI only drafts), so
	// demanding a takeover there was pure friction — the composer is always
	// a human writing an email.
	if !conv.IsBotPaused && (channel == nil || channel.Type != "email") {
		return nil, domain.ErrConversationNotAssumed
	}
	if channel != nil && domain.ChannelHas24hWindow(channel.Type) &&
		msgType != "template" && !conv.IsWithin24hWindow() {
		return nil, domain.ErrOutside24hWindow
	}

	convID := dto.ConversationID
	content := dto.Content
	senderName := dto.UserName
	if senderName == "" {
		senderName = "Atendente"
	}
	msg := &domain.Message{
		ID:             ulid.Make().String(),
		ChannelID:      conv.ChannelID,
		ConversationID: &convID,
		From:           dto.UserID,
		SenderName:     &senderName,
		Type:           msgType,
		Content:        &content,
		Direction:      "outbound",
		Timestamp:      time.Now(),
	}
	// Email: thread the reply onto the customer's last message
	// (In-Reply-To / References / "Re: " subject) and honour its Reply-To.
	// Without this the answer lands in the hospital's Outlook as a loose
	// email outside the thread. Best-effort: a failed lookup still sends.
	recipient := conv.ContactID
	if channel != nil && channel.Type == "email" {
		lastInbound, liErr := uc.msgRepo.LastInboundInConversation(ctx, dto.ConversationID)
		if liErr != nil {
			slog.Warn("last inbound lookup failed on human email send",
				"conversation_id", dto.ConversationID, "error", liErr)
		}
		if meta := EmailReplyMetadata(lastInbound, dto.Subject); meta != nil {
			msg.Metadata = meta
		}
		recipient = EmailReplyRecipient(lastInbound, conv.ContactID)
	}
	if _, err := uc.msgRepo.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("persist message: %w", err)
	}

	// Mission 6 analytics: if the human is the FIRST responder (e.g.
	// customer pinged outside business hours, attendant picked up
	// before the bot), stamp first_response_by=human. Idempotent —
	// bot-first conversations leave this a no-op.
	if err := uc.convRepo.SetFirstResponse(ctx, dto.ConversationID, dto.AccountID, "human"); err != nil {
		slog.Warn("set first response (human) failed",
			"conversation_id", dto.ConversationID, "error", err)
	}

	// Clear waiting_since — the customer's wait is answered. Best-effort.
	_ = uc.convRepo.ClearWaitingSince(ctx, dto.ConversationID, dto.AccountID)

	// Broadcast the new message to the dashboard before dispatching to
	// the channel — the optimistic UI pattern the app expects. If the
	// channel send fails the message still shows as sent in the UI but
	// with a best-effort warning in the logs; this matches how
	// HandleHumanReply already behaves and is important so the
	// attendant can see their own message appear immediately.
	if uc.broadcaster != nil {
		payload, _ := json.Marshal(map[string]any{
			"event":           "new_message",
			"conversation_id": dto.ConversationID,
			"message":         msg,
		})
		uc.broadcaster.BroadcastToAccount(dto.AccountID, payload)
	}

	// `channel` was already resolved above for the 24h window gate.
	if channel == nil {
		return msg, nil
	}
	provider, ok := uc.providers[channel.Type]
	if !ok {
		slog.Warn("no provider registered for channel type",
			"channel_type", channel.Type, "channel_id", channel.ID)
		return msg, nil
	}
	if err := provider.SendMessage(ctx, channel, recipient, msg); err != nil {
		// US-009 (review fixes): the customer never got this reply — a
		// silent success left the attendant thinking it was delivered.
		// Mirror the AI path: stamp the row failed, broadcast so the
		// bubble flips, and return the failure to the handler (502).
		reason := err.Error()
		slog.Error("failed to send human message via channel",
			"channel_id", channel.ID, "channel_type", channel.Type,
			"message_id", msg.ID, "error", err)
		if mErr := uc.msgRepo.MarkSendFailed(ctx, msg.ID, reason); mErr != nil {
			slog.Warn("failed to mark human message as send-failed",
				"message_id", msg.ID, "error", mErr)
		}
		uc.broadcastSendFailed(dto.AccountID, dto.ConversationID, msg.ID, reason)
		return msg, domain.ErrMessageSendFailed
	}
	return msg, nil
}

// broadcastSendFailed emits the same message_send_failed event the AI
// reply path broadcasts, so the dashboard flips the optimistic bubble
// to a failed state without a manual refresh.
func (uc *ConversationTakeoverUseCase) broadcastSendFailed(accountID, conversationID, messageID, reason string) {
	if uc.broadcaster == nil {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"event":           "message_send_failed",
		"conversation_id": conversationID,
		"message_id":      messageID,
		"failed_reason":   reason,
	})
	if err != nil {
		return
	}
	uc.broadcaster.BroadcastToAccount(accountID, payload)
}

func (uc *ConversationTakeoverUseCase) broadcastAssignment(accountID string, a *domain.ConversationAssignment) {
	if uc.broadcaster == nil || a == nil {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"event":              "conversation_assignment_changed",
		"conversation_id":    a.ConversationID,
		"assigned_user_id":   a.AssignedTo,
		"assigned_user_name": a.AssignedToName,
		"assigned_at":        a.AssignedAt,
		"bot_paused":         a.IsBotPaused,
	})
	if err != nil {
		return
	}
	uc.broadcaster.BroadcastToAccount(accountID, payload)
}
