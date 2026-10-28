package domain

import (
	"context"
	"time"
)

// ── LGPD artifacts ─────────────────────────────────────────────────────────

// LGPDExportConversation is the flattened conversation view used in an
// LGPD export bundle. Distinct from Conversation to avoid leaking fields
// that aren't relevant to the titular (e.g. internal assignments).
type LGPDExportConversation struct {
	ID          string    `json:"id"`
	ChannelID   string    `json:"channel_id"`
	AgentID     *string   `json:"agent_id,omitempty"`
	ContactID   string    `json:"contact_id"`
	ContactName *string   `json:"contact_name,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// LGPDExportMessage flattens a message row for the export bundle.
type LGPDExportMessage struct {
	ID             string    `json:"id"`
	ConversationID *string   `json:"conversation_id,omitempty"`
	ChannelID      *string   `json:"channel_id,omitempty"`
	Sender         string    `json:"sender"`
	SenderName     *string   `json:"sender_name,omitempty"`
	Type           string    `json:"type"`
	Content        *string   `json:"content,omitempty"`
	Direction      string    `json:"direction"`
	CreatedAt      time.Time `json:"created_at"`
}

// LGPDCampaignParticipation is one broadcast send that targeted this
// contact, flattened for export.
type LGPDCampaignParticipation struct {
	RecipientID  string     `json:"recipient_id"`
	CampaignID   string     `json:"campaign_id"`
	CampaignName string     `json:"campaign_name"`
	Recipient    string     `json:"recipient"`
	Status       string     `json:"status"`
	SentAt       *time.Time `json:"sent_at,omitempty"`
	DeliveredAt  *time.Time `json:"delivered_at,omitempty"`
	ReadAt       *time.Time `json:"read_at,omitempty"`
	FailedAt     *time.Time `json:"failed_at,omitempty"`
}

// LGPDExportBundle is the full JSON payload downloaded by an admin as part
// of a LGPD Art. 18 "right of access / portability" request.
type LGPDExportBundle struct {
	GeneratedAt            time.Time                    `json:"generated_at"`
	AccountID              string                       `json:"account_id"`
	Contact                *Contact                     `json:"contact,omitempty"`
	Conversations          []*LGPDExportConversation    `json:"conversations"`
	Messages               []*LGPDExportMessage         `json:"messages"`
	CampaignParticipations []*LGPDCampaignParticipation `json:"campaign_participations"`
}

// LGPDRepository exposes the read + anonymize operations needed by the
// LGPD use cases. Kept separate from the main repositories so the
// anonymization SQL is co-located with the export SQL it pairs with.
type LGPDRepository interface {
	ListConversationsByContact(ctx context.Context, accountID, externalID string) ([]*LGPDExportConversation, error)
	ListMessagesByContact(ctx context.Context, accountID, externalID string) ([]*LGPDExportMessage, error)
	ListCampaignParticipations(ctx context.Context, accountID, externalID string) ([]*LGPDCampaignParticipation, error)

	// AnonymizeContact replaces PII fields on the contact, its messages, and
	// the denormalized contact_name on conversations. The externalID is
	// replaced with anonymousExternalID so future messages from the same
	// phone create a fresh relationship (the old one is gone for good).
	AnonymizeContact(ctx context.Context, accountID, externalID, anonymousExternalID string) error
}
