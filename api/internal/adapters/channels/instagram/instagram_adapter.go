package instagram

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

const graphAPIBase = "https://graph.instagram.com/v21.0"

// instagramConfig is the expected JSON inside Channel.Config for Instagram channels.
type instagramConfig struct {
	PageID      string `json:"page_id"`
	AccessToken string `json:"access_token"`
}

// InstagramAdapter implements domain.MessageProvider for the Instagram Messaging API.
type InstagramAdapter struct {
	httpClient *http.Client
	baseURL    string
}

func NewInstagramAdapter() *InstagramAdapter {
	return &InstagramAdapter{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    graphAPIBase,
	}
}

// WithBaseURL re-points the Graph API host — tests aim it at an
// httptest.Server; production always uses the default. Returns the
// adapter for chaining.
func (a *InstagramAdapter) WithBaseURL(u string) *InstagramAdapter {
	a.baseURL = u
	return a
}

func parseConfig(channel *domain.Channel) (*instagramConfig, error) {
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

	var cfg instagramConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("channel %s: failed to parse config: %w", channel.ID, err)
	}
	if cfg.PageID == "" || cfg.AccessToken == "" {
		return nil, fmt.Errorf("channel %s: missing page_id or access_token in config", channel.ID)
	}
	return &cfg, nil
}

// SendMessage sends a message to an Instagram user via the Instagram Messaging API.
// `to` is the Instagram-scoped user ID (IGSID).
func (a *InstagramAdapter) SendMessage(ctx context.Context, channel *domain.Channel, to string, msg *domain.Message) error {
	cfg, err := parseConfig(channel)
	if err != nil {
		return err
	}

	msgType := msg.Type
	if msgType == "" {
		msgType = "text"
	}

	var payload map[string]interface{}

	switch msgType {
	// Meta's Instagram messaging API accepts image/audio/video
	// attachments. Anything else (document, sticker, location, ...)
	// has no attachment mapping and must NOT fall through to the text
	// branch — an empty {"text":""} is rejected by Meta with an opaque
	// 400 and the message silently dies.
	case "image", "audio", "video":
		payload = map[string]interface{}{
			"recipient": map[string]string{"id": to},
			"message": map[string]interface{}{
				"attachment": map[string]interface{}{
					"type": msgType,
					"payload": map[string]string{
						"url": deref(msg.MediaURL),
					},
				},
			},
		}
	default: // "text" and unsupported media types carrying a caption
		text := deref(msg.Content)
		if text == "" {
			return fmt.Errorf("instagram: message type %q with no text content cannot be sent (Meta rejects empty text and has no attachment type for it)", msgType)
		}
		payload = map[string]interface{}{
			"recipient": map[string]string{"id": to},
			"message": map[string]string{
				"text": text,
			},
		}
	}

	apiURL := fmt.Sprintf("%s/%s/messages", a.baseURL, cfg.PageID)
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("instagram: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.AccessToken)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("instagram: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("instagram: API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	slog.Info("message sent via Instagram", "type", msgType, "channel_id", channel.ID)
	return nil
}

// HandleWebhook implements domain.MessageProvider for Instagram.
func (a *InstagramAdapter) HandleWebhook(ctx context.Context, channel *domain.Channel, payload []byte) error {
	messages, err := ParseWebhookPayload(payload)
	if err != nil {
		return fmt.Errorf("instagram: failed to parse webhook: %w", err)
	}
	slog.Info("Instagram webhook received", "messages_count", len(messages), "channel_id", channel.ID)
	return nil
}

// FetchSenderName resolves an IGSID to a human-readable display name
// via the Graph API user-profile endpoint (needs the same
// instagram_manage_messages token used for sends). Prefers `name`,
// falls back to `username`; returns "" (no error) when the profile
// exposes neither. Callers treat failures as best-effort — a raw IGSID
// in the dashboard is worse UX, not a broken pipeline.
func (a *InstagramAdapter) FetchSenderName(ctx context.Context, channel *domain.Channel, igsid string) (string, error) {
	cfg, err := parseConfig(channel)
	if err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf("%s/%s?fields=name,username", a.baseURL, url.PathEscape(igsid))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("instagram: failed to create profile request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.AccessToken)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("instagram: profile request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return "", fmt.Errorf("instagram: profile API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var profile struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return "", fmt.Errorf("instagram: failed to decode profile: %w", err)
	}
	if profile.Name != "" {
		return profile.Name, nil
	}
	return profile.Username, nil
}

// ParseWebhook parses an Instagram webhook payload and extracts the sender and message text.
type ParsedMessage struct {
	SenderID   string
	MessageID  string // Meta's "mid"; used as external_message_id for webhook retry dedupe
	Text       string
	MediaURL   string
	MediaType  string
	SenderName string
}

// ParseWebhookPayload extracts messages from an Instagram webhook callback.
//
// Meta app webhook subscription requirements (documented here because the
// payload shape below depends on them): subscribe the app to the
// **Instagram** object with the `messages` field (Meta App Dashboard →
// Webhooks → Instagram → messages). The IG professional account must be
// linked to a Facebook Page and the access token needs the
// `instagram_manage_messages` permission. DMs arrive under
// entry[].messaging[] — the Messenger Platform envelope.
func ParseWebhookPayload(payload []byte) ([]ParsedMessage, error) {
	var webhook struct {
		Object string `json:"object"`
		Entry  []struct {
			ID        string `json:"id"`
			Time      int64  `json:"time"`
			Messaging []struct {
				Sender    struct{ ID string } `json:"sender"`
				Recipient struct{ ID string } `json:"recipient"`
				Timestamp int64               `json:"timestamp"`
				Message   *struct {
					MID         string `json:"mid"`
					Text        string `json:"text"`
					IsEcho      bool   `json:"is_echo"`
					Attachments []struct {
						Type    string `json:"type"`
						Payload struct {
							URL string `json:"url"`
						} `json:"payload"`
					} `json:"attachments"`
				} `json:"message"`
			} `json:"messaging"`
		} `json:"entry"`
	}

	if err := json.Unmarshal(payload, &webhook); err != nil {
		return nil, fmt.Errorf("instagram: failed to parse webhook: %w", err)
	}

	var messages []ParsedMessage
	for _, entry := range webhook.Entry {
		for _, m := range entry.Messaging {
			if m.Message == nil {
				continue
			}
			// Echo suppression: Meta reflects every message SENT BY the
			// page (including our own bot replies) back to the webhook
			// with message.is_echo=true. Treating an echo as a fresh
			// inbound would feed the bot its own reply — an infinite
			// send/receive loop. Drop them here so no caller can forget.
			if m.Message.IsEcho {
				continue
			}
			pm := ParsedMessage{
				SenderID:  m.Sender.ID,
				MessageID: m.Message.MID,
				Text:      m.Message.Text,
				MediaType: "text",
			}
			if len(m.Message.Attachments) > 0 {
				att := m.Message.Attachments[0]
				pm.MediaURL = att.Payload.URL
				pm.MediaType = att.Type
				if pm.Text == "" {
					pm.Text = "[" + att.Type + "]"
				}
			}
			messages = append(messages, pm)
		}
	}

	return messages, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// VerifySignature checks Meta's X-Hub-Signature-256 header against the raw
// webhook body using the app secret. Same contract as the WhatsApp
// adapter's VerifySignature: header must be exactly "sha256=" + 64 hex
// chars (sha256-hex is always 64 bytes) so a degenerate "sha256=" or
// oversized header can't pass the prefix check; prefix match is
// case-insensitive as a defense against proxies that uppercase header
// values. Package-level (not a method) because the inbound handler
// validates before any adapter instance is involved.
func VerifySignature(payload []byte, signatureHeader, appSecret string) bool {
	const prefix = "sha256="
	const hexLen = 64
	if len(signatureHeader) != len(prefix)+hexLen {
		return false
	}
	if !strings.EqualFold(signatureHeader[:len(prefix)], prefix) {
		return false
	}

	expectedSignature := signatureHeader[len(prefix):]

	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write(payload)
	calculatedSignature := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedSignature), []byte(calculatedSignature))
}
