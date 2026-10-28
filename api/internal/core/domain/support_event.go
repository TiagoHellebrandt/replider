package domain

import (
	"encoding/json"
	"time"
)

// Internal CS threads (canal-interno US-003) have TWO live audiences: the
// platform inbox (the seller watching their carteira) and the customer
// user's support widget. Every message — customer inbound, platform AI
// reply, seller human reply — is therefore broadcast to BOTH account ids.
// The platform side reuses the standard `new_message` shape its inbox
// already consumes; the customer side gets the dedicated event built here,
// carrying the internal-thread marker and the thread-owner user id so the
// front end renders it ONLY for that user (per-user thread privacy) and
// never mixes it into the customer's normal inbox.

// SupportMessageEvent is the customer-side WS event name for internal CS
// thread messages. Deliberately distinct from "new_message" so the customer
// dashboard's inbox listener never picks it up.
const SupportMessageEvent = "support_message"

// SupportMessageEventPayload builds the customer-side WS event for an
// internal-thread message. userID is the THREAD OWNER (the conversation's
// contact), not necessarily the message author — the widget filters on it
// so only the owner's session renders the event.
func SupportMessageEventPayload(conversationID, userID string, msg *Message) ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"event":           SupportMessageEvent,
		"internal_thread": true,
		"user_id":         userID,
		"conversation_id": conversationID,
		"message":         msg,
	})
}

// SupportFallbackCandidate is one internal thread whose owner hasn't read
// a platform-side message past the configured window (US-004): the email
// fallback worker turns each row into a single "you have an unread reply"
// email. Currency comes from the CUSTOMER account and drives the email
// locale (EmailLocaleForCurrency) — empty for unstamped legacy threads,
// which safely falls back to pt-BR.
type SupportFallbackCandidate struct {
	ConversationID    string
	UserID            string // thread owner (conversation contact_id)
	UserEmail         string
	UserName          string
	CustomerAccountID string
	Currency          string
	LastOutboundAt    time.Time // newest platform-side message in the thread
}
