package domain

import (
	"context"
	"encoding/json"
	"time"
)

// WhatsAppTemplate is one Meta-approved message template cached locally
// (US-018). Templates are required to open an outbound conversation
// outside the 24h customer-service window. We sync them from the Graph
// API (message_templates) so the dashboard + send pipeline can render and
// validate parameters without a Graph roundtrip every time.
//
// MetaStatus mirrors Meta's lifecycle verbatim (APPROVED | PENDING |
// REJECTED | PAUSED | DISABLED) so a drift between our cache and Meta is
// always a visible "we're behind a sync" rather than a silent mismapping.
// RejectedReason is populated only when MetaStatus == "REJECTED".
type WhatsAppTemplate struct {
	ID             string          `json:"id"`
	AccountID      string          `json:"account_id"`
	ChannelID      string          `json:"channel_id"`
	MetaTemplateID string          `json:"meta_template_id"`
	Name           string          `json:"name"`
	Language       string          `json:"language"`
	Category       string          `json:"category"`
	MetaStatus     string          `json:"meta_status"`
	Body           string          `json:"body"`
	Components     json.RawMessage `json:"components,omitempty"`
	Variables      json.RawMessage `json:"variables,omitempty"`
	RejectedReason string          `json:"rejected_reason,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// WhatsAppTemplateRepository persists the per-channel template cache.
// All operations are channel-scoped; the use case layer enforces account
// ownership before calling in.
type WhatsAppTemplateRepository interface {
	// Upsert inserts or updates a template keyed by (channel_id, name,
	// language) so re-syncing is idempotent.
	Upsert(ctx context.Context, t *WhatsAppTemplate) error
	// ListByChannel returns every cached template for a channel ordered
	// by name then language.
	ListByChannel(ctx context.Context, channelID string) ([]*WhatsAppTemplate, error)
}

// ErrWhatsAppTemplateNotFound is returned when a template lookup misses.
var ErrWhatsAppTemplateNotFound = &AppError{Code: "WHATSAPP_TEMPLATE_NOT_FOUND", Message: "whatsapp template not found"}
