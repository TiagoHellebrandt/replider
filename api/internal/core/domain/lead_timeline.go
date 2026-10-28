package domain

import (
	"context"
	"time"
)

// What has actually been said to this customer, and what they said back.
//
// A vendedor could not see this. The inbox is `RequireRole("admin",
// "attendant")` and a vendedor deliberately does not satisfy attendant —
// "opening every conversation in the account is not part of the job" — so the
// person who sent a proposal had no way to read the reply to it. That is a hole,
// not a permission: reading the conversation of YOUR OWN lead is a different
// question from reading any conversation in the account, and it is the lead's
// ownership rules that answer it.

// TimelineMessage is one line of that history, flattened for reading.
//
// Deliberately not `Message`: this is a read model. A vendedor needs who spoke,
// when, and what was said — not the provider's message id, the media path, or
// the transcript cache, all of which belong to the inbox that operates on them.
type TimelineMessage struct {
	ID string `json:"id"`
	// Author is who said it, resolved from the raw sender. See TimelineAuthor.
	Author TimelineAuthor `json:"author"`
	// AuthorName is the human behind an outbound reply, when there is one. Empty
	// for the customer and for the bot, whose names the row already carries.
	AuthorName string `json:"author_name,omitempty"`
	// Text is what was actually said. Empty for a media-only message, which the
	// client renders as its Kind rather than as a blank line.
	Text string `json:"text"`
	// Kind is the message type ('text', 'image', 'audio', …). Carried so a photo
	// reads as a photo instead of vanishing.
	Kind        string    `json:"kind"`
	ChannelType string    `json:"channel_type"`
	SentAt      time.Time `json:"sent_at"`
	// FailedAt/FailedReason expose the channel adapter's delivery failure
	// (messages.failed_at). Without them an approach the adapter could not
	// deliver rendered in the lead's history exactly like one that reached
	// the customer — the salesperson then "followed up" on a message nobody
	// ever received. The inbox already shows this; the timeline must not be
	// the one read that hides it.
	FailedAt     *time.Time `json:"failed_at,omitempty"`
	FailedReason string     `json:"failed_reason,omitempty"`
}

// TimelineAuthor is who said a line, in the only three categories that change
// how a salesperson reads it.
type TimelineAuthor string

const (
	// AuthorCustomer is the person being sold to. Every inbound message.
	AuthorCustomer TimelineAuthor = "customer"
	// AuthorBot is the AI. Kept distinct from a teammate on purpose: "we
	// answered" and "the bot answered" are different facts about the account.
	AuthorBot TimelineAuthor = "bot"
	// AuthorTeam is a human on this side.
	AuthorTeam TimelineAuthor = "team"
)

// ResolveTimelineAuthor turns the stored `messages.sender` into who spoke.
//
// The column is overloaded by history: inbound rows carry the CONTACT's phone or
// external id, bot rows carry the literal 'bot', and a human reply carries the
// user's ULID. The literal 'customer' only appears in old seeds and tests, never
// in production — but it costs one line to keep reading it, and a row that
// renders as a teammate when it was the customer is a misread of the whole
// conversation.
func ResolveTimelineAuthor(direction, sender string) TimelineAuthor {
	if direction == "inbound" {
		return AuthorCustomer
	}
	switch sender {
	case "bot", "":
		return AuthorBot
	case "customer":
		// Outbound-from-'customer' is not a thing; treat it as the bot rather
		// than claiming a teammate wrote it.
		return AuthorBot
	default:
		return AuthorTeam
	}
}

// LeadTimelineRepository reads the exchange with one contact.
//
// Its own port rather than a method on ConversationRepository: that interface has
// a dozen mocks across the test suite, and this needs none of what they
// implement. A contact can hold conversations on several channels, so the query
// spans them — the customer had one relationship, not one per channel.
type LeadTimelineRepository interface {
	// ListForContact returns the MOST RECENT [limit] messages, oldest first.
	//
	// The cap takes the latest ones and then reverses: taking the first N of a
	// five-hundred-message history would show a conversation from two years ago
	// and call it the timeline.
	ListForContact(ctx context.Context, accountID, contactID string, limit int32) ([]TimelineMessage, error)
}

// TimelineLimit is how many lines one read returns.
//
// Enough to cover a normal back-and-forth; past it, what a salesperson needs is
// the recent stretch, and the inbox remains the place to read a long history in
// full.
const TimelineLimit = 100

// ErrLeadTimelineUnavailable is what an unwired timeline reports. Returning an
// empty list instead would tell a salesperson that nobody has ever written to
// this customer, which is a different and much worse statement than "this is not
// working".
var ErrLeadTimelineUnavailable = &AppError{
	Code:    "LEAD_TIMELINE_UNAVAILABLE",
	Message: "o histórico de conversa não está disponível nesta instalação",
}
