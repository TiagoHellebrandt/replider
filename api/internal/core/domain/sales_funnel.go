package domain

import (
	"context"
	"encoding/json"
	"regexp"
	"time"
)

// MetadataFieldType enumerates the value types a funnel metadata field can
// hold. The AI is instructed to collect a value matching the declared type.
type MetadataFieldType string

const (
	MetadataFieldText   MetadataFieldType = "text"
	MetadataFieldNumber MetadataFieldType = "number"
	MetadataFieldCPF    MetadataFieldType = "cpf"
	MetadataFieldCNPJ   MetadataFieldType = "cnpj"
	MetadataFieldDate   MetadataFieldType = "date"
	MetadataFieldBool   MetadataFieldType = "bool"
	MetadataFieldEnum   MetadataFieldType = "enum"
)

// IsValid reports whether t is one of the allowed metadata field types.
func (t MetadataFieldType) IsValid() bool {
	switch t {
	case MetadataFieldText, MetadataFieldNumber, MetadataFieldCPF,
		MetadataFieldCNPJ, MetadataFieldDate, MetadataFieldBool, MetadataFieldEnum:
		return true
	default:
		return false
	}
}

// MaxMetadataFields caps how many fields a single funnel schema may declare,
// keeping the dynamically-generated tool parameters bounded.
const MaxMetadataFields = 30

// metadataKeyRe matches a snake_case identifier (lowercase letter first, then
// lowercase letters/digits/underscores).
var metadataKeyRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// MetadataFieldDef describes one field the AI should invisibly collect during
// the conversation. Persisted as one element of the funnel's metadata_schema
// JSONB array.
type MetadataFieldDef struct {
	Key         string            `json:"key"`                    // snake_case, unique within the funnel
	Label       string            `json:"label"`                  // human-facing label for the dashboard
	Type        MetadataFieldType `json:"type"`                   // text/number/cpf/cnpj/date/bool/enum
	Required    bool              `json:"required"`               // gates the lead.qualified webhook
	AIHint      string            `json:"ai_hint,omitempty"`      // guidance for the AI on how/when to collect
	EnumOptions []string          `json:"enum_options,omitempty"` // allowed values when Type == enum
}

// SalesFunnel ties a product, agent and channel to a dynamic metadata schema
// and webhook configuration (US-002).
type SalesFunnel struct {
	ID                        string          `json:"id"`
	AccountID                 string          `json:"account_id"`
	Name                      string          `json:"name"`
	ProductID                 *string         `json:"product_id,omitempty"`
	AgentID                   *string         `json:"agent_id,omitempty"`
	ChannelID                 *string         `json:"channel_id,omitempty"`
	PersonaPrompt             string          `json:"persona_prompt"`
	MetadataSchema            json.RawMessage `json:"metadata_schema"`
	WebhookURL                string          `json:"webhook_url"`
	WebhookSecret             string          `json:"webhook_secret"`
	TriggerOnRequiredComplete bool            `json:"trigger_on_required_complete"`
	TriggerOnBuyIntent        bool            `json:"trigger_on_buy_intent"`
	Active                    bool            `json:"active"`
	// IsDefault marks the account's implicit funnel: created with the
	// account, named for what it holds ("Clientes"), and the destination for
	// anything imported or found without a funnel chosen. There is at most
	// one per account and it cannot be deleted — a person who never opened
	// the funnel screen must still have somewhere for her customers to be.
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultFunnelName is what the implicit funnel is called. Portuguese on
// purpose: it is the label a Brazilian distributor reads on the Clientes tab,
// and the account has no locale of its own to render it from.
const DefaultFunnelName = "Clientes"

// MetadataFields decodes the funnel's metadata_schema JSONB into typed field
// definitions. An empty/nil schema yields an empty slice with no error.
func (f *SalesFunnel) MetadataFields() ([]MetadataFieldDef, error) {
	if len(f.MetadataSchema) == 0 {
		return nil, nil
	}
	var fields []MetadataFieldDef
	if err := json.Unmarshal(f.MetadataSchema, &fields); err != nil {
		return nil, ErrInvalidMetadataSchema
	}
	return fields, nil
}

// ValidateMetadataSchema validates a slice of field definitions: keys must be
// snake_case, unique within the funnel, types must be allowed, and enum fields
// must declare at least one option. Returns nil for an empty schema.
func ValidateMetadataSchema(fields []MetadataFieldDef) error {
	if len(fields) > MaxMetadataFields {
		return ErrTooManyMetadataFields
	}
	seen := make(map[string]struct{}, len(fields))
	for _, fd := range fields {
		if !metadataKeyRe.MatchString(fd.Key) {
			return ErrInvalidMetadataKey
		}
		if _, dup := seen[fd.Key]; dup {
			return ErrDuplicateMetadataKey
		}
		seen[fd.Key] = struct{}{}
		if !fd.Type.IsValid() {
			return ErrInvalidMetadataType
		}
		if fd.Type == MetadataFieldEnum && len(fd.EnumOptions) == 0 {
			return ErrEnumOptionsRequired
		}
	}
	return nil
}

// SalesFunnelRepository defines persistence operations for sales funnels.
type SalesFunnelRepository interface {
	Create(ctx context.Context, funnel *SalesFunnel) error
	GetByID(ctx context.Context, id string) (*SalesFunnel, error)
	// GetByChannelID returns the active funnel bound to a channel, or nil
	// when no active funnel backs it (US-011 invisible lead collection).
	GetByChannelID(ctx context.Context, channelID string) (*SalesFunnel, error)
	ListByAccountID(ctx context.Context, accountID string) ([]*SalesFunnel, error)
	Update(ctx context.Context, funnel *SalesFunnel) error
	Delete(ctx context.Context, id string) error
	// EnsureDefault returns the account's default funnel, creating it when
	// the account has none. Idempotent and safe under concurrent callers.
	EnsureDefault(ctx context.Context, accountID string) (*SalesFunnel, error)
}

var (
	ErrSalesFunnelNotFound   = &AppError{Code: "SALES_FUNNEL_NOT_FOUND", Message: "sales funnel not found"}
	ErrCannotDeleteDefault   = &AppError{Code: "SALES_FUNNEL_IS_DEFAULT", Message: "the default funnel cannot be deleted"}
	ErrInvalidMetadataSchema = &AppError{Code: "FUNNEL_INVALID_METADATA_SCHEMA", Message: "metadata schema is not valid JSON"}
	ErrInvalidMetadataKey    = &AppError{Code: "FUNNEL_INVALID_METADATA_KEY", Message: "metadata field key must be snake_case"}
	ErrDuplicateMetadataKey  = &AppError{Code: "FUNNEL_DUPLICATE_METADATA_KEY", Message: "metadata field keys must be unique within the funnel"}
	ErrInvalidMetadataType   = &AppError{Code: "FUNNEL_INVALID_METADATA_TYPE", Message: "metadata field type must be one of text/number/cpf/cnpj/date/bool/enum"}
	ErrEnumOptionsRequired   = &AppError{Code: "FUNNEL_ENUM_OPTIONS_REQUIRED", Message: "enum metadata field requires at least one option"}
	ErrTooManyMetadataFields = &AppError{Code: "FUNNEL_TOO_MANY_METADATA_FIELDS", Message: "metadata schema exceeds the maximum number of fields"}
	ErrFunnelWebhookNotSet   = &AppError{Code: "FUNNEL_WEBHOOK_NOT_CONFIGURED", Message: "funnel has no webhook URL configured"}
	// ErrFunnelWebhookTesterUnavailable is returned when the webhook-test
	// dispatcher was not wired (US-017) — a server misconfiguration, not a
	// client error.
	ErrFunnelWebhookTesterUnavailable = &AppError{Code: "FUNNEL_WEBHOOK_TESTER_UNAVAILABLE", Message: "webhook test is not available"}
	// ErrFunnelWebhookTestFailed wraps a transport failure reaching the
	// consumer's endpoint during a webhook test (US-017).
	ErrFunnelWebhookTestFailed = &AppError{Code: "FUNNEL_WEBHOOK_TEST_FAILED", Message: "failed to reach the funnel webhook endpoint"}
)
