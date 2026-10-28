package usecases

import (
	"encoding/json"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// Metadata keys shared with the email channel adapter. Duplicated here on
// purpose: usecases cannot import internal/adapters/channels/email
// (dependency direction is adapters -> usecases -> domain). Keep in sync
// with MetaMessageID / MetaInReplyTo / MetaReferences / MetaSubject in
// email_adapter.go and MetaReplyTo in inbound.go.
const (
	emailMetaMessageID  = "message_id"
	emailMetaInReplyTo  = "in_reply_to"
	emailMetaReferences = "references"
	emailMetaSubject    = "subject"
	emailMetaReplyTo    = "reply_to"
)

// EmailReplyMetadata builds the metadata a human reply to an inbound email
// needs so the receiving client files it in the same thread: In-Reply-To
// points at the customer's Message-ID, References carries the chain plus
// that id, and — when the attendant typed no subject — the subject is the
// customer's with a single "Re: " prefix. A nil lastInbound (first outbound
// of the conversation) yields only the subject the caller gave, if any.
// Pure: no I/O, safe to unit-test.
func EmailReplyMetadata(lastInbound *domain.Message, subject string) map[string]any {
	out := map[string]any{}
	subject = strings.TrimSpace(subject)
	in := messageMetadataMap(lastInbound)

	if id := strings.TrimSpace(stringAt(in, emailMetaMessageID)); id != "" {
		out[emailMetaInReplyTo] = id
		refs := strings.TrimSpace(stringAt(in, emailMetaReferences))
		switch {
		case refs == "":
			refs = id
		case !strings.Contains(refs, id):
			refs += " " + id
		}
		out[emailMetaReferences] = refs
	}

	if subject == "" {
		if s := strings.TrimSpace(stringAt(in, emailMetaSubject)); s != "" {
			if !strings.HasPrefix(strings.ToLower(s), "re:") {
				s = "Re: " + s
			}
			subject = s
		}
	}
	if subject != "" {
		out[emailMetaSubject] = subject
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// EmailReplyRecipient picks where a human email reply goes: the customer's
// Reply-To when the inbound carried a valid one (shared mailboxes and
// ticketing systems set it deliberately), otherwise fallback — normally the
// conversation's contact id, i.e. the From address.
func EmailReplyRecipient(lastInbound *domain.Message, fallback string) string {
	rt := strings.TrimSpace(stringAt(messageMetadataMap(lastInbound), emailMetaReplyTo))
	if rt != "" && domain.IsEmailAddress(rt) {
		return rt
	}
	return fallback
}

// messageMetadataMap normalizes Message.Metadata — a map when built in
// memory, raw JSON when read back from the DB, nil when absent — into a map.
func messageMetadataMap(msg *domain.Message) map[string]any {
	if msg == nil || msg.Metadata == nil {
		return nil
	}
	var raw []byte
	switch v := msg.Metadata.(type) {
	case map[string]any:
		return v
	case json.RawMessage:
		raw = v
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return nil
	}
	m := map[string]any{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

func stringAt(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}
