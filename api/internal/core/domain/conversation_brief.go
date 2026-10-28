package domain

import (
	"context"
	"time"
)

// A thread summary written for the SALESPERSON.
//
// Deliberately not conversations.summary, which is the rolling digest of
// turns that aged out of the model's context window: that one is written for
// the model, gets overwritten as the window slides, and says nothing about
// where the deal stands.
//
// This one answers the question someone actually has when they open a
// forty-message thread after a week away, or inherit a lead from a colleague:
// what was agreed, what is pending, and who owes the next move.

// ConversationBrief is one stored summary.
type ConversationBrief struct {
	ConversationID string `json:"conversation_id"`
	AccountID      string `json:"-"`
	Text           string `json:"text"`
	// The three answers a salesperson actually needs, split so the UI can
	// label them. Empty on rows written before the structured prompt;
	// readers fall back to Text.
	Agreed   string `json:"agreed,omitempty"`
	Pending  string `json:"pending,omitempty"`
	NextStep string `json:"next_step,omitempty"`
	// Model is recorded so a bad summary is traceable to a routing change
	// rather than argued about.
	Model string `json:"model,omitempty"`
	// CoversUntil is the newest message this brief accounts for.
	CoversUntil time.Time `json:"covers_until"`
	GeneratedAt time.Time `json:"generated_at"`
}

// IsStale reports whether the conversation has moved since this was written.
//
// A brief that silently describes a negotiation that has since progressed is
// worse than no brief: the reader acts on it. So staleness is computed from
// the data, never assumed from a TTL — a thread with no new messages has a
// brief that is still correct a month later, and one that got three messages
// an hour ago does not.
func (b *ConversationBrief) IsStale(latestMessageAt time.Time) bool {
	if b == nil {
		return true
	}
	return latestMessageAt.After(b.CoversUntil)
}

// MinMessagesForBrief is how much thread has to exist before summarising is
// worth a model call.
//
// Four: below that the messages fit on one screen and the summary would be
// longer than what it replaces. The feature earns its cost on the long
// threads, and running it on every two-message exchange is how a cheap model
// becomes an expensive line item.
const MinMessagesForBrief = 4

// ConversationBriefRepository stores and reads briefs.
type ConversationBriefRepository interface {
	Get(ctx context.Context, conversationID, accountID string) (*ConversationBrief, error)
	Save(ctx context.Context, b *ConversationBrief) error
}
