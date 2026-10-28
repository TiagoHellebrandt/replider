package resend

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	chemail "github.com/replider/api/internal/adapters/channels/email"
	"github.com/replider/api/internal/core/domain"
)

// SendMessageAndGetID satisfies the same outbound-approach sender port the
// SMTP adapter does, so the approval path stays single: a bulk approval swaps
// the sender, nothing else. The mail still says it is FROM the customer's own
// address — Resend signs the customer's (bulk-verified) domain — and replies
// go back to the customer's mailbox, where the normal inbound pipeline
// threads them.
func (c *Client) SendMessageAndGetID(ctx context.Context, channel *domain.Channel, to string, msg *domain.Message) (string, error) {
	if !c.Configured() {
		return "", domain.ErrBulkSendingUnavailable
	}
	if !domain.IsEmailAddress(to) {
		return "", &domain.AppError{Code: "VALIDATION_ERROR", Message: "recipient is not a valid email address"}
	}
	fromAddr, fromName := channelFrom(channel)
	if fromAddr == "" {
		return "", &domain.AppError{Code: "CHANNEL_MISSING_CONFIG", Message: "email channel has no from_address"}
	}
	body := ""
	if msg != nil && msg.Content != nil {
		body = *msg.Content
	}
	if strings.TrimSpace(body) == "" {
		return "", &domain.AppError{Code: "EMPTY_MESSAGE", Message: "email body is empty"}
	}
	subject := ""
	if msg != nil {
		if meta, ok := metadataAsMap(msg.Metadata); ok {
			subject, _ = meta[chemail.MetaSubject].(string)
		}
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		name := fromName
		if name == "" {
			name = channel.Name
		}
		subject = fmt.Sprintf("Mensagem de %s", name)
	}

	from := fromAddr
	if fromName != "" {
		from = fmt.Sprintf("%s <%s>", fromName, fromAddr)
	}
	return c.SendEmail(ctx, from, fromAddr, to, subject, body)
}

// channelFrom pulls from_address/from_name off the channel config, tolerating
// the same shapes configFrom in the SMTP adapter does (map or JSON blob).
func channelFrom(channel *domain.Channel) (addr, name string) {
	if channel == nil || channel.Config == nil {
		return "", ""
	}
	m, ok := metadataAsMap(channel.Config)
	if !ok {
		return "", ""
	}
	get := func(k string) string {
		s, _ := m[k].(string)
		return strings.TrimSpace(s)
	}
	return strings.ToLower(get("from_address")), get("from_name")
}

func metadataAsMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case []byte:
		var out map[string]any
		if json.Unmarshal(m, &out) == nil {
			return out, true
		}
	case json.RawMessage:
		var out map[string]any
		if json.Unmarshal(m, &out) == nil {
			return out, true
		}
	default:
		raw, err := json.Marshal(v)
		if err == nil {
			var out map[string]any
			if json.Unmarshal(raw, &out) == nil {
				return out, true
			}
		}
	}
	return nil, false
}
