package whatsapp

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
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/replider/api/internal/adapters/httpsafe"
	mediautil "github.com/replider/api/internal/adapters/media"
	"github.com/replider/api/internal/core/domain"
)

const metaAPIVersion = "v25.0"

// metaGraphBaseURL is the Meta Graph API host. Exposed as a package var so
// tests can point it at an httptest.Server. NEVER mutate in production.
var metaGraphBaseURL = "https://graph.facebook.com"

// whatsappConfig represents the expected JSON structure inside Channel.Config
type whatsappConfig struct {
	PhoneNumberID string `json:"phone_number_id"`
	AccessToken   string `json:"access_token"`
}

// WhatsAppAdapter implements domain.MessageProvider for Meta's WhatsApp Cloud API
type WhatsAppAdapter struct {
	httpClient *http.Client
	// mediaClient fetches media at a URL the CUSTOMER supplied (an
	// attendant's reply). Built by httpsafe so the dial refuses internal
	// addresses; httpClient is reserved for Meta's own API.
	mediaClient    *http.Client
	appAccessToken string // app-level token from WHATSAPP_ACCESS_TOKEN env var (BSP mode)
	pairLimiter    *PairRateLimiter
	// optOut, when set, is consulted before EVERY outbound send (text or
	// template). A recipient that has opted out aborts the send before any
	// network call to Meta, returning domain.ErrContactOptedOut (US-025).
	// Optional — nil disables the guard (e.g. unit tests that don't exercise
	// opt-out). Wired in main.go from the ContactRepository.
	optOut OptOutChecker
}

// NewWhatsAppAdapter creates a new instance of the adapter.
// appAccessToken is the app-level System User token used when the channel config
// does not include its own access_token (tech provider / BSP mode).
func NewWhatsAppAdapter(appAccessToken string) *WhatsAppAdapter {
	return &WhatsAppAdapter{
		httpClient:     metaHTTPClient,
		mediaClient:    httpsafe.Client(60 * time.Second),
		appAccessToken: appAccessToken,
		// Default per-pair limiter prevents Meta error 131056 when a
		// runaway loop in our own code sends many messages to one
		// recipient. Capacity 6 / 1s refill matches Meta's documented
		// per-pair pacing without pushing too close.
		pairLimiter: NewPairRateLimiter(6, 1*time.Second),
	}
}

// SetPairLimiter overrides the default limiter — useful in tests
// (faster refill) or when an operator wants tighter / looser pacing
// per deployment.
func (a *WhatsAppAdapter) SetPairLimiter(l *PairRateLimiter) {
	a.pairLimiter = l
}

// parseConfig extracts WhatsApp credentials from channel.Config (JSONB stored as []byte or map).
func parseConfig(channel *domain.Channel) (*whatsappConfig, error) {
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

	var cfg whatsappConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("channel %s: failed to parse config: %w", channel.ID, err)
	}

	if cfg.PhoneNumberID == "" {
		return nil, fmt.Errorf("channel %s: missing phone_number_id in config", channel.ID)
	}

	return &cfg, nil
}

// resolveAccessToken returns the access token to use for API calls.
// If the channel config has its own access_token, use it (backward compat).
// Otherwise, fall back to the app-level token (BSP / tech provider mode).
func (a *WhatsAppAdapter) resolveAccessToken(cfg *whatsappConfig) (string, error) {
	if cfg.AccessToken != "" {
		return cfg.AccessToken, nil
	}
	if a.appAccessToken != "" {
		return a.appAccessToken, nil
	}
	return "", fmt.Errorf("whatsapp: no access token available (neither in channel config nor app-level)")
}

// SendMessage sends a message via the Meta WhatsApp Cloud API. The
// returned wamid is discarded for interface compatibility with other
// channels; callers that need it (e.g. to correlate later status
// webhooks back to the outbound row) should use SendMessageAndGetID.
func (a *WhatsAppAdapter) SendMessage(ctx context.Context, channel *domain.Channel, to string, msg *domain.Message) error {
	_, err := a.SendMessageAndGetID(ctx, channel, to, msg)
	return err
}

// metadataMap normalizes the heterogeneous shapes domain.Message.Metadata
// can take (map, json.RawMessage, []byte from sqlc). Returns an empty
// map on any decode failure — callers always read fields via metaString
// so a nil-safe map is enough.
func metadataMap(m interface{}) map[string]any {
	if m == nil {
		return nil
	}
	switch v := m.(type) {
	case map[string]any:
		return v
	case []byte:
		var out map[string]any
		_ = json.Unmarshal(v, &out)
		return out
	case json.RawMessage:
		var out map[string]any
		_ = json.Unmarshal(v, &out)
		return out
	}
	// Last resort: re-marshal then unmarshal so anonymous structs are
	// reachable. Acceptable since sends are not on a hot loop.
	raw, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}

// metaString reads a string field out of the metadata map, treating
// missing keys and non-string values as "".
func metaString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// textBody builds a Cloud API text payload, opting into URL preview
// when the body actually contains an http(s) URL. Default-disabling
// preview_url loses every link card in outbound bot replies, which the
// design previously did to "play it safe" but the cost is silent UX
// degradation in every reply that includes a URL.
func textBody(body string) map[string]any {
	out := map[string]any{"body": body}
	if containsURL(body) {
		out["preview_url"] = true
	}
	return out
}

func containsURL(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "http://") || strings.Contains(lower, "https://")
}

// normalizeWAPhone strips formatting characters from a recipient phone
// so the Cloud API receives the bare E.164 digits Meta expects. Common
// inputs from contact lists / dashboards are "+55 11 99999-9999" or
// "(11) 99999-9999"; Meta rejects either with code 131009 (invalid
// parameter) and the customer never gets the message.
func normalizeWAPhone(p string) string {
	if p == "" {
		return ""
	}
	out := make([]byte, 0, len(p))
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c >= '0' && c <= '9' {
			out = append(out, c)
		}
	}
	return string(out)
}

// SendMessageAndGetID is the WhatsApp-specific send entrypoint that
// surfaces Meta's `messages[0].id` (wamid). Without the wamid, the
// status update webhook (delivered / read / failed) cannot be matched
// to the outbound row — read receipts and the 24h window indicator
// silently break for every non-template outbound message.
//
// Supports all media types: text, image, audio, voice, video, document, location.
func (a *WhatsAppAdapter) SendMessageAndGetID(ctx context.Context, channel *domain.Channel, to string, msg *domain.Message) (string, error) {
	cfg, err := parseConfig(channel)
	if err != nil {
		return "", err
	}
	to = normalizeWAPhone(to)
	if to == "" {
		return "", fmt.Errorf("whatsapp: empty recipient phone after normalization")
	}
	// Anti-ban opt-out guard (US-025): abort BEFORE any Meta call when the
	// recipient has opted out. Checked on the normalized phone so it matches
	// the digits-only external_id stored on the contact row.
	if err := a.ensureNotOptedOut(ctx, channel, to); err != nil {
		return "", err
	}
	// Per-pair pacing: prevent a runaway loop in our code from
	// burning through the customer's quota with Meta's 131056 pair
	// rate limit. The limiter is in-memory (per-instance, not
	// global) — campaign workers must still pace at the
	// orchestration layer.
	if !a.pairLimiter.Allow(channel.ID, to) {
		return "", &MetaError{Code: 131056, Message: "pair rate limit (in-process limiter)"}
	}

	apiURL := fmt.Sprintf("%s/%s/%s/messages", metaGraphBaseURL, metaAPIVersion, cfg.PhoneNumberID)

	accessToken, err := a.resolveAccessToken(cfg)
	if err != nil {
		return "", err
	}

	msgType := msg.Type
	if msgType == "" {
		msgType = "text"
	}

	base := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                to,
	}

	var payload map[string]interface{}

	meta := metadataMap(msg.Metadata)

	switch msgType {
	case "image":
		mediaObj := map[string]string{"link": deref(msg.MediaURL)}
		if deref(msg.Content) != "" {
			mediaObj["caption"] = deref(msg.Content)
		}
		payload = base
		payload["type"] = "image"
		payload["image"] = mediaObj

	case "audio", "voice":
		// Voice notes recorded by the dashboard's MediaRecorder ship as
		// webm/opus on web (browsers don't expose AAC encoding) but are
		// uploaded with content-type audio/mp4. Meta validates the link
		// and rejects on the codec/container mismatch, so the customer
		// never receives the audio. Same fix Telegram already has:
		// fetch the bytes, transcode to OGG-Opus, upload via
		// /{phone-number-id}/media to obtain a media_id Meta owns, and
		// send `audio.id` instead of `audio.link`. Fallback to the
		// original link path on any local failure so we don't regress
		// senders that already work (mobile-recorded m4a, hosted CDN
		// URLs Meta accepts directly).
		rawURL := deref(msg.MediaURL)
		payload = base
		payload["type"] = "audio"
		if mediaID, err := a.transcodeAndUploadAudio(ctx, cfg, accessToken, rawURL); err == nil {
			payload["audio"] = map[string]string{"id": mediaID}
		} else {
			slog.Warn("whatsapp: audio transcode/upload failed, falling back to link",
				"channel_id", channel.ID, "error", err)
			payload["audio"] = map[string]string{"link": rawURL}
		}

	case "video":
		mediaObj := map[string]string{"link": deref(msg.MediaURL)}
		if deref(msg.Content) != "" {
			mediaObj["caption"] = deref(msg.Content)
		}
		payload = base
		payload["type"] = "video"
		payload["video"] = mediaObj

	case "document":
		// filename is a Cloud-API-specific field that controls the name
		// the recipient sees in WhatsApp's file UI. It is NOT a caption.
		// We pull it from metadata.filename so msg.Content can carry an
		// actual caption (the previous behavior overloaded Content as
		// filename, making captions impossible and rendering captions
		// the AI emitted as bizarre file names).
		docObj := map[string]string{"link": deref(msg.MediaURL)}
		if filename := metaString(meta, "filename"); filename != "" {
			docObj["filename"] = filename
		}
		if deref(msg.Content) != "" {
			docObj["caption"] = deref(msg.Content)
		}
		payload = base
		payload["type"] = "document"
		payload["document"] = docObj

	case "location":
		parts := strings.SplitN(deref(msg.Content), ",", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("whatsapp: invalid location format %q, expected 'lat,lon'", deref(msg.Content))
		}
		lat, errLat := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		lon, errLon := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if errLat != nil || errLon != nil {
			return "", fmt.Errorf("whatsapp: failed to parse location coordinates from %q", deref(msg.Content))
		}
		payload = base
		payload["type"] = "location"
		payload["location"] = map[string]float64{"latitude": lat, "longitude": lon}

	case "reaction":
		// Reactions are scoped to a previously-received wamid. Empty
		// emoji removes a prior reaction (Meta's documented behavior).
		// Metadata carries the target wamid so the call site doesn't
		// have to overload Content / MediaURL.
		targetWamid := metaString(meta, "wa_reaction_to")
		if targetWamid == "" {
			return "", fmt.Errorf("whatsapp: reaction send needs metadata.wa_reaction_to")
		}
		payload = base
		payload["type"] = "reaction"
		payload["reaction"] = map[string]string{
			"message_id": targetWamid,
			"emoji":      deref(msg.Content),
		}

	case "template":
		// Outbound template send (US-019). The template name, language,
		// category, per-recipient variables (filled from lead/contact
		// data) and button definitions ride on msg.Metadata. The builder
		// enforces the anti-ban opt-out guard, so a marketing/approach
		// template without an opt-out quick-reply button is rejected here,
		// before any network call.
		tmpl, err := buildTemplatePayload(parseTemplateSpecFromMeta(meta))
		if err != nil {
			return "", err
		}
		payload = base
		payload["type"] = "template"
		payload["template"] = tmpl

	case "interactive":
		// Interactive messages (buttons/lists) are sent via metadata JSON
		if msg.Metadata != nil {
			raw, _ := json.Marshal(msg.Metadata)
			var interactive map[string]interface{}
			if json.Unmarshal(raw, &interactive) == nil {
				payload = base
				payload["type"] = "interactive"
				payload["interactive"] = interactive
				break
			}
		}
		// Fallback to text if no valid interactive payload
		payload = base
		payload["type"] = "text"
		payload["text"] = textBody(deref(msg.Content))

	default: // "text"
		payload = base
		payload["type"] = "text"
		payload["text"] = textBody(deref(msg.Content))
	}

	// Reply-to: when the caller stashes the inbound wamid in
	// metadata.wa_reply_to, set the Cloud API context envelope so the
	// outbound bubble renders as a quoted reply in the customer's
	// thread. Useful for split bot replies that should clearly attach
	// to the question that triggered them.
	if replyTo := metaString(meta, "wa_reply_to"); replyTo != "" {
		payload["context"] = map[string]string{"message_id": replyTo}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("whatsapp: failed to marshal message payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("whatsapp: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("whatsapp: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		metaErr := ParseMetaError(resp.StatusCode, respBody)
		logMetaSendError(channel.ID, cfg.PhoneNumberID, msgType, metaErr, respBody)
		return "", metaErr
	}

	// Pull `messages[0].id` (wamid). Without it, the status update
	// webhook for this outbound — sent / delivered / read / failed —
	// has nothing to correlate against and the dashboard's delivery
	// ticks + failed_reason silently break.
	wamid := extractFirstMessageID(respBody)

	slog.Info("message sent via WhatsApp",
		"type", msgType,
		"channel_id", channel.ID,
		"phone_number_id", cfg.PhoneNumberID,
		"wamid", wamid)
	return wamid, nil
}

// transcodeAndUploadAudio fetches audio bytes from rawURL, transcodes
// to OGG-Opus, and uploads via Meta's /{phone-number-id}/media endpoint
// to obtain a media_id callable as `audio.id` on the messages payload.
// Returns an error on any leg so the caller can fall back to the
// original `audio.link` path. ffmpeg is already in the API container
// (Telegram's voice-note transcode shipped first).
func (a *WhatsAppAdapter) transcodeAndUploadAudio(
	ctx context.Context,
	cfg *whatsappConfig,
	accessToken string,
	rawURL string,
) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("whatsapp: audio message has no media URL")
	}
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("whatsapp: build media fetch: %w", err)
	}
	getResp, err := a.mediaClient.Do(getReq)
	if err != nil {
		return "", fmt.Errorf("whatsapp: fetch media: %w", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode >= 400 {
		return "", fmt.Errorf("whatsapp: fetch media: status %d", getResp.StatusCode)
	}
	audioBytes, err := io.ReadAll(getResp.Body)
	if err != nil {
		return "", fmt.Errorf("whatsapp: read media: %w", err)
	}

	transcoded, err := mediautil.TranscodeToOggOpus(ctx, audioBytes)
	if err != nil {
		return "", fmt.Errorf("whatsapp: transcode to ogg-opus: %w", err)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("messaging_product", "whatsapp"); err != nil {
		return "", fmt.Errorf("whatsapp: write messaging_product: %w", err)
	}
	if err := mw.WriteField("type", "audio/ogg"); err != nil {
		return "", fmt.Errorf("whatsapp: write type: %w", err)
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="voice.ogg"`)
	h.Set("Content-Type", "audio/ogg")
	part, err := mw.CreatePart(h)
	if err != nil {
		return "", fmt.Errorf("whatsapp: create multipart part: %w", err)
	}
	if _, err := part.Write(transcoded); err != nil {
		return "", fmt.Errorf("whatsapp: write audio bytes: %w", err)
	}
	if err := mw.Close(); err != nil {
		return "", fmt.Errorf("whatsapp: close multipart: %w", err)
	}

	uploadURL := fmt.Sprintf("%s/%s/%s/media", metaGraphBaseURL, metaAPIVersion, cfg.PhoneNumberID)
	upReq, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &body)
	if err != nil {
		return "", fmt.Errorf("whatsapp: build media upload: %w", err)
	}
	upReq.Header.Set("Authorization", "Bearer "+accessToken)
	upReq.Header.Set("Content-Type", mw.FormDataContentType())

	upResp, err := a.httpClient.Do(upReq)
	if err != nil {
		return "", fmt.Errorf("whatsapp: media upload: %w", err)
	}
	defer upResp.Body.Close()
	upBody, _ := io.ReadAll(upResp.Body)
	if upResp.StatusCode >= 400 {
		return "", ParseMetaError(upResp.StatusCode, upBody)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(upBody, &out); err != nil || out.ID == "" {
		return "", fmt.Errorf("whatsapp: parse media upload response: %s", string(upBody))
	}
	slog.Info("whatsapp audio media uploaded",
		"phone_number_id", cfg.PhoneNumberID, "media_id", out.ID, "bytes", len(transcoded))
	return out.ID, nil
}

// extractFirstMessageID pulls `messages[0].id` from a Cloud API send
// response. Returns "" when the payload is malformed or absent — the
// caller treats missing wamid as a non-fatal logging gap.
func extractFirstMessageID(body []byte) string {
	var resp struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return ""
	}
	if len(resp.Messages) == 0 {
		return ""
	}
	return resp.Messages[0].ID
}

// HandleWebhook implements the domain.MessageProvider interface for incoming webhooks
func (a *WhatsAppAdapter) HandleWebhook(ctx context.Context, channel *domain.Channel, payload []byte) error {
	var webhookData WebhookPayload
	if err := json.Unmarshal(payload, &webhookData); err != nil {
		return fmt.Errorf("failed to parse whatsapp webhook: %w", err)
	}

	if webhookData.Object != "whatsapp_business_account" {
		return fmt.Errorf("unsupported webhook object type: %s", webhookData.Object)
	}

	for _, entry := range webhookData.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}

			for _, msg := range change.Value.Messages {
				slog.Info("WhatsApp message received", "message_id", msg.ID, "type", msg.Type)
			}

			for _, status := range change.Value.Statuses {
				slog.Info("WhatsApp message status update", "message_id", status.ID, "status", status.Status)
			}
		}
	}

	return nil
}

// ParsedInbound holds the normalized data extracted from one WhatsApp Cloud API message.
type ParsedInbound struct {
	PhoneNumberID string // identifies which channel received the message
	From          string // sender's WhatsApp phone number (e.g. "5511999999999")
	SenderName    string // display name from contacts array
	MessageID     string // WhatsApp message ID (wamid.XXX) — needed for read receipts
	MessageType   string // "text", "image", "audio", "voice", "video", "document", "sticker", "location", "reaction", "contacts", "order", "system"
	Text          string // text body or caption (or human-readable summary for non-text types)
	MediaID       string // Meta media object ID (resolve via GetMediaURL)
	LatLon        string // "lat,lon" for location messages
	// Metadata is a JSON blob persisted on the message row. Used to
	// retain structured detail for non-text inbound (reaction emoji,
	// vCard fields, order line items, NFM form payload, system event
	// type) without polluting the primary content column.
	Metadata []byte
	// Skip signals that the parser recognized the inbound but the
	// pipeline should NOT run AI / billing / fanout (e.g. system
	// events, parser errors). The handler should still ack 200 to
	// stop Meta from retrying.
	Skip bool
}

// ParseInbound extracts the first actionable message from a WhatsApp Cloud API webhook payload.
// Returns (nil, false) for delivery status updates and other non-message events.
//
// Kept for backward compatibility with single-message call sites (older
// tests, ad-hoc tools). Production webhook handling MUST use ParseInboundAll
// — Meta batches up to 100 messages per delivery and only the first
// would otherwise be processed.
func ParseInbound(body []byte) (*ParsedInbound, bool) {
	all := ParseInboundAll(body)
	if len(all) == 0 {
		return nil, false
	}
	return all[0], true
}

// ParseInboundAll extracts EVERY message from a WhatsApp Cloud API webhook
// payload. Meta delivers up to 100 messages in a single POST under
// `entry[].changes[].value.messages[]`; processing only the first
// silently drops the rest, which we observed losing inbound during
// burst traffic.
func ParseInboundAll(body []byte) []*ParsedInbound {
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	if payload.Object != "whatsapp_business_account" {
		return nil
	}

	var out []*ParsedInbound
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}
			val := change.Value
			if len(val.Messages) == 0 {
				continue
			}
			for _, msg := range val.Messages {
				if parsed := parseSingleMessage(msg, val); parsed != nil {
					out = append(out, parsed)
				}
			}
		}
	}
	return out
}

// parseSingleMessage normalizes one inbound message into ParsedInbound.
// Returns nil for unrecognized types (so the caller drops them) — system
// events / unknown payloads still emit a non-nil result with Skip=true.
func parseSingleMessage(msg Message, val Value) *ParsedInbound {
	senderName := ""
	for _, c := range val.Contacts {
		if c.WaID == msg.From {
			senderName = c.Profile.Name
			break
		}
	}

	result := &ParsedInbound{
		PhoneNumberID: val.Metadata.PhoneNumberID,
		From:          msg.From,
		SenderName:    senderName,
		MessageID:     msg.ID,
	}

	switch msg.Type {
	case "text":
		result.MessageType = "text"
		if msg.Text != nil {
			result.Text = msg.Text.Body
		}
	case "image":
		result.MessageType = "image"
		if msg.Image != nil {
			result.MediaID = msg.Image.ID
			result.Text = msg.Image.Caption
		}
	case "audio":
		result.MessageType = "audio"
		if msg.Audio != nil {
			result.MediaID = msg.Audio.ID
		}
	case "voice":
		result.MessageType = "voice"
		if msg.Voice != nil {
			result.MediaID = msg.Voice.ID
		}
	case "video":
		result.MessageType = "video"
		if msg.Video != nil {
			result.MediaID = msg.Video.ID
			result.Text = msg.Video.Caption
		}
	case "document":
		result.MessageType = "document"
		if msg.Document != nil {
			result.MediaID = msg.Document.ID
			result.Text = msg.Document.FileName
		}
	case "sticker":
		result.MessageType = "sticker"
		if msg.Sticker != nil {
			result.MediaID = msg.Sticker.ID
		}
	case "location":
		result.MessageType = "location"
		if msg.Location != nil {
			result.LatLon = fmt.Sprintf("%.6f,%.6f", msg.Location.Latitude, msg.Location.Longitude)
			result.Text = result.LatLon
		}
	case "interactive":
		result.MessageType = "text"
		if msg.Interactive != nil {
			switch msg.Interactive.Type {
			case "button_reply":
				if msg.Interactive.ButtonReply != nil {
					// Surface the button id alongside the title so the AI prompt
					// can branch on the structured payload (e.g. "id=confirm_yes")
					// rather than relying on the human-facing label, which the
					// agent can change at any time.
					btnID := msg.Interactive.ButtonReply.ID
					title := msg.Interactive.ButtonReply.Title
					if btnID != "" {
						result.Text = fmt.Sprintf("[botão %s] %s", btnID, title)
					} else {
						result.Text = title
					}
					result.Metadata, _ = json.Marshal(map[string]any{
						"interactive_type": "button_reply",
						"button_id":        btnID,
						"button_title":     title,
					})
				}
			case "list_reply":
				if msg.Interactive.ListReply != nil {
					// Carry the list item ID + description into the prompt
					// so the AI sees the full structured selection, not just
					// the visible title (which Meta lets agents reuse across
					// many list rows).
					listID := msg.Interactive.ListReply.ID
					title := msg.Interactive.ListReply.Title
					desc := msg.Interactive.ListReply.Description
					switch {
					case listID != "" && desc != "":
						result.Text = fmt.Sprintf("[lista %s] %s — %s", listID, title, desc)
					case listID != "":
						result.Text = fmt.Sprintf("[lista %s] %s", listID, title)
					case desc != "":
						result.Text = fmt.Sprintf("%s — %s", title, desc)
					default:
						result.Text = title
					}
					result.Metadata, _ = json.Marshal(map[string]any{
						"interactive_type": "list_reply",
						"list_id":          listID,
						"list_title":       title,
						"description":      desc,
					})
				}
			case "nfm_reply":
				// Flows native-form submission. Body is a short summary;
				// the structured payload (response_json) lives in metadata
				// so agents/automations can read individual fields.
				if msg.Interactive.NFMReply != nil {
					result.Text = msg.Interactive.NFMReply.Body
					if result.Text == "" {
						result.Text = "[formulário enviado]"
					}
					result.Metadata, _ = json.Marshal(map[string]any{
						"interactive_type": "nfm_reply",
						"flow_name":        msg.Interactive.NFMReply.Name,
						"response_json":    msg.Interactive.NFMReply.ResponseJSON,
					})
				}
			}
		}
	case "button":
		result.MessageType = "text"
		if msg.Button != nil {
			result.Text = msg.Button.Text
			result.Metadata, _ = json.Marshal(map[string]any{
				"button_payload": msg.Button.Payload,
			})
		}
	case "reaction":
		// Render in-thread as a tiny system-style line. The agent
		// usually doesn't need to reply to a reaction, but the
		// attendant should see it.
		result.MessageType = "text"
		if msg.Reaction != nil {
			emoji := msg.Reaction.Emoji
			if emoji == "" {
				result.Text = "[reação removida]"
			} else {
				result.Text = "Reagiu com " + emoji
			}
			result.Metadata, _ = json.Marshal(map[string]any{
				"reaction_emoji":      msg.Reaction.Emoji,
				"reaction_message_id": msg.Reaction.MessageID,
			})
		}
	case "contacts":
		result.MessageType = "text"
		// A body that stays empty renders as a blank bubble, which reads as
		// "nothing arrived" rather than "we could not describe what arrived".
		// Meta says this is a contact card; say so even when the payload is
		// shaped in a way we cannot summarise.
		result.Text = "📇 Contato compartilhado"
		if len(msg.Contacts) > 0 {
			names := make([]string, 0, len(msg.Contacts))
			// Copied, not mutated in place: the raw card is dropped from the
			// stored blob (it can embed a base64 PHOTO) while what we read out
			// of it is kept.
			cards := make([]ContactCard, 0, len(msg.Contacts))
			for _, c := range msg.Contacts {
				n := c.Name.FormattedName
				if n == "" {
					n = strings.TrimSpace(c.Name.FirstName + " " + c.Name.LastName)
				}

				card := decodeVCard(c.VCard)
				// The business's own name. Shown only when it differs from the
				// name above — which came from the sharer's phonebook — because
				// on most cards the two are identical and printing both would
				// just double every line. When they DO differ, the phonebook
				// label is a nickname and this is who the number belongs to.
				bizName := vcardProperty(card, waBizName)
				if bizName != "" && !sameDisplayName(bizName, n) {
					n = n + " · " + bizName
				}

				if len(c.Phones) > 0 && c.Phones[0].Phone != "" {
					n = n + " (" + c.Phones[0].Phone + ")"
				}
				// How a business describes itself. For a shared number this is
				// often the only thing that says what it IS — a name and a
				// phone alone leave the attendant guessing.
				desc := vcardProperty(card, waBizDescription)
				if desc != "" {
					n = n + " — " + desc
				}
				names = append(names, n)

				c.BusinessDescription = desc
				c.BusinessName = bizName
				c.VCard = ""
				cards = append(cards, c)
			}
			result.Text = "📇 Contato compartilhado: " + strings.Join(names, ", ")
			result.Metadata, _ = json.Marshal(map[string]any{
				"contacts": cards,
			})
		}
	case "order":
		result.MessageType = "text"
		if msg.Order != nil {
			summary := "🛒 Pedido recebido"
			if len(msg.Order.ProductItems) > 0 {
				summary = fmt.Sprintf("🛒 Pedido com %d item(s)", len(msg.Order.ProductItems))
			}
			if msg.Order.Text != "" {
				summary = summary + ": " + msg.Order.Text
			}
			result.Text = summary
			result.Metadata, _ = json.Marshal(map[string]any{
				"order": msg.Order,
			})
		}
	case "system":
		// Service event from WhatsApp itself — must be tracked but
		// never replied to. Skip prevents AI/billing fanout.
		result.MessageType = "system"
		result.Skip = true
		if msg.System != nil {
			if msg.System.Body != "" {
				result.Text = msg.System.Body
			} else {
				result.Text = "Evento do sistema (" + msg.System.Type + ")"
			}
			result.Metadata, _ = json.Marshal(map[string]any{
				"system": msg.System,
			})
		}
	case "request_welcome":
		// Customer tapped a "Send a Message" CTA on a Catalog or
		// ad — Meta sends an empty welcome event. Treat as a
		// blank inbound text so the agent can fire its greeting.
		result.MessageType = "text"
		result.Text = ""
	case "unsupported", "unknown":
		// Meta cannot deliver some message types at all (poll, live
		// location, view-once media). It still tells us one arrived, as
		// `type: "unsupported"` carrying an `errors` envelope whose title
		// is "Message type unknown" (code 131051).
		//
		// That title is why this branch read `case "unknown"` until
		// 2026-08-18: the string was lifted from the error title instead of
		// the `type` field, so it never matched and every such message fell
		// through to `default:` — which returns nil and DROPS it. The
		// customer saw their message sent; the attendant saw nothing at all,
		// with no row to notice later. "unknown" stays as a second label
		// because Meta has renamed these strings before and a stale alias
		// costs nothing.
		//
		// Skip stays on: the AI cannot answer content Meta refused to hand
		// over, and a confident reply to an invisible message is worse than
		// silence. A human sees the placeholder and can ask.
		result.MessageType = "text"
		result.Skip = true
		result.Text = "⚠️ O cliente enviou uma mensagem que o WhatsApp não entrega para atendimentos (ex.: enquete, localização ao vivo, visualização única). Peça para ele reenviar de outra forma."
		meta := map[string]any{"unsupported_type": msg.Type}
		if len(msg.Errors) > 0 {
			meta["errors"] = msg.Errors
		}
		result.Metadata, _ = json.Marshal(meta)
	default:
		slog.Warn("WhatsApp inbound type not recognized",
			"type", msg.Type,
			"phone_number_id", val.Metadata.PhoneNumberID,
			"wamid", msg.ID)
		return nil
	}

	// Merge referral + reply-context into the existing metadata blob.
	// Both are orthogonal to message type — a referral can land on a
	// text inbound just as easily as on an image — so we layer them on
	// last instead of inlining them in every case branch.
	if msg.Referral != nil || msg.Context != nil {
		extra := map[string]any{}
		if len(result.Metadata) > 0 {
			_ = json.Unmarshal(result.Metadata, &extra)
		}
		if msg.Referral != nil {
			extra["referral"] = map[string]any{
				"source_url":    msg.Referral.SourceURL,
				"source_type":   msg.Referral.SourceType,
				"source_id":     msg.Referral.SourceID,
				"headline":      msg.Referral.Headline,
				"body":          msg.Referral.Body,
				"media_type":    msg.Referral.MediaType,
				"image_url":     msg.Referral.ImageURL,
				"video_url":     msg.Referral.VideoURL,
				"thumbnail_url": msg.Referral.ThumbnailURL,
				"ctwa_clid":     msg.Referral.CTWACLID,
			}
		}
		if msg.Context != nil && msg.Context.ID != "" {
			extra["reply_to_wamid"] = msg.Context.ID
		}
		result.Metadata, _ = json.Marshal(extra)
	}

	return result
}

// GetMediaURL resolves a Meta media object ID to a downloadable HTTPS URL.
// It calls the Graph API to get the URL, then returns it directly (the URL is short-lived).
func (a *WhatsAppAdapter) GetMediaURL(ctx context.Context, channel *domain.Channel, mediaID string) (string, error) {
	cfg, err := parseConfig(channel)
	if err != nil {
		return "", err
	}

	accessToken, err := a.resolveAccessToken(cfg)
	if err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf("%s/%s/%s", metaGraphBaseURL, metaAPIVersion, mediaID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("whatsapp: failed to create media request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("whatsapp: media request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		URL      string `json:"url"`
		MimeType string `json:"mime_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("whatsapp: failed to decode media response: %w", err)
	}
	if result.URL == "" {
		return "", fmt.Errorf("whatsapp: empty media URL returned for media ID %s", mediaID)
	}
	return result.URL, nil
}

// metaMediaHostAllowlist is the set of host suffixes Cloud API may
// return as the resolved media download URL. Meta documents the URL as
// hosted on its own CDN; pinning to known hosts prevents an attacker
// who can influence the response from /{media_id} (compromised proxy,
// rogue Graph endpoint substitute, BSP MITM) from steering us to send
// a Bearer access token at an arbitrary host (SSRF + token leak).
//
// Each entry must match either the full host or a suffix preceded by
// a dot (so "fbcdn.net" matches "scontent-xyz.fbcdn.net" but NOT
// "evil-fbcdn.net").
var metaMediaHostAllowlist = []string{
	"fbcdn.net",
	"lookaside.fbsbx.com",
	"fbsbx.com",
	"cdninstagram.com",
}

// isAllowedMediaHost reports whether host is one of Meta's documented
// CDN hosts. Empty hosts and non-allowlisted hosts return false.
func isAllowedMediaHost(host string) bool {
	host = strings.ToLower(host)
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	for _, suffix := range metaMediaHostAllowlist {
		if host == suffix {
			return true
		}
		if strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

// DownloadMedia fetches the raw media bytes for a resolved CDN URL. Meta
// CDN URLs are short-lived and require the access token in the Authorization
// header — a plain unauthenticated GET returns 401. Used by the voice-note
// transcription path before we pipe the audio into Whisper.
func (a *WhatsAppAdapter) DownloadMedia(ctx context.Context, channel *domain.Channel, mediaURL string) ([]byte, string, error) {
	cfg, err := parseConfig(channel)
	if err != nil {
		return nil, "", err
	}
	accessToken, err := a.resolveAccessToken(cfg)
	if err != nil {
		return nil, "", err
	}
	parsedURL, err := url.Parse(mediaURL)
	if err != nil {
		return nil, "", fmt.Errorf("whatsapp: invalid media URL: %w", err)
	}
	if parsedURL.Scheme != "https" {
		return nil, "", fmt.Errorf("whatsapp: refusing to send Bearer to non-HTTPS media URL")
	}
	if !isAllowedMediaHost(parsedURL.Host) {
		return nil, "", fmt.Errorf("whatsapp: refusing to send Bearer to disallowed host %q", parsedURL.Host)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("whatsapp: failed to build media download request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("whatsapp: media download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("whatsapp: media download returned %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return data, resp.Header.Get("Content-Type"), nil
}

// RegisterAppWebhook subscribes the app to whatsapp_business_account webhook events
// via the Graph API subscriptions endpoint. This is a one-time setup per App ID.
// webhookURL must be a public HTTPS URL.
func RegisterAppWebhook(ctx context.Context, appID, appSecret, verifyToken, webhookURL string) error {
	apiURL := fmt.Sprintf("%s/%s/%s/subscriptions", metaGraphBaseURL, metaAPIVersion, appID)

	// App-level access token: app_id|app_secret
	appToken := appID + "|" + appSecret

	form := url.Values{}
	form.Set("object", "whatsapp_business_account")
	form.Set("callback_url", webhookURL)
	form.Set("verify_token", verifyToken)
	form.Set("fields", "messages,message_deliveries,message_reads")
	form.Set("access_token", appToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL,
		bytes.NewBufferString(form.Encode()))
	if err != nil {
		return fmt.Errorf("whatsapp: failed to create subscriptions request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: subscriptions request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success bool `json:"success"`
		Error   *struct {
			Message   string `json:"message"`
			Type      string `json:"type"`
			Code      int    `json:"code"`
			Subcode   int    `json:"error_subcode"`
			UserMsg   string `json:"error_user_msg"`
			FBTraceID string `json:"fbtrace_id"`
		} `json:"error,omitempty"`
	}
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &result)

	if !result.Success {
		// Surface the full Graph API envelope so ops can tell "already
		// subscribed with a different callback URL" apart from "wrong
		// app_secret" apart from a transient Meta 5xx.
		if result.Error != nil {
			return fmt.Errorf(
				"whatsapp: app webhook subscription failed (http=%d, code=%d/%d, type=%s, fbtrace=%s): %s",
				resp.StatusCode,
				result.Error.Code,
				result.Error.Subcode,
				result.Error.Type,
				result.Error.FBTraceID,
				result.Error.Message,
			)
		}
		return fmt.Errorf("whatsapp: app webhook subscription failed (http=%d): %s", resp.StatusCode, string(body))
	}

	slog.Info("WhatsApp app webhook registered", "webhook_url", webhookURL)
	return nil
}

// MaxWebhookAge bounds how stale a Meta webhook payload may be before
// the receiver treats it as a replay attempt. Meta retries internally
// for up to ~24h on a 5xx response, but legitimate first-delivery and
// short-window retries arrive within a couple of minutes. 1h leaves
// a comfortable margin while shutting down the long tail an attacker
// could capture once and replay forever.
const MaxWebhookAge = 1 * 60 * 60 // seconds

// EntryTimestampUnix returns the oldest entry.time in the webhook
// payload (unix seconds), or 0 when no parseable entry is present.
// Used by the receiver to reject replays without parsing the full
// envelope twice.
func EntryTimestampUnix(body []byte) int64 {
	var p struct {
		Entry []struct {
			Time int64 `json:"time"`
		} `json:"entry"`
	}
	if err := jsonUnmarshal(body, &p); err != nil {
		return 0
	}
	var oldest int64
	for _, e := range p.Entry {
		if e.Time == 0 {
			continue
		}
		if oldest == 0 || e.Time < oldest {
			oldest = e.Time
		}
	}
	return oldest
}

// jsonUnmarshal indirection so the test suite can swap behaviour
// without touching encoding/json directly.
var jsonUnmarshal = jsonUnmarshalDefault

func jsonUnmarshalDefault(b []byte, v any) error { return json.Unmarshal(b, v) }

// VerifySignature ensures the webhook payload actually came from Meta
// using the APP_SECRET. Per Meta's docs the header is "sha256=<64 hex
// chars>" — we enforce the exact length (sha256-hex is always 64
// bytes) so a degenerate "sha256=" or oversized header can't pass the
// prefix check and reach hmac.Equal with a length-mismatch shortcut.
// Prefix matching is case-insensitive as a defense against future
// proxies that uppercase header values; Meta itself sends lowercase.
func (a *WhatsAppAdapter) VerifySignature(payload []byte, signatureHeader string, appSecret string) bool {
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

// ParseStatuses extracts delivery status updates from a WhatsApp webhook payload.
// Returns nil if the payload contains no status updates.
func ParseStatuses(body []byte) []ParsedStatus {
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}

	var statuses []ParsedStatus
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}
			for _, s := range change.Value.Statuses {
				ps := ParsedStatus{
					PhoneNumberID: change.Value.Metadata.PhoneNumberID,
					MessageID:     s.ID,
					Status:        s.Status,
					Recipient:     s.Recipient,
					Timestamp:     s.Timestamp,
				}
				if s.Conversation != nil {
					ps.ConversationID = s.Conversation.ID
					ps.ConversationOrigin = s.Conversation.Origin.Type
					ps.ConversationExpires = s.Conversation.ExpirationTimestamp
				}
				if s.Pricing != nil {
					ps.PricingCategory = s.Pricing.Category
					ps.PricingBillable = s.Pricing.Billable
				}
				if s.Status == "failed" && len(s.Errors) > 0 {
					e := s.Errors[0]
					if e.Message != "" {
						ps.FailedReason = e.Message
					} else if e.Title != "" {
						ps.FailedReason = e.Title
					}
				}
				statuses = append(statuses, ps)
			}
		}
	}
	return statuses
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
