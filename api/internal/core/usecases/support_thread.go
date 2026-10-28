package usecases

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// ──────────────────────────────── SupportThreadUseCase ────────────────────────────────
//
// Canal interno de CS (US-002): authenticated customer users message the
// platform (Replider/vendedores) from inside their own dashboard. Each
// USER of a customer account gets their own thread (privacy: an admin
// discusses plan/cancellation without their team watching) on the
// platform account's single internal channel. The thread is keyed by
// (internal channel, user_id) — the user's ULID is the conversation
// contact_id — and rides the exact same SaveInbound → ProcessAI pipeline
// as WhatsApp/Telegram, so the platform agent replies first and bot
// pause/takeover work unchanged.

// ErrSupportUnavailable is returned when the internal CS channel isn't
// configured (PLATFORM_ACCOUNT_ID unset / channel missing) or when a
// platform-account user tries to open a thread with themselves.
var ErrSupportUnavailable = &domain.AppError{Code: "SUPPORT_NOT_AVAILABLE", Message: "internal support channel is not available"}

// ErrSupportEmptyMessage rejects blank/whitespace-only content.
var ErrSupportEmptyMessage = &domain.AppError{Code: "EMPTY_MESSAGE", Message: "content is required"}

// ErrSupportMessageTooLong caps a single support message (same 4000-char
// bound as the webchat widget).
var ErrSupportMessageTooLong = &domain.AppError{Code: "MESSAGE_TOO_LONG", Message: "content must be <= 4000 chars"}

// supportMessageMaxLen mirrors the webchat widget's per-message cap.
const supportMessageMaxLen = 4000

// CustomerAccountStamper is the narrow port for stamping the customer
// account on a fresh internal thread. Satisfied by
// *postgres.ConversationRepository — deliberately NOT added to
// domain.ConversationRepository so its many test mocks stay untouched.
type CustomerAccountStamper interface {
	SetCustomerAccount(ctx context.Context, conversationID, customerAccountID string) error
}

// ThreadReadStamper is the narrow port for the US-004 read tracking:
// opening (GET) or writing to (POST) the support thread stamps
// conversations.support_last_read_at, which both re-arms the unread email
// fallback and feeds the widget's unread badge. Satisfied by
// *postgres.SupportNotifyRepository.
type ThreadReadStamper interface {
	MarkThreadRead(ctx context.Context, conversationID string) error
}

// SupportPusher is the narrow port over
// PushNotificationUseCase.NotifySupportMessage: fire-and-forget web push to
// the thread owner's devices. The use case only needs it for the AI reply —
// human replies and takeover sends go through the inapp adapter's
// SendMessage, whose notifier hook covers them (wired in main.go).
type SupportPusher interface {
	NotifySupportMessage(userID, conversationID string, msg *domain.Message)
}

// SupportThreadResult is returned by SendMessage so the widget can render
// the persisted message (and hold the conversation id for follow-ups).
type SupportThreadResult struct {
	ConversationID string          `json:"conversation_id"`
	Message        *domain.Message `json:"message"`
}

type SupportThreadUseCase struct {
	platformAccountID string
	channelRepo       domain.ChannelRepository
	convRepo          domain.ConversationRepository
	msgRepo           domain.MessageRepository
	accountRepo       domain.AccountRepository
	userRepo          domain.UserRepository
	inbound           *InboundWebhookUseCase
	stamper           CustomerAccountStamper
	// broadcaster, when wired (SetBroadcaster), fans every thread message
	// out to BOTH sides in real time (US-003): platform inbox + customer
	// widget. Nil keeps the flow working on GET polling only.
	broadcaster domain.EventBroadcaster
	// readStamper, when wired (SetReadStamper), records the thread owner's
	// last-read timestamp on every GET/POST (US-004). Nil-safe.
	readStamper ThreadReadStamper
	// pusher, when wired (SetSupportPusher), web-pushes the AI reply to the
	// thread owner (US-004). Human/takeover sends are pushed by the inapp
	// adapter's notifier hook instead. Nil-safe.
	pusher SupportPusher
	// dispatch runs the AI turn off the request path so the customer's
	// POST returns immediately (same shape as the webchat handler).
	// Defaults to `go f()`; tests replace it to run synchronously or to
	// just assert the dispatch happened.
	dispatch func(func())
	// processAI is a testhook over inbound.ProcessAI so tests can fake
	// the AI turn without mocking the whole agent pipeline.
	processAI func(ctx context.Context, partial *InboundPartialResult, content string) *ProcessAIResult
}

func NewSupportThreadUseCase(
	platformAccountID string,
	channelRepo domain.ChannelRepository,
	convRepo domain.ConversationRepository,
	msgRepo domain.MessageRepository,
	accountRepo domain.AccountRepository,
	userRepo domain.UserRepository,
	inbound *InboundWebhookUseCase,
	stamper CustomerAccountStamper,
) *SupportThreadUseCase {
	return &SupportThreadUseCase{
		platformAccountID: platformAccountID,
		channelRepo:       channelRepo,
		convRepo:          convRepo,
		msgRepo:           msgRepo,
		accountRepo:       accountRepo,
		userRepo:          userRepo,
		inbound:           inbound,
		stamper:           stamper,
		dispatch:          func(f func()) { go f() },
		processAI:         inbound.ProcessAI,
	}
}

// SetBroadcaster wires the WebSocket hub for real-time delivery on both
// sides of the thread (optional; nil-safe).
func (uc *SupportThreadUseCase) SetBroadcaster(b domain.EventBroadcaster) {
	uc.broadcaster = b
}

// SetReadStamper wires the US-004 read tracking (optional; nil-safe).
func (uc *SupportThreadUseCase) SetReadStamper(s ThreadReadStamper) {
	uc.readStamper = s
}

// SetSupportPusher wires the AI-reply web push (optional; nil-safe).
func (uc *SupportThreadUseCase) SetSupportPusher(p SupportPusher) {
	uc.pusher = p
}

// markRead stamps the owner's read cursor. Best-effort: a failed stamp can
// at worst cause one redundant fallback email, never a lost message.
func (uc *SupportThreadUseCase) markRead(ctx context.Context, conversationID string) {
	if uc.readStamper == nil || conversationID == "" {
		return
	}
	if err := uc.readStamper.MarkThreadRead(ctx, conversationID); err != nil {
		slog.Warn("support thread: mark read failed",
			"conversation_id", conversationID, "error", err)
	}
}

// broadcastBothSides fans one internal-thread message out to its two
// audiences: the platform inbox gets the standard `new_message` event
// (sellers' inbox consumes it unchanged) and the customer account gets the
// `support_message` event carrying the internal-thread marker + thread-owner
// user id, so only that user's widget renders it. Best-effort.
func (uc *SupportThreadUseCase) broadcastBothSides(customerAccountID, userID, conversationID string, msg *domain.Message) {
	if uc.broadcaster == nil || msg == nil {
		return
	}
	platformPayload, err := json.Marshal(map[string]interface{}{
		"event":           "new_message",
		"conversation_id": conversationID,
		"message":         msg,
	})
	if err != nil {
		slog.Warn("support thread: marshal platform broadcast failed",
			"conversation_id", conversationID, "error", err)
	} else {
		uc.broadcaster.BroadcastToAccount(uc.platformAccountID, platformPayload)
	}

	customerPayload, err := domain.SupportMessageEventPayload(conversationID, userID, msg)
	if err != nil {
		slog.Warn("support thread: marshal customer broadcast failed",
			"conversation_id", conversationID, "error", err)
		return
	}
	uc.broadcaster.BroadcastToAccount(customerAccountID, customerPayload)
}

// internalChannel resolves the platform account's single internal channel.
// Returns ErrSupportUnavailable when the feature is off (no platform
// account) or the boot-time EnsureInternalChannelUseCase never ran.
func (uc *SupportThreadUseCase) internalChannel(ctx context.Context, customerAccountID string) (*domain.Channel, error) {
	if uc.platformAccountID == "" || customerAccountID == uc.platformAccountID {
		// Platform users don't get a CS thread with themselves — their
		// side of the channel is the normal platform inbox.
		return nil, ErrSupportUnavailable
	}
	channels, err := uc.channelRepo.ListByAccountID(ctx, uc.platformAccountID)
	if err != nil {
		return nil, err
	}
	for _, ch := range channels {
		if ch.Type == domain.ChannelTypeInternal && ch.IsActive {
			return ch, nil
		}
	}
	return nil, ErrSupportUnavailable
}

// SendMessage persists a customer user's message on their own internal
// thread (creating it on first contact) and dispatches the platform
// agent's AI turn in the background.
//
// New-thread side effects, in order:
//  1. SaveInbound created the conversation (platform rodízio auto-assign
//     already ran inside it, when wired) and persisted the message.
//  2. The customer account is stamped on the row (carteira grouping).
//  3. If the customer account has a linked seller (accounts.seller_user_id),
//     the thread is re-assigned to that seller, overriding the rodízio
//     pick — carteira wins over rotation; NULL seller keeps the rodízio.
//
// Steps 2–3 are best-effort: the user's message is already persisted, so
// a stamp/assign hiccup logs and degrades instead of failing the send.
func (uc *SupportThreadUseCase) SendMessage(ctx context.Context, customerAccountID, userID, content string) (*SupportThreadResult, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, ErrSupportEmptyMessage
	}
	if len(content) > supportMessageMaxLen {
		return nil, ErrSupportMessageTooLong
	}

	channel, err := uc.internalChannel(ctx, customerAccountID)
	if err != nil {
		return nil, err
	}

	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, domain.ErrUnauthorized
	}

	partial, err := uc.inbound.SaveInbound(ctx, channel, InboundMessageDTO{
		// The user's ULID keys the thread: one conversation per
		// (internal channel, user). Name/email become the contact
		// fields the platform inbox displays.
		SenderPhone:    userID,
		SenderName:     user.Name,
		SenderUsername: user.Email,
		MessageType:    "text",
		Content:        content,
	})
	if err != nil {
		return nil, err
	}

	if partial.IsNewConversation {
		if uc.stamper != nil {
			if err := uc.stamper.SetCustomerAccount(ctx, partial.ConversationID, customerAccountID); err != nil {
				slog.Warn("support thread: customer account stamp failed",
					"conversation_id", partial.ConversationID,
					"customer_account_id", customerAccountID, "error", err)
			}
		}
		uc.assignSeller(ctx, partial.ConversationID, customerAccountID)
	}

	// Writing to the thread implies having read it — keeps the unread
	// email fallback from firing at a user who is actively chatting.
	uc.markRead(ctx, partial.ConversationID)

	// Real-time fan-out of the customer's message (US-003): platform
	// inbox sees it live; the customer's other sessions/tabs stay in sync.
	uc.broadcastBothSides(customerAccountID, userID, partial.ConversationID, partial.Message)

	// AI turn off the request path — the reply lands via the WebSocket
	// broadcast below (GET polling remains the fallback).
	uc.dispatch(func() {
		aiCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if aiResult := uc.processAI(aiCtx, partial, content); aiResult != nil && aiResult.Message != nil {
			uc.broadcastBothSides(customerAccountID, userID, partial.ConversationID, aiResult.Message)
			// The AI reply never passes through the inapp adapter's
			// SendMessage (delivery = persist + broadcast), so its web
			// push fires here (US-004).
			if uc.pusher != nil {
				uc.pusher.NotifySupportMessage(userID, partial.ConversationID, aiResult.Message)
			}
		}
	})

	return &SupportThreadResult{
		ConversationID: partial.ConversationID,
		Message:        partial.Message,
	}, nil
}

// assignSeller re-assigns a fresh thread to the customer account's
// carteira seller when one is linked. Best-effort: a lookup/write failure
// leaves the rodízio assignment (or no assignment) in place.
func (uc *SupportThreadUseCase) assignSeller(ctx context.Context, conversationID, customerAccountID string) {
	account, err := uc.accountRepo.GetByID(ctx, customerAccountID)
	if err != nil || account == nil {
		slog.Warn("support thread: customer account lookup failed",
			"customer_account_id", customerAccountID, "error", err)
		return
	}
	if account.SellerUserID == nil || *account.SellerUserID == "" {
		return
	}
	if err := uc.convRepo.AssignConversation(ctx, conversationID, uc.platformAccountID, account.SellerUserID); err != nil {
		slog.Warn("support thread: seller assignment failed",
			"conversation_id", conversationID,
			"seller_user_id", *account.SellerUserID, "error", err)
	}
}

// ListMessages returns the calling user's OWN thread, paginated in
// chronological order. Isolation is structural: the thread is looked up
// by the caller's user id, so there is no parameter through which another
// user's thread could be addressed. A user who never wrote gets an empty
// page, not an error.
func (uc *SupportThreadUseCase) ListMessages(ctx context.Context, customerAccountID, userID string, limit, offset int32) (*SupportThreadPage, error) {
	channel, err := uc.internalChannel(ctx, customerAccountID)
	if err != nil {
		return nil, err
	}

	conv, err := uc.convRepo.FindByContact(ctx, channel.ID, userID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return &SupportThreadPage{Messages: []*domain.Message{}}, nil
	}

	// Opening the thread marks it read (US-004): stops the pending email
	// fallback window and clears the widget badge.
	uc.markRead(ctx, conv.ID)

	msgs, err := uc.msgRepo.ListByConversationID(ctx, conv.ID, limit, offset)
	if err != nil {
		return nil, err
	}
	return &SupportThreadPage{
		ConversationID: conv.ID,
		IsBotPaused:    conv.IsBotPaused,
		Messages:       msgs,
	}, nil
}

// SupportThreadPage is the GET response: the user's thread id (empty
// before first contact) and one page of messages, oldest first.
type SupportThreadPage struct {
	ConversationID string            `json:"conversation_id,omitempty"`
	IsBotPaused    bool              `json:"is_bot_paused"`
	Messages       []*domain.Message `json:"messages"`
}

// ErrSupportNoAdmin means the target customer account has no active admin
// user to open a thread with — structurally impossible for a live account
// (the last admin can't be removed), so it signals a deleted/corrupt one.
var ErrSupportNoAdmin = &domain.AppError{Code: "SUPPORT_NO_ADMIN", Message: "target account has no active admin user"}

// SupportOpenThreadResult is the POST /seller/support-threads/open payload:
// the (possibly fresh) internal thread with the target account's admin.
type SupportOpenThreadResult struct {
	ConversationID string `json:"conversation_id"`
	UserID         string `json:"user_id"`
	UserName       string `json:"user_name"`
	Created        bool   `json:"created"`
	IsBotPaused    bool   `json:"is_bot_paused"`
}

// OpenThread opens (or reuses) the internal CS thread with the ADMIN user
// of a carteira account, outbound-first (US-008): the co-pilot's 1-click
// action needs a thread to pre-fill even when the customer never wrote.
//
// Only the account's own carteira seller may open it — the co-pilot feed
// is carteira-scoped, and a non-carteira account answers ErrAccountNotFound
// (same no-leak convention as the suggestion dismissals).
//
// A FRESH thread is born in takeover mode (IsBotPaused=true) and assigned
// to the calling seller: the human initiated the outreach, so the platform
// agent must not hijack the customer's reply mid-conversation, and the
// SendHumanMessage bot-pause gate passes without an extra Assume step. An
// EXISTING thread is returned untouched (whatever bot/assignment state the
// normal pipeline left it in). No message is created here — the seller
// edits the template in the inbox composer and sends through the normal
// human-reply path, which fires the US-004 notification flow (inapp
// adapter notifier hook) like any outbound provider send.
func (uc *SupportThreadUseCase) OpenThread(ctx context.Context, sellerUserID, customerAccountID string) (*SupportOpenThreadResult, error) {
	channel, err := uc.internalChannel(ctx, customerAccountID)
	if err != nil {
		return nil, err
	}

	account, err := uc.accountRepo.GetByID(ctx, customerAccountID)
	if err != nil {
		return nil, err
	}
	if account == nil || account.SellerUserID == nil || *account.SellerUserID != sellerUserID {
		return nil, domain.ErrAccountNotFound
	}

	admin, err := uc.userRepo.GetAdminByAccountID(ctx, customerAccountID)
	if err != nil {
		return nil, err
	}
	if admin == nil {
		return nil, ErrSupportNoAdmin
	}

	conv, err := uc.convRepo.FindByContact(ctx, channel.ID, admin.ID)
	if err != nil {
		return nil, err
	}
	if conv != nil {
		return &SupportOpenThreadResult{
			ConversationID: conv.ID,
			UserID:         admin.ID,
			UserName:       admin.Name,
			Created:        false,
			IsBotPaused:    conv.IsBotPaused,
		}, nil
	}

	adminName := admin.Name
	adminEmail := admin.Email
	conv = &domain.Conversation{
		ID:              ulid.Make().String(),
		AccountID:       uc.platformAccountID,
		ChannelID:       channel.ID,
		AgentID:         channel.AgentID,
		ContactID:       admin.ID,
		ContactName:     &adminName,
		ContactUsername: &adminEmail,
		Status:          domain.ConversationOpen,
		IsBotPaused:     true,
	}
	created := true
	if err := uc.convRepo.Create(ctx, conv); err != nil {
		// Same race recovery as SaveInbound: the customer's first message
		// (or a parallel open) won the INSERT under the open-per-contact
		// partial unique index — reuse the winning row untouched.
		if err == domain.ErrConversationAlreadyOpen {
			existing, fetchErr := uc.convRepo.FindByContact(ctx, channel.ID, admin.ID)
			if fetchErr != nil || existing == nil {
				return nil, ErrSupportUnavailable
			}
			conv = existing
			created = false
		} else {
			return nil, err
		}
	}

	if created {
		// Stamp + assign mirror the inbound path (best-effort there); here
		// the thread exists solely for the seller's outreach, so both are
		// hard requirements of the flow — but a stamp/assign failure still
		// leaves a usable thread, so log-and-degrade keeps the 1-click
		// working (the grouped view self-heals on the customer's reply).
		if uc.stamper != nil {
			if err := uc.stamper.SetCustomerAccount(ctx, conv.ID, customerAccountID); err != nil {
				slog.Warn("support open thread: customer account stamp failed",
					"conversation_id", conv.ID,
					"customer_account_id", customerAccountID, "error", err)
			}
		}
		if err := uc.convRepo.AssignConversation(ctx, conv.ID, uc.platformAccountID, &sellerUserID); err != nil {
			slog.Warn("support open thread: seller assignment failed",
				"conversation_id", conv.ID,
				"seller_user_id", sellerUserID, "error", err)
		}
	}

	return &SupportOpenThreadResult{
		ConversationID: conv.ID,
		UserID:         admin.ID,
		UserName:       admin.Name,
		Created:        created,
		IsBotPaused:    conv.IsBotPaused,
	}, nil
}
