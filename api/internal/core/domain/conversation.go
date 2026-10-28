package domain

import "time"

// ContactStats summarizes lifetime activity of a single contact
// (phone) inside an account. Surfaced on the dashboard contact sheet
// so the attendant has a quick read on customer history without
// scrolling through past conversations.
type ContactStats struct {
	TotalConversations int64      `json:"total_conversations"`
	TotalMessages      int64      `json:"total_messages"`
	FirstContactAt     *time.Time `json:"first_contact_at,omitempty"`
	LastMessageAt      *time.Time `json:"last_message_at,omitempty"`
}

// ConversationStatus defines the state of a conversation
type ConversationStatus string

const (
	ConversationOpen         ConversationStatus = "open"
	ConversationPendingHuman ConversationStatus = "pending_human"
	ConversationClosed       ConversationStatus = "closed"
)

// AssignedFilterUnassigned is the sentinel the conversations `assigned`
// filter accepts to mean "no assignee" (US-020 — the dashboard's
// "Não atribuídas" bucket). It can never collide with a real user id:
// assigned_to values are 26-char ULIDs.
const AssignedFilterUnassigned = "none"

// Conversation represents a messaging thread between a customer and an agent
type Conversation struct {
	ID        string  `json:"id"`
	AccountID string  `json:"account_id"`
	ChannelID string  `json:"channel_id"`
	AgentID   *string `json:"agent_id,omitempty"`
	ContactID string  `json:"contact_id"`
	// ContactRefID is the FK to the contacts row behind this thread — the
	// opaque ULID, not the identifier the channel wrote.
	//
	// ContactID above holds the sender exactly as the CHANNEL sent it (bare
	// digits on WhatsApp, an address on email), which is what routing the next
	// inbound needs. Anything that starts from a PERSON — the lead panel, the
	// timeline, the opt-out guard — has to join on this instead, because a lead
	// knows the contact ULID and has no way to know which channel first reached
	// them. Nil when the contact could not be resolved (identity indexing is
	// best-effort) and on rows written before migration 000196.
	ContactRefID           *string            `json:"contact_ref_id,omitempty"`
	ContactName            *string            `json:"contact_name,omitempty"`
	ContactUsername        *string            `json:"contact_username,omitempty"`
	ContactAvatarURL       *string            `json:"contact_avatar_url,omitempty"`
	ContactAvatarFetchedAt *time.Time         `json:"contact_avatar_fetched_at,omitempty"`
	ContactBio             *string            `json:"contact_bio,omitempty"`
	Status                 ConversationStatus `json:"status"`
	IsBotPaused            bool               `json:"is_bot_paused"`
	AssignedTo             *string            `json:"assigned_to,omitempty"`
	AssignedAt             *time.Time         `json:"assigned_at,omitempty"`
	Priority               string             `json:"priority"`
	FirstResponseAt        *time.Time         `json:"first_response_at,omitempty"`
	ResolvedAt             *time.Time         `json:"resolved_at,omitempty"`
	LastCustomerMessageAt  *time.Time         `json:"last_customer_message_at,omitempty"`
	WaitingSince           *time.Time         `json:"waiting_since,omitempty"`
	// Language is an IETF-ish tag (e.g. "pt_BR", "en", "es") detected
	// from the customer's first inbound message. Nullable — legacy rows
	// and messages without text (media only) leave it unset, in which
	// case consumers fall back to pt_BR.
	Language *string `json:"language,omitempty"`
	// Sentiment is the analyzer's last classification of the rolling
	// inbound tone — one of positive/neutral/negative/angry. Nullable
	// because the analyzer is best-effort (skipped on media-only
	// messages, dropped on LLM failure). SentimentScore is the
	// model's confidence (0..1) so consumers can hide low-confidence
	// neutrals if they choose.
	Sentiment      *string  `json:"sentiment,omitempty"`
	SentimentScore *float32 `json:"sentiment_score,omitempty"`
	// Summary is a rolling, model-generated digest of the turns that have
	// aged out of the AI prompt window (US-024). It lets long
	// conversations keep context without replaying the entire (unbounded)
	// history on every turn. Nil/empty until the conversation grows past
	// the window. SummaryUpdatedAt throttles regeneration.
	Summary          *string    `json:"summary,omitempty"`
	SummaryUpdatedAt *time.Time `json:"summary_updated_at,omitempty"`
	// CustomerAccountID is set ONLY on internal CS threads (canal-interno
	// US-002): the conversation row belongs to the PLATFORM account, and
	// this stamps which CUSTOMER account the thread's user comes from, so
	// the seller panel can group threads by carteira account. Nil for all
	// regular channel conversations.
	CustomerAccountID *string   `json:"customer_account_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ConversationSummary includes enriched display data for listing
type ConversationSummary struct {
	ID                    string             `json:"id"`
	AccountID             string             `json:"account_id"`
	ChannelID             string             `json:"channel_id"`
	AgentID               *string            `json:"agent_id,omitempty"`
	ContactID             string             `json:"contact_id"`
	ContactName           string             `json:"contact_name"`
	ContactUsername       *string            `json:"contact_username,omitempty"`
	ContactAvatarURL      *string            `json:"contact_avatar_url,omitempty"`
	ContactBio            *string            `json:"contact_bio,omitempty"`
	Status                ConversationStatus `json:"status"`
	IsBotPaused           bool               `json:"is_bot_paused"`
	AssignedTo            *string            `json:"assigned_to,omitempty"`
	AssignedToName        *string            `json:"assigned_to_name,omitempty"`
	AssignedAt            *time.Time         `json:"assigned_at,omitempty"`
	Priority              string             `json:"priority"`
	FirstResponseAt       *time.Time         `json:"first_response_at,omitempty"`
	ResolvedAt            *time.Time         `json:"resolved_at,omitempty"`
	LastCustomerMessageAt *time.Time         `json:"last_customer_message_at,omitempty"`
	WaitingSince          *time.Time         `json:"waiting_since,omitempty"`
	CreatedAt             time.Time          `json:"created_at"`
	UpdatedAt             time.Time          `json:"updated_at"`
	ChannelType           string             `json:"channel_type"`
	LastMessage           *string            `json:"last_message,omitempty"`
	LastMessageType       string             `json:"last_message_type,omitempty"`
	Sentiment             *string            `json:"sentiment,omitempty"`
	SentimentScore        *float32           `json:"sentiment_score,omitempty"`
	Tags                  []*Tag             `json:"tags,omitempty"`

	// Lead context for the inbox (US-M024): which pipeline stage this
	// contact sits in, so an attendant answering a message knows whether
	// they are talking to a stranger or to a deal in negotiation. All
	// three are empty when the contact is not a lead — the inbox works
	// exactly as before for pure support conversations.
	LeadID string `json:"lead_id,omitempty"`
	// LeadStageName is the customer's own column name. Empty for leads
	// created before the board existed; LeadStatus is then the fallback,
	// because a chip reading the canonical status is still more useful
	// than no chip.
	LeadStageName string `json:"lead_stage_name,omitempty"`
	LeadStatus    string `json:"lead_status,omitempty"`

	// Inbox triage (§6.10 camada 3). Empty for a conversation classified
	// before the feature existed, and for every channel that is not a shared
	// mailbox — the inbox falls back to chronological, which is correct for
	// a chat.
	InboxCategory   string `json:"inbox_category,omitempty"`
	InboxImportance int    `json:"inbox_importance,omitempty"`

	// UnreadCount is how many customer (inbound) messages nobody opened
	// yet — messages.read_at IS NULL, stamped by POST /conversations/:id/read.
	// The tile used to derive "unread" from status = pending_human, which
	// is a different fact (waiting on a human, not unseen).
	UnreadCount int `json:"unread_count"`
}

// ConversationAssignment is the slim view returned by GetAssignment.
// It carries enough state for the dashboard to render the takeover
// badge ("Em atendimento por X") and enable/disable the reply input
// without a second roundtrip.
type ConversationAssignment struct {
	ConversationID string     `json:"conversation_id"`
	AssignedTo     *string    `json:"assigned_to,omitempty"`
	AssignedToName *string    `json:"assigned_to_name,omitempty"`
	AssignedAt     *time.Time `json:"assigned_at,omitempty"`
	IsBotPaused    bool       `json:"is_bot_paused"`
}

// IsWithin24hWindow checks if the last customer message was within 24 hours.
// This is critical for channels like WhatsApp that enforce a 24h reply window.
func (c *Conversation) IsWithin24hWindow() bool {
	if c.LastCustomerMessageAt == nil {
		return false
	}
	return time.Since(*c.LastCustomerMessageAt) <= 24*time.Hour
}

// ReminderCandidate is the worker-shaped row used by the reply-reminder
// scanner. It carries every field needed to dispatch push / email /
// WS in one place so the worker doesn't have to re-query per row.
//
// AssignedTo is the attendant currently holding the conversation
// (set by AssumeConversation). When nil, the worker falls back to
// notifying every admin on the account.
type ReminderCandidate struct {
	AccountID        string
	ConversationID   string
	ChannelID        string
	ContactName      string
	AssignedTo       *string
	ThresholdMinutes int
	SentCount        int
	WaitingSince     time.Time
}
