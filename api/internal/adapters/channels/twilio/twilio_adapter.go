// Package twilio implements domain.MessageProvider for SMS via the
// Twilio Messages API (BYO credentials: the customer pastes their own
// Account SID + Auth Token + purchased number, so Twilio usage bills
// to THEIR account, not ours).
package twilio

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	twilioclient "github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"

	"github.com/replider/api/internal/core/domain"
)

// smsMaxBodyRunes is Twilio's hard cap on a Message body (error 21617
// above it). Segmented long messages are fine below the cap; anything
// past it is rejected by the API, so we truncate instead of failing
// the whole send.
const smsMaxBodyRunes = 1600

// twilioConfig is the expected JSON inside Channel.Config for SMS channels.
type twilioConfig struct {
	AccountSID string `json:"account_sid"`
	AuthToken  string `json:"auth_token"`
	FromNumber string `json:"from_number"`
}

// messageAPI is the slice of the generated Twilio Api service the
// adapter uses. Narrow so tests can swap in a client wired to an
// httptest server (the SDK pins baseURL to api.twilio.com).
type messageAPI interface {
	CreateMessage(params *openapi.CreateMessageParams) (*openapi.ApiV2010Message, error)
}

// TwilioAdapter implements domain.MessageProvider for SMS via Twilio.
type TwilioAdapter struct {
	// newMessageAPI builds a per-channel API client from the channel's
	// own credentials (BYO Twilio — every channel can point at a
	// different Twilio account). Overridable in tests.
	newMessageAPI func(accountSID, authToken string) messageAPI
}

func NewTwilioAdapter() *TwilioAdapter {
	return &TwilioAdapter{
		newMessageAPI: func(accountSID, authToken string) messageAPI {
			rc := twilioclient.NewRestClientWithParams(twilioclient.ClientParams{
				Username: accountSID,
				Password: authToken,
			})
			rc.Client.SetTimeout(30 * time.Second)
			return rc.Api
		},
	}
}

func parseConfig(channel *domain.Channel) (*twilioConfig, error) {
	if channel.Config == nil {
		return nil, fmt.Errorf("channel %s has no configuration", channel.ID)
	}

	var raw []byte
	switch v := channel.Config.(type) {
	case []byte:
		raw = v
	case json.RawMessage:
		raw = []byte(v)
	default:
		var err error
		raw, err = json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("channel %s: failed to marshal config: %w", channel.ID, err)
		}
	}

	var cfg twilioConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("channel %s: failed to parse config: %w", channel.ID, err)
	}
	if cfg.AccountSID == "" || cfg.AuthToken == "" || cfg.FromNumber == "" {
		return nil, fmt.Errorf("channel %s: missing account_sid, auth_token or from_number in config", channel.ID)
	}
	return &cfg, nil
}

// SendMessage sends an SMS (or MMS when the message carries media) to
// `to`, an E.164 phone number.
func (a *TwilioAdapter) SendMessage(ctx context.Context, channel *domain.Channel, to string, msg *domain.Message) error {
	cfg, err := parseConfig(channel)
	if err != nil {
		return err
	}
	if to == "" {
		return fmt.Errorf("twilio: empty recipient for channel %s", channel.ID)
	}

	params := &openapi.CreateMessageParams{}
	params.SetTo(to)
	params.SetFrom(cfg.FromNumber)

	body := deref(msg.Content)
	switch msg.Type {
	case "image", "video", "audio", "voice", "document", "sticker":
		// SMS has no native media bubbles; Twilio MMS takes the URL
		// (US/CA only) and the caption rides as the body.
		if mediaURL := deref(msg.MediaURL); mediaURL != "" {
			params.SetMediaUrl([]string{mediaURL})
		}
	case "location":
		// "lat,lon" content → a tappable maps link beats raw coordinates.
		if body != "" {
			body = "https://maps.google.com/?q=" + strings.ReplaceAll(body, " ", "")
		}
	}

	body, truncated := truncateBody(body)
	if truncated {
		slog.Warn("twilio: SMS body exceeded 1600-char Twilio cap, truncated",
			"channel_id", channel.ID)
	}
	if body != "" {
		params.SetBody(body)
	}
	if body == "" && params.MediaUrl == nil {
		return fmt.Errorf("twilio: message has no body or media for channel %s", channel.ID)
	}

	// The generated SDK doesn't thread contexts; the 30s client timeout
	// bounds the call. Honor an already-cancelled ctx before dialing.
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("twilio: %w", err)
	}

	resp, err := a.newMessageAPI(cfg.AccountSID, cfg.AuthToken).CreateMessage(params)
	if err != nil {
		return fmt.Errorf("twilio: send failed for channel %s: %w", channel.ID, err)
	}
	if resp.ErrorCode != nil && *resp.ErrorCode != 0 {
		return fmt.Errorf("twilio: send rejected for channel %s: error code %d", channel.ID, *resp.ErrorCode)
	}

	slog.Info("message sent via Twilio SMS",
		"type", msg.Type, "channel_id", channel.ID, "sid", deref(resp.Sid), "status", deref(resp.Status))
	return nil
}

// HandleWebhook satisfies domain.MessageProvider. Inbound SMS is parsed
// by the inbound webhook handler (Twilio posts form-encoded, not JSON),
// so this is a logging no-op kept for interface parity.
func (a *TwilioAdapter) HandleWebhook(ctx context.Context, channel *domain.Channel, payload []byte) error {
	slog.Info("Twilio webhook payload received", "channel_id", channel.ID, "bytes", len(payload))
	return nil
}

// truncateBody enforces Twilio's 1600-char Message cap: over-long
// bodies are cut on a rune boundary with a trailing ellipsis instead
// of bouncing the whole send with API error 21617.
func truncateBody(body string) (string, bool) {
	runes := []rune(body)
	if len(runes) <= smsMaxBodyRunes {
		return body, false
	}
	return string(runes[:smsMaxBodyRunes-1]) + "…", true
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
