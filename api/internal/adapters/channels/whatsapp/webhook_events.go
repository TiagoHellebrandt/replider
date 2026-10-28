package whatsapp

import (
	"encoding/json"
)

// ChangeEvent is a typed representation of every non-message webhook
// `change.field` Meta sends. Today the API stack only consumed
// `messages` and ignored everything else; that meant template
// approvals, quality flags and account-review events were dropped
// silently. This type lets the handler dispatch on Field while
// retaining the raw payload for future-proofing and ops triage.
type ChangeEvent struct {
	Field         string          // e.g. "message_template_status_update"
	WABAID        string          // entry.id — the WABA the event targets
	PhoneNumberID string          // best-effort, only set when present
	Payload       json.RawMessage // raw `value` blob; parse per field as needed
}

// ParseEvents returns every change in the webhook payload, regardless
// of field. Use Filter() / typed accessors to handle a specific field.
func ParseEvents(body []byte) []ChangeEvent {
	var p struct {
		Object string `json:"object"`
		Entry  []struct {
			ID      string `json:"id"`
			Changes []struct {
				Field string          `json:"field"`
				Value json.RawMessage `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return nil
	}
	if p.Object != "whatsapp_business_account" {
		return nil
	}
	out := make([]ChangeEvent, 0, len(p.Entry))
	for _, e := range p.Entry {
		for _, c := range e.Changes {
			ev := ChangeEvent{
				Field:   c.Field,
				WABAID:  e.ID,
				Payload: c.Value,
			}
			// Try to surface phone_number_id for fields that carry it
			// inside metadata so the handler can resolve a channel.
			var withMeta struct {
				Metadata struct {
					PhoneNumberID string `json:"phone_number_id"`
				} `json:"metadata"`
				DisplayPhoneNumber string `json:"display_phone_number"`
				PhoneNumber        string `json:"phone_number"`
				WABAInfo           struct {
					ID string `json:"id"`
				} `json:"whatsapp_business_account_id"`
			}
			if err := json.Unmarshal(c.Value, &withMeta); err == nil {
				ev.PhoneNumberID = withMeta.Metadata.PhoneNumberID
			}
			out = append(out, ev)
		}
	}
	return out
}

// TemplateStatusUpdate is the parsed body of a
// `message_template_status_update` event. Fired when a template
// transitions APPROVED/REJECTED/PAUSED/DISABLED — the dashboard uses
// it to invalidate cached lists and surface a banner.
type TemplateStatusUpdate struct {
	Event           string `json:"event"` // e.g. "APPROVED", "REJECTED", "FLAGGED"
	MessageTemplate struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Language string `json:"language"`
	} `json:"message_template"`
	Reason    string `json:"reason,omitempty"`
	OtherInfo string `json:"other_info,omitempty"`
}

func (e ChangeEvent) AsTemplateStatus() (*TemplateStatusUpdate, bool) {
	if e.Field != "message_template_status_update" {
		return nil, false
	}
	var v TemplateStatusUpdate
	if err := json.Unmarshal(e.Payload, &v); err != nil {
		return nil, false
	}
	return &v, true
}

// QualityUpdate is the parsed body of `phone_number_quality_update`.
// Drives the in-app banner that warns the customer when their number
// is at risk of rate-limit or ban.
type QualityUpdate struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	Event              string `json:"event"`           // "FLAGGED" / "UNFLAGGED" / "QUALITY_DROP"
	CurrentLimit       string `json:"current_limit"`   // e.g. "TIER_1K"
	Quality            string `json:"current_quality"` // "GREEN" / "YELLOW" / "RED"
}

func (e ChangeEvent) AsQualityUpdate() (*QualityUpdate, bool) {
	if e.Field != "phone_number_quality_update" {
		return nil, false
	}
	var v QualityUpdate
	if err := json.Unmarshal(e.Payload, &v); err != nil {
		return nil, false
	}
	return &v, true
}

// AccountReviewUpdate fires when the WABA itself transitions
// review state. Mostly informational; we surface it so customers
// don't get blindsided by a sudden block.
type AccountReviewUpdate struct {
	DecisionEvent string `json:"decision_event"`
}

func (e ChangeEvent) AsAccountReviewUpdate() (*AccountReviewUpdate, bool) {
	if e.Field != "account_review_update" {
		return nil, false
	}
	var v AccountReviewUpdate
	if err := json.Unmarshal(e.Payload, &v); err != nil {
		return nil, false
	}
	return &v, true
}

// IsMessageEvent reports whether this change carries inbound messages
// or status updates (already consumed by ParseInbound / ParseStatuses).
// Use it to skip the meta-event branch for the high-volume path.
func (e ChangeEvent) IsMessageEvent() bool {
	return e.Field == "messages"
}

// The `flows` lifecycle parser lived here and went with the feature. Note this
// is only about MANAGING flows: an inbound `nfm_reply` — a customer submitting a
// native form that was built in Business Manager — is still parsed and still
// becomes a message (see dto.go / whatsapp_adapter.go). Dropping that would
// discard a real customer's answer.
