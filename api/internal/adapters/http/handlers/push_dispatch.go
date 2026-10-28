package handlers

import (
	"context"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// composeInboundPushPayload turns a freshly-saved inbound message into
// the user-facing notification body. Lives next to the dispatch helper
// so every channel-handler that receives an inbound (WhatsApp, generic
// webhook, webchat widget) renders push copy the same way.
//
// Localisation policy: the server never bakes language-specific copy
// into the payload. Title is the raw contact name (untranslated proper
// noun); Body is the raw message content; when the message is media
// only, Body is left empty and MediaType carries the discriminator so
// the service-worker can render a translated label like "📷 Image"
// keyed by the browser's locale. An account with attendants in
// multiple languages gets each device's correct translation without
// the API knowing per-user locale.
func composeInboundPushPayload(partial *usecases.InboundPartialResult) domain.PushPayload {
	if partial == nil || partial.Message == nil {
		return domain.PushPayload{}
	}
	var title string
	if partial.Message.SenderName != nil && *partial.Message.SenderName != "" {
		title = *partial.Message.SenderName
	}
	var body string
	if partial.Message.Content != nil {
		body = *partial.Message.Content
	}
	return domain.PushPayload{
		Title:          title, // empty → SW fills generic "Replider" / "New message"
		Body:           body,  // empty → SW renders localised media-type fallback
		MediaType:      partial.Message.Type,
		Tag:            "conv:" + partial.ConversationID,
		ConversationID: partial.ConversationID,
		Event:          "new_message",
		// The inbox route with the thread pre-selected. There is no
		// /conversations/<id> route in the app — the old URL landed on
		// the router's not-found fallback.
		ClickURL: "/home?conversation=" + partial.ConversationID,
	}
}

// dispatchInboundPushAsync runs NotifyAccount in a detached goroutine so
// the request path is never blocked by a network round-trip to the push
// service. Recovers panics — push failures must never propagate out of
// a webhook ack.
//
// Pass a nil pushUC for handlers that haven't been wired yet; the call
// becomes a no-op.
func dispatchInboundPushAsync(pushUC *usecases.PushNotificationUseCase, partial *usecases.InboundPartialResult) {
	if pushUC == nil || partial == nil || partial.Message == nil {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("inbound push panic", "recover", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		pushUC.NotifyAccount(ctx, partial.AccountID, "", composeInboundPushPayload(partial))
	}()
}

// composeHandoffPushPayload builds the dedicated push payload fired
// when the AI throws a silent handoff. Keeping the title/body distinct
// from a regular inbound push lets the service-worker (and the OS
// notification shade on mobile) render different copy — "AI handed off"
// vs "New message" — so an attendant who already saw the inbound push
// understands a second, urgency-bearing notification arrived.
//
// We re-use Tag = "conv:<id>" so the OS coalesces the two notifications
// into the same thread instead of stacking them; the handoff push
// supersedes the inbound one on screen because it lands second.
func composeHandoffPushPayload(accountID, conversationID, contactName string) domain.PushPayload {
	return domain.PushPayload{
		Title:          contactName,
		Body:           "🟠 IA não soube responder — atendimento humano necessário",
		Tag:            "conv:" + conversationID,
		ConversationID: conversationID,
		Event:          "human_handoff_requested",
		ClickURL:       "/home?conversation=" + conversationID,
	}
}

// dispatchHandoffPushAsync fires the handoff-specific push in a
// detached goroutine. No-op when pushUC is nil so handlers wired
// before the push UC is present don't crash.
func dispatchHandoffPushAsync(pushUC *usecases.PushNotificationUseCase, accountID, conversationID, contactName string) {
	if pushUC == nil {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("handoff push panic", "recover", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		pushUC.NotifyAccount(ctx, accountID, "", composeHandoffPushPayload(accountID, conversationID, contactName))
	}()
}
