package domain

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// ChannelTypeInternal is the in-product CS channel between the platform
// account and customer-account users. Delivery is the DB row + WebSocket
// broadcast — no external messaging API. Exactly one internal channel
// exists, owned by the platform account (created idempotently at startup
// when PLATFORM_ACCOUNT_ID is set); customers can never create or see one.
const ChannelTypeInternal = "internal"

// Channel represents an integration with an external messaging platform.
// A channel always belongs to an account. It may optionally be assigned to
// an agent for automatic AI responses. If AgentID is nil, inbound messages
// are still saved and shown in the dashboard, but no AI response is generated.
type Channel struct {
	ID           string      `json:"id"`
	AccountID    string      `json:"account_id"`
	AgentID      *string     `json:"agent_id,omitempty"`
	Type         string      `json:"type"` // "whatsapp" | "telegram" | "instagram" | "sms" | "internal"
	Name         string      `json:"name"`
	WebhookToken string      `json:"webhook_token"`
	Config       interface{} `json:"config" swaggertype:"object"` // phone_number, bot_token, etc.
	IsActive     bool        `json:"is_active"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`

	// IsFullyConfigured is a derived response-only field. True when the
	// channel has all credentials needed to actually talk to Meta (e.g.
	// WhatsApp needs phone_number_id + waba_id + access_token). Lets the
	// Flutter UI distinguish "stub channel" from "ready channel" without
	// reading masked config strings. Not persisted.
	IsFullyConfigured bool `json:"is_fully_configured,omitempty"`

	// TokenExpiresAt / TokenExpiringSoon are derived response-only
	// fields for channels whose access token dies of old age (Instagram
	// long-lived tokens last ~60 days). The stamp lives in the config
	// JSONB as `token_expires_at` (written when the token is set or
	// rotated); these surface it so the dashboard can flag "reconnect
	// soon" before inbound DMs silently stop. Not persisted as columns.
	TokenExpiresAt    *time.Time `json:"token_expires_at,omitempty"`
	TokenExpiringSoon bool       `json:"token_expiring_soon,omitempty"`

	// EmailWarmup is a derived response-only field for email channels: the
	// mailbox's warmup state and today's send budget, computed by the SAME
	// rules the send gate applies, so the screen and the refusal can never
	// disagree. Not persisted.
	EmailWarmup *EmailWarmupStatus `json:"email_warmup,omitempty"`

	// IMAP poll health, derived response-only fields for email channels
	// (read from email_imap_cursors on GET /channels). A failure count above
	// zero means replies are NOT being read — a revoked Google grant or a
	// changed password shows up here and nowhere else, so the dashboard can
	// say "reconnect" instead of the mailbox silently going quiet.
	IMAPLastError    string     `json:"imap_last_error,omitempty"`
	IMAPFailureCount int        `json:"imap_failure_count,omitempty"`
	IMAPLastPolledAt *time.Time `json:"imap_last_polled_at,omitempty"`
}

// InstagramTokenTTL is how long a freshly-issued Instagram long-lived
// access token remains valid. Meta does not deliver the expiry via
// webhook, so the API stamps `token_expires_at = now + InstagramTokenTTL`
// into the channel config whenever the token is set or rotated.
const InstagramTokenTTL = 60 * 24 * time.Hour

// ValidChannelType is the allow-list of channel types the platform
// knows how to route. `channels.provider` is a plain VARCHAR (no DB
// CHECK), so this is the only gate keeping garbage types out — a typo'd
// type would create a channel no adapter ever serves.
func ValidChannelType(t string) bool {
	switch t {
	case "whatsapp", "telegram", "instagram", "sms", "webchat", "email":
		return true
	}
	return false
}

var (
	e164Re          = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)
	twilioAcctSIDRe = regexp.MustCompile(`^AC[0-9a-fA-F]{32}$`)
)

// IsE164 reports whether s is a full international phone number
// (+, country code, up to 15 digits) — the format Twilio requires for
// from_number and the only shape the SMS adapter can dial.
func IsE164(s string) bool {
	return e164Re.MatchString(s)
}

// IsTwilioAccountSID reports whether s has the shape of a Twilio
// Account SID ("AC" + 32 hex chars). Catches the common paste mistakes
// (API Key SIDs start with "SK", Auth Tokens have no prefix) before a
// send fails opaquely upstream.
func IsTwilioAccountSID(s string) bool {
	return twilioAcctSIDRe.MatchString(s)
}

// SensitiveConfigKey reports whether a channel-config key holds a
// credential. Sensitive keys are masked in every API response and,
// on update, a masked/empty/missing value keeps the stored credential
// instead of overwriting it (see UpdateChannelUseCase). Matches by
// suffix so new channel types get the protection for free: bot_token,
// access_token, auth_token, app_secret, api_key, password, smtp_password.
//
// The `_password` suffix matters: the email channel (US-M003) stores its
// SMTP credential as `smtp_password`, and an exact-match-only rule left it
// unmasked in every channel read, unencrypted at rest, and overwritable by
// a client echoing back what it fetched.
func SensitiveConfigKey(key string) bool {
	return key == "password" ||
		strings.HasSuffix(key, "_password") ||
		strings.HasSuffix(key, "_token") ||
		strings.HasSuffix(key, "_secret") ||
		strings.HasSuffix(key, "_key")
}

// ChannelHas24hWindow reports whether a channel type is subject to a
// platform-enforced 24h customer-service window (free-form outbound
// only within 24h of the customer's last inbound message). Meta
// enforces it on WhatsApp AND Instagram DMs; SMS (Twilio), Telegram
// and internal channels have no such upstream restriction, so human
// replies on them must never be blocked by the window gate.
func ChannelHas24hWindow(channelType string) bool {
	switch channelType {
	case "whatsapp", "instagram":
		return true
	default:
		return false
	}
}

// Message represents a normalized message across all channels
type Message struct {
	ID             string      `json:"id"`
	ChannelID      string      `json:"channel_id"`
	ConversationID *string     `json:"conversation_id,omitempty"`
	From           string      `json:"sender"`                // 'customer', 'bot', user ULID
	SenderName     *string     `json:"sender_name,omitempty"` // display name
	Type           string      `json:"type"`                  // 'text', 'image', 'video', 'audio', 'voice', 'document', 'sticker', 'location'
	Content        *string     `json:"content"`
	MediaURL       *string     `json:"media_url,omitempty"` // CDN / signed URL for media files
	Metadata       interface{} `json:"metadata" swaggertype:"object"`
	Direction      string      `json:"direction"` // 'inbound' or 'outbound'
	// ExternalMessageID is the provider-side id (e.g. WhatsApp wamid) used
	// to dedupe webhook retries. Nil for outbound messages and for channels
	// that don't expose stable ids (legacy Telegram rows, seeded data).
	ExternalMessageID *string    `json:"external_message_id,omitempty"`
	ReadAt            *time.Time `json:"read_at,omitempty"` // when attendant read this message
	// Transcript is the cached Whisper/Gemini output for audio/voice
	// messages. Nil when the attendant hasn't requested transcription yet;
	// empty string when the transcriber ran but returned nothing so we
	// don't keep retrying the same silent clip.
	Transcript *string   `json:"transcript,omitempty"`
	Timestamp  time.Time `json:"created_at"`
	// US-030: per-message AI token accounting + estimated provider cost.
	// Populated only on AI-generated outbound messages; nil everywhere
	// else (inbound, human replies, system messages). Token counts come
	// from the provider's usage report with a len(text)/4 fallback;
	// ProviderCostUSD is a coarse list-price estimate (see services.ProviderCostUSD).
	InputTokens     *int32   `json:"input_tokens,omitempty"`
	OutputTokens    *int32   `json:"output_tokens,omitempty"`
	ProviderCostUSD *float64 `json:"provider_cost_usd,omitempty"`
	// US-042: true when the input/output token counts came from the
	// len(text)/4 fallback because the provider omitted usage (e.g. a
	// truncated streamed response). Lets A/B attribution + billing discount
	// estimates instead of trusting them as exact provider counts. Nil on
	// non-AI messages and on rows written before the flag existed.
	TokenEstimated *bool `json:"token_estimated,omitempty"`
	// FailedAt/FailedReason surface an outbound send failure (SDK/HTTP
	// layer or a later delivery-status webhook) so the dashboard bubble
	// can render "send failed" instead of silently looking delivered.
	// Nil on inbound rows and on outbound rows that sent fine.
	FailedAt     *time.Time `json:"failed_at,omitempty"`
	FailedReason *string    `json:"failed_reason,omitempty"`
}

// MessageAttempt is one provider-side send attempt for a parent
// messages row. The bot reply pipeline splits an AI reply into 2-4
// bubbles and sends each as a separate WhatsApp message, generating
// distinct wamids; the parent row only carries one external_message_id
// so per-bubble status correlation lives here instead.
type MessageAttempt struct {
	ID                string
	MessageID         string
	ChannelID         string
	ExternalMessageID *string
	BubbleIndex       int16
	SentAt            *time.Time
	DeliveredAt       *time.Time
	ReadAt            *time.Time
	FailedAt          *time.Time
	FailedReason      *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// WhatsAppStatusUpdate carries the parsed Cloud API status envelope
// the inbound webhook handler applies to outbound messages — delivered
// / read timestamps, failure reason, and the conversation/pricing
// metadata Meta attaches once per 24h window.
type WhatsAppStatusUpdate struct {
	ChannelID         string
	ExternalMessageID string
	// SentAt is when Meta confirmed the outbound was accepted into the
	// pipeline (`status = sent`). Distinct from messages.created_at
	// (which is when WE queued the row) — the gap matters for the
	// dashboard's "send rate" timeline and for support investigating a
	// "stuck on queue" complaint.
	SentAt      *time.Time
	DeliveredAt *time.Time
	ReadAt      *time.Time
	// FailedAt is the moment Meta rejected the outbound. Without it
	// the dashboard could only show the failure reason but not when
	// it happened, making "did this fail before or after the customer
	// answered" investigations impossible.
	FailedAt                *time.Time
	FailedReason            string
	WAConversationID        string
	WAConversationExpiresAt *time.Time
	ConversationOrigin      string // marketing/utility/authentication/service/referral_conversion
	PricingCategory         string
	PricingBillable         *bool
}

// MessageProvider defines the "Plug-and-Play" interface for channels (Adapter Pattern)
// ChannelConfigString reads one string field out of a channel's config.
//
// In domain because three layers now need it — the approach sender, the mailbox
// poller and the DNS screen — and each had grown its own copy that decoded the
// blob slightly differently. The config arrives as raw JSON, a byte slice or an
// already-decoded map depending on which repository read it, so the shapes are
// handled here once.
func ChannelConfigString(channel *Channel, key string) string {
	if channel == nil || channel.Config == nil {
		return ""
	}
	var m map[string]any
	switch v := channel.Config.(type) {
	case map[string]any:
		m = v
	case []byte:
		if json.Unmarshal(v, &m) != nil {
			return ""
		}
	case json.RawMessage:
		if json.Unmarshal(v, &m) != nil {
			return ""
		}
	default:
		raw, err := json.Marshal(v)
		if err != nil || json.Unmarshal(raw, &m) != nil {
			return ""
		}
	}
	s, _ := m[key].(string)
	return strings.TrimSpace(s)
}

type MessageProvider interface {
	SendMessage(ctx context.Context, channel *Channel, to string, msg *Message) error
	HandleWebhook(ctx context.Context, channel *Channel, payload []byte) error
}

// EventBroadcaster is the minimal port that use cases use to push
// real-time updates to connected dashboard clients. The adapter side
// (internal/adapters/http.Hub) satisfies it; keeping it in domain
// lets use cases broadcast without importing the HTTP layer.
type EventBroadcaster interface {
	BroadcastToAccount(accountID string, payload []byte)
}

// Email channel helpers (US-M003).

var emailAddressRe = regexp.MustCompile(`^[^\s@]+@[^\s@.]+(\.[^\s@.]+)+$`)

// IsEmailAddress is the shape check applied to a channel's from_address.
// Deliberately permissive about exotic-but-legal addresses and strict about
// the shapes that break authentication (spaces, no domain, no TLD).
func IsEmailAddress(s string) bool {
	return emailAddressRe.MatchString(strings.TrimSpace(strings.ToLower(s)))
}

// freeMailDomains are consumer mailbox providers. Cold outbound from one
// cannot pass DMARC alignment for a domain the customer owns — the mail
// lands in spam and drags down the provider's shared reputation, which is
// why the channel refuses them outright rather than warning.
var freeMailDomains = map[string]struct{}{
	"gmail.com": {}, "googlemail.com": {}, "hotmail.com": {}, "outlook.com": {},
	"live.com": {}, "msn.com": {}, "yahoo.com": {}, "yahoo.com.br": {},
	"icloud.com": {}, "me.com": {}, "aol.com": {}, "proton.me": {},
	"protonmail.com": {}, "bol.com.br": {}, "uol.com.br": {}, "terra.com.br": {},
	"ig.com.br": {}, "globo.com": {},
}

// IsFreeMailProvider reports whether the address belongs to a consumer
// mailbox provider.
func IsFreeMailProvider(address string) bool {
	at := strings.LastIndex(address, "@")
	if at < 0 {
		return false
	}
	domain := strings.ToLower(strings.TrimSpace(address[at+1:]))
	_, ok := freeMailDomains[domain]
	return ok
}

// EmailDomain returns the domain part of an address, lowercased. Used by the
// DNS setup flow (US-M006) and the per-domain deliverability caps (US-M007).
func EmailDomain(address string) string {
	at := strings.LastIndex(address, "@")
	if at < 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(address[at+1:]))
}

// AccountCanSend reports whether any of these channels is a door an offer can
// leave through.
//
// One predicate, two callers: the approval queue's banner and the drafting
// gate. Two places deciding "can this account send?" is how a screen ends up
// promising something the engine refuses — the exact class of bug the
// 2026-09-09 audit found twice over.
//
// The internal channel is excluded because it is the CS thread with the
// Replider platform, not a way to reach a hospital. An inactive channel is
// excluded because a paused door is a closed one.
func AccountCanSend(channels []*Channel) bool {
	for _, ch := range channels {
		if ch != nil && ch.IsActive && ch.Type != ChannelTypeInternal {
			return true
		}
	}
	return false
}

// CanReachContact reports whether any of these channels has an address on
// this contact it could actually deliver to.
//
// AccountCanSend answers "is there a door?"; this answers "does this
// particular envelope have an address?", and production proved the second
// question is not implied by the first. One account carries 70 contacts, 52 of
// them with a phone and **none** with an e-mail — so a mailbox-only account
// would pass AccountCanSend and still be writing letters nobody can post. Half
// its queue was that: finished offers, correct text, no recipient.
//
// Only e-mail and phone are cold-outbound addresses. Telegram, Instagram and
// webchat can answer a thread the person opened but cannot start one, so they
// do not make a stranger reachable; a contact who did write in has a phone or
// an address on file by then, which the last branch covers.
//
// Errs toward reachable, like every other drafting gate: with no channels
// listed (an unwired reader, a failed lookup) the caller skips the check
// entirely rather than blocking a working account.
func CanReachContact(channels []*Channel, c *Contact) bool {
	if c == nil {
		return false
	}
	hasEmail := c.Email != nil && strings.TrimSpace(*c.Email) != ""
	hasPhone := c.Phone != nil && strings.TrimSpace(*c.Phone) != ""
	if !hasEmail && !hasPhone {
		return false
	}
	for _, ch := range channels {
		if ch == nil || !ch.IsActive || ch.Type == ChannelTypeInternal {
			continue
		}
		switch ch.Type {
		case "email":
			if hasEmail {
				return true
			}
		case "whatsapp", "sms":
			if hasPhone {
				return true
			}
		default:
			return true
		}
	}
	return false
}

// QueueBlockedNoChannel is the approval queue's reason code for an account
// with nowhere to send from.
//
// A code rather than a sentence: the screen owns the words, and the same
// fact has to render in two languages. It is a REASON, not an error — the
// drafts are real and worth reading, they just cannot leave yet.
const QueueBlockedNoChannel = "NO_CHANNEL"

// QueueBlockedNoAddress is the same kind of reason, one card down: the
// account can send, this particular recipient has nowhere to receive.
//
// Per-card rather than per-queue because it is per-card work to fix — find
// this buyer's e-mail — and because a queue is usually a mix. Rendering it as
// an account-level banner would tell someone with 37 sendable offers that
// nothing can go out.
const QueueBlockedNoAddress = "NO_ADDRESS"

// UndeliverableQueueCounts is how much of an approval queue has nowhere to be
// posted, counted once per kind of missing address.
//
// Three numbers rather than one because "undeliverable" is not a property of
// the lead: it is the lead's addresses crossed with the account's channels.
// UndeliverableWith does that crossing.
type UndeliverableQueueCounts struct {
	// NoAddress: neither an e-mail nor a phone. Unreachable through any
	// channel that exists.
	NoAddress int64 `json:"no_address"`
	NoEmail   int64 `json:"no_email"`
	NoPhone   int64 `json:"no_phone"`
	// Pending is the whole queue, so a caller can say "37 of 50".
	Pending int64 `json:"pending"`
}

// UndeliverableWith applies these counts to the channels an account actually
// has, and reports how many queued drafts cannot go out.
//
// A mailbox-only account is blocked by every contact with no e-mail; a
// WhatsApp-only one by every contact with no phone; an account with both only
// by contacts with neither. With no usable channel at all every draft is
// stuck, which the account-level gate already says — so this returns 0 and
// lets that message stand rather than saying the same thing twice.
func (c UndeliverableQueueCounts) UndeliverableWith(channels []*Channel) int64 {
	email, phone := false, false
	for _, ch := range channels {
		if ch == nil || !ch.IsActive || ch.Type == ChannelTypeInternal {
			continue
		}
		switch ch.Type {
		case "email":
			email = true
		case "whatsapp", "sms":
			phone = true
		default:
			// A reply-only channel reaches whoever already wrote in, and
			// those contacts carry an address. Treat it as covering both
			// rather than inventing a fourth count.
			email, phone = true, true
		}
	}
	switch {
	case email && phone:
		return c.NoAddress
	case email:
		return c.NoEmail
	case phone:
		return c.NoPhone
	default:
		return 0
	}
}
