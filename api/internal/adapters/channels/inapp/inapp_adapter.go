// Package inapp implements the "internal" channel type: messages between
// the platform account (Replider CS/sellers) and customer-account users,
// delivered inside the product itself. There is no external messaging API —
// "delivery" is the persisted message row plus the WebSocket broadcast the
// caller already performs for every channel. The package is named inapp
// (not internal) because a directory called internal would trigger Go's
// internal-package visibility rule and hide it from cmd/api.
package inapp

import (
	"context"
	"log/slog"

	"github.com/replider/api/internal/core/domain"
)

// NotifyFunc is the hook fired after an outbound internal message is
// accepted for delivery. US-004 wires push/badge/email fallback through
// it; until then it stays nil and SendMessage is a pure no-op success.
type NotifyFunc func(ctx context.Context, channel *domain.Channel, to string, msg *domain.Message)

// ConversationResolver resolves the conversation an outbound internal
// message belongs to, so the adapter can find the CUSTOMER account for the
// widget-side broadcast. Narrow port satisfied by
// domain.ConversationRepository (GetByID is account-scoped; internal
// conversations live on the platform account = channel.AccountID).
type ConversationResolver interface {
	GetByID(ctx context.Context, id string, accountID string) (*domain.Conversation, error)
}

// Adapter satisfies domain.MessageProvider for channel type "internal".
type Adapter struct {
	notify        NotifyFunc
	broadcaster   domain.EventBroadcaster
	conversations ConversationResolver
}

func NewAdapter() *Adapter { return &Adapter{} }

// SetNotifier installs the outbound-notification hook (optional; nil-safe).
func (a *Adapter) SetNotifier(fn NotifyFunc) { a.notify = fn }

// SetBroadcaster wires the customer-side real-time mirror (US-003): every
// outbound send on an internal thread — human reply, takeover send — is
// broadcast to the CUSTOMER account as a `support_message` event so the
// support widget updates live. The platform-side `new_message` broadcast is
// already performed by the callers (reply handler / takeover use case), so
// the adapter only covers the side they don't. Optional; nil keeps sends
// working without real-time delivery.
func (a *Adapter) SetBroadcaster(b domain.EventBroadcaster, conversations ConversationResolver) {
	a.broadcaster = b
	a.conversations = conversations
}

// SendMessage performs no external call: the caller persists the message
// and broadcasts it to the platform inbox, which together with the
// customer-side mirror below IS the delivery for an internal conversation.
func (a *Adapter) SendMessage(ctx context.Context, channel *domain.Channel, to string, msg *domain.Message) error {
	if a.notify != nil {
		a.notify(ctx, channel, to, msg)
	}
	a.mirrorToCustomer(ctx, channel, msg)
	return nil
}

// mirrorToCustomer broadcasts an outbound internal message to the customer
// account's WebSocket clients, tagged with the thread-owner user id so only
// that user's widget renders it. Best-effort: the message is already
// persisted, so a lookup/broadcast hiccup degrades to GET polling.
func (a *Adapter) mirrorToCustomer(ctx context.Context, channel *domain.Channel, msg *domain.Message) {
	if a.broadcaster == nil || a.conversations == nil || msg == nil || msg.ConversationID == nil {
		return
	}
	conv, err := a.conversations.GetByID(ctx, *msg.ConversationID, channel.AccountID)
	if err != nil || conv == nil {
		slog.Warn("inapp: conversation lookup for customer broadcast failed",
			"conversation_id", *msg.ConversationID, "error", err)
		return
	}
	if conv.CustomerAccountID == nil || *conv.CustomerAccountID == "" {
		return
	}
	payload, err := domain.SupportMessageEventPayload(conv.ID, conv.ContactID, msg)
	if err != nil {
		slog.Warn("inapp: marshal customer broadcast failed",
			"conversation_id", conv.ID, "error", err)
		return
	}
	a.broadcaster.BroadcastToAccount(*conv.CustomerAccountID, payload)
}

// HandleWebhook never runs: internal inbound is the authenticated
// /me/support/messages endpoint, not a webhook.
func (a *Adapter) HandleWebhook(ctx context.Context, channel *domain.Channel, payload []byte) error {
	return &domain.AppError{Code: "CHANNEL_NO_WEBHOOK", Message: "internal channels have no inbound webhook"}
}
