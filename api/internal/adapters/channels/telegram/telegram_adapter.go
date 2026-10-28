package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/replider/api/internal/adapters/httpsafe"
	mediautil "github.com/replider/api/internal/adapters/media"
	"github.com/replider/api/internal/core/domain"
)

const telegramAPIBase = "https://api.telegram.org/bot%s"

// telegramConfig is the expected JSON inside Channel.Config for Telegram channels.
type telegramConfig struct {
	BotToken string `json:"bot_token"`
}

// TelegramAdapter implements domain.MessageProvider for the Telegram Bot API.
type TelegramAdapter struct {
	httpClient *http.Client
	// mediaClient fetches media at a URL the CUSTOMER supplied (an
	// attendant's reply). Built by httpsafe so the dial refuses internal
	// addresses; httpClient is reserved for Telegram's own API.
	mediaClient *http.Client
	signer      *URLSigner // optional; when nil, falls back to raw telegram URLs (bot token leaks — tests only)
}

func NewTelegramAdapter(signer *URLSigner) *TelegramAdapter {
	return &TelegramAdapter{
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		mediaClient: httpsafe.Client(60 * time.Second),
		signer:      signer,
	}
}

func parseConfig(channel *domain.Channel) (*telegramConfig, error) {
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

	var cfg telegramConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("channel %s: failed to parse config: %w", channel.ID, err)
	}
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("channel %s: missing bot_token in config", channel.ID)
	}
	return &cfg, nil
}

// SendMessage sends a message to a Telegram chat.
// Supports all media types: text, image, audio, voice, video, document, location.
// `to` is the chat_id (string form of int64).
func (a *TelegramAdapter) SendMessage(ctx context.Context, channel *domain.Channel, to string, msg *domain.Message) error {
	cfg, err := parseConfig(channel)
	if err != nil {
		return err
	}

	msgType := msg.Type
	if msgType == "" {
		msgType = "text"
	}

	// Audio/voice: Telegram renders URL-based sendAudio calls as a
	// plain document bubble when the remote file is an m4a behind a
	// signed GCS URL — the URL's query string breaks its internal
	// extension sniff. Multipart upload with an explicit filename
	// ("voice.ogg" or "voice.m4a") is the only reliable way to force
	// the audio-bubble renderer on the receiver side.
	if msgType == "voice" || msgType == "audio" {
		return a.sendAudioMultipart(ctx, cfg, to, msgType, msg)
	}

	var endpoint string
	var payload map[string]interface{}

	switch msgType {
	case "image":
		endpoint = "sendPhoto"
		payload = map[string]interface{}{
			"chat_id": to,
			"photo":   deref(msg.MediaURL),
		}
		if deref(msg.Content) != "" {
			payload["caption"] = deref(msg.Content)
		}

	case "video":
		endpoint = "sendVideo"
		payload = map[string]interface{}{
			"chat_id": to,
			"video":   deref(msg.MediaURL),
		}
		if deref(msg.Content) != "" {
			payload["caption"] = deref(msg.Content)
		}

	case "document":
		endpoint = "sendDocument"
		payload = map[string]interface{}{
			"chat_id":  to,
			"document": deref(msg.MediaURL),
		}
		if deref(msg.Content) != "" {
			payload["caption"] = deref(msg.Content)
		}

	case "location":
		endpoint = "sendLocation"
		parts := strings.SplitN(deref(msg.Content), ",", 2)
		if len(parts) != 2 {
			return fmt.Errorf("telegram: invalid location format %q, expected 'lat,lon'", deref(msg.Content))
		}
		lat, errLat := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		lon, errLon := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if errLat != nil || errLon != nil {
			return fmt.Errorf("telegram: failed to parse location coordinates from %q", deref(msg.Content))
		}
		payload = map[string]interface{}{
			"chat_id":   to,
			"latitude":  lat,
			"longitude": lon,
		}

	default: // "text" and any unknown type
		endpoint = "sendMessage"
		payload = map[string]interface{}{
			"chat_id": to,
			"text":    deref(msg.Content),
		}
	}

	apiURL := fmt.Sprintf(telegramAPIBase+"/%s", cfg.BotToken, endpoint)
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram: API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	slog.Info("message sent via Telegram", "type", msgType, "channel_id", channel.ID)
	return nil
}

// HandleWebhook parses a Telegram Update and returns the normalized sender + text.
// The actual inbound flow is handled by InboundWebhookHandler via ParseUpdate.
func (a *TelegramAdapter) HandleWebhook(ctx context.Context, channel *domain.Channel, payload []byte) error {
	var update Update
	if err := json.Unmarshal(payload, &update); err != nil {
		return fmt.Errorf("telegram: failed to parse update: %w", err)
	}
	slog.Info("Telegram update received", "update_id", update.UpdateID)
	return nil
}

// ParsedUpdate holds all normalized data extracted from a Telegram Update payload.
type ParsedUpdate struct {
	ChatID      string // string form of the Chat.ID
	MessageID   string // Telegram message_id scoped to the chat; used as external_message_id for dedupe
	UserID      int64  // raw Telegram user id — needed for getUserProfilePhotos
	Text        string // message text, or caption for media messages
	SenderName  string // first + last name of the sender
	Username    string // @handle (without @), may be empty
	MediaFileID string // Telegram file_id to resolve via getFile API; empty for text/location
	MediaType   string // "image", "audio", "voice", "video", "document", "sticker", "location"
	LatLon      string // "lat,lon" — populated only for location messages
	Date        int64  // message.date (unix seconds) — used as a replay-attack guard
}

// ParseUpdate extracts all relevant fields from a raw Telegram Update payload.
// Returns (nil, false) for updates that carry no actionable content (e.g. bot added to group).
func ParseUpdate(payload []byte) (*ParsedUpdate, bool) {
	var update Update
	if err := json.Unmarshal(payload, &update); err != nil {
		return nil, false
	}
	msg := update.Message
	if msg == nil {
		return nil, false
	}

	// Determine sender info
	name := ""
	uname := ""
	var userID int64
	if msg.From != nil {
		name = msg.From.FirstName
		if msg.From.LastName != "" {
			name += " " + msg.From.LastName
		}
		uname = msg.From.Username
		userID = msg.From.ID
	}

	chatID := fmt.Sprintf("%d", msg.Chat.ID)
	// Scope message_id to the chat so the partial unique index on
	// (channel_id, external_message_id) cannot collide across chats that
	// happen to reuse small integer ids.
	externalID := fmt.Sprintf("%s:%d", chatID, msg.MessageID)
	result := &ParsedUpdate{
		ChatID:     chatID,
		MessageID:  externalID,
		UserID:     userID,
		SenderName: name,
		Username:   uname,
		Date:       msg.Date,
	}

	switch {
	case msg.Text != "":
		result.Text = msg.Text
		result.MediaType = "text"

	case len(msg.Photo) > 0:
		// Telegram sends multiple resolutions; use the last (largest) one
		result.MediaFileID = msg.Photo[len(msg.Photo)-1].FileID
		result.MediaType = "image"
		result.Text = msg.Caption

	case msg.Audio != nil:
		result.MediaFileID = msg.Audio.FileID
		result.MediaType = "audio"
		result.Text = msg.Caption

	case msg.Voice != nil:
		result.MediaFileID = msg.Voice.FileID
		result.MediaType = "voice"

	case msg.Video != nil:
		result.MediaFileID = msg.Video.FileID
		result.MediaType = "video"
		result.Text = msg.Caption

	case msg.Document != nil:
		result.MediaFileID = msg.Document.FileID
		result.MediaType = "document"
		result.Text = msg.Caption
		if msg.Document.FileName != "" && result.Text == "" {
			result.Text = msg.Document.FileName
		}

	case msg.Sticker != nil:
		result.MediaFileID = msg.Sticker.FileID
		result.MediaType = "sticker"

	case msg.Location != nil:
		result.MediaType = "location"
		result.LatLon = fmt.Sprintf("%.6f,%.6f", msg.Location.Latitude, msg.Location.Longitude)
		result.Text = result.LatLon

	default:
		// Unknown update type (e.g. poll, dice) — acknowledge silently
		return nil, false
	}

	return result, true
}

// GetFileURL resolves a Telegram file_id into a downloadable HTTPS URL.
// Returns a signed API-proxy URL when a signer is wired so the Telegram
// bot token never reaches the browser. Falls back to the raw Telegram
// CDN URL only in test builds where no signer is configured.
func (a *TelegramAdapter) GetFileURL(ctx context.Context, channel *domain.Channel, fileID string) (string, error) {
	filePath, err := a.ResolveFilePath(ctx, channel, fileID)
	if err != nil {
		return "", err
	}
	if a.signer != nil {
		return a.signer.SignFileURL(channel.ID, filePath, 7*24*time.Hour), nil
	}
	// Legacy/test-only: exposes bot token; never hits prod because main.go
	// always injects a signer.
	cfg, err := parseConfig(channel)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", cfg.BotToken, filePath), nil
}

// ResolveFilePath calls the getFile Bot API endpoint and returns just
// the relative file_path (e.g., "profile_photos/file_1.jpg"). Kept
// separate from GetFileURL so the proxy handler can hand a path to
// FetchFile without re-resolving.
func (a *TelegramAdapter) ResolveFilePath(ctx context.Context, channel *domain.Channel, fileID string) (string, error) {
	cfg, err := parseConfig(channel)
	if err != nil {
		return "", err
	}
	apiURL := fmt.Sprintf(telegramAPIBase+"/getFile?file_id=%s", cfg.BotToken, fileID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("telegram: failed to create getFile request: %w", err)
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("telegram: getFile request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("telegram: failed to decode getFile response: %w", err)
	}
	if !result.OK {
		return "", fmt.Errorf("telegram: getFile failed: %s", result.Description)
	}
	return result.Result.FilePath, nil
}

// FetchFile streams a Telegram file by its file_path, using the
// channel's bot token. Returns the raw HTTP response so the proxy
// handler can forward headers + body without re-buffering.
func (a *TelegramAdapter) FetchFile(ctx context.Context, channel *domain.Channel, filePath string) (*http.Response, error) {
	cfg, err := parseConfig(channel)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", cfg.BotToken, filePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	return a.httpClient.Do(req)
}

// VerifyFileURLSignature exposes the adapter's signer to HTTP handlers
// that need to validate proxy URLs without importing the telegram
// package's internals.
func (a *TelegramAdapter) VerifyFileURLSignature(channelID, filePath, expStr, sig string) bool {
	if a.signer == nil {
		return false
	}
	return a.signer.Verify(channelID, filePath, expStr, sig)
}

// ChatInfo bundles the user-facing profile fields the Bot API exposes
// for a private chat. Both fields are best-effort: photo privacy or a
// missing bio just land as empty strings, never errors.
type ChatInfo struct {
	PhotoURL string
	Bio      string
}

// GetChatInfo calls getChat for a private chat and resolves the photo
// file_id to a downloadable URL. getChat is the richest endpoint for
// bot-visible profile data (bio is only here — getUserProfilePhotos
// exposes photos only). Returns zero values with nil error when the
// user has no photo or bio.
func (a *TelegramAdapter) GetChatInfo(ctx context.Context, channel *domain.Channel, chatID string) (*ChatInfo, error) {
	if chatID == "" {
		return &ChatInfo{}, nil
	}
	cfg, err := parseConfig(channel)
	if err != nil {
		return nil, err
	}

	apiURL := fmt.Sprintf(telegramAPIBase+"/getChat?chat_id=%s", cfg.BotToken, chatID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("telegram: failed to create getChat request: %w", err)
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram: getChat request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			Bio   string `json:"bio"`
			Photo *struct {
				BigFileID string `json:"big_file_id"`
			} `json:"photo"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("telegram: failed to decode getChat response: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("telegram: getChat failed: %s", result.Description)
	}

	info := &ChatInfo{Bio: result.Result.Bio}
	if result.Result.Photo != nil && result.Result.Photo.BigFileID != "" {
		if url, err := a.GetFileURL(ctx, channel, result.Result.Photo.BigFileID); err == nil {
			info.PhotoURL = url
		}
	}
	return info, nil
}

// GetUserProfilePhotoURL fetches the user's current profile picture via
// getUserProfilePhotos + getFile. Returns "" (no error) when the user has
// no photos or has photos privacy set so the bot cannot see them — both
// are normal states and should not surface as webhook failures. Callers
// should store the returned URL on the Conversation row so the dashboard
// can render the avatar without re-fetching on every view.
func (a *TelegramAdapter) GetUserProfilePhotoURL(ctx context.Context, channel *domain.Channel, userID int64) (string, error) {
	if userID == 0 {
		return "", nil
	}
	cfg, err := parseConfig(channel)
	if err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf(telegramAPIBase+"/getUserProfilePhotos?user_id=%d&limit=1", cfg.BotToken, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("telegram: failed to create getUserProfilePhotos request: %w", err)
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("telegram: getUserProfilePhotos request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			TotalCount int           `json:"total_count"`
			Photos     [][]PhotoSize `json:"photos"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("telegram: failed to decode getUserProfilePhotos response: %w", err)
	}
	if !result.OK {
		return "", fmt.Errorf("telegram: getUserProfilePhotos failed: %s", result.Description)
	}
	if result.Result.TotalCount == 0 || len(result.Result.Photos) == 0 || len(result.Result.Photos[0]) == 0 {
		return "", nil
	}
	// Photos come as an array of resolutions, smallest to largest.
	// Largest is the last entry — good enough for a dashboard avatar.
	sizes := result.Result.Photos[0]
	best := sizes[len(sizes)-1]
	return a.GetFileURL(ctx, channel, best.FileID)
}

// WebhookInfo holds the status returned by Telegram's getWebhookInfo endpoint.
type WebhookInfo struct {
	URL                  string `json:"url"`
	HasCustomCertificate bool   `json:"has_custom_certificate"`
	PendingUpdateCount   int    `json:"pending_update_count"`
	LastErrorDate        int64  `json:"last_error_date,omitempty"`
	LastErrorMessage     string `json:"last_error_message,omitempty"`
	MaxConnections       int    `json:"max_connections,omitempty"`
}

// GetWebhookInfo queries Telegram's getWebhookInfo API and returns the current webhook status.
func (a *TelegramAdapter) GetWebhookInfo(ctx context.Context, channel *domain.Channel) (*WebhookInfo, error) {
	cfg, err := parseConfig(channel)
	if err != nil {
		return nil, err
	}

	apiURL := fmt.Sprintf(telegramAPIBase+"/getWebhookInfo", cfg.BotToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("telegram: failed to create getWebhookInfo request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram: getWebhookInfo request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK          bool        `json:"ok"`
		Result      WebhookInfo `json:"result"`
		Description string      `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("telegram: failed to decode getWebhookInfo response: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("telegram: getWebhookInfo failed: %s", result.Description)
	}
	return &result.Result, nil
}

// RegisterWebhook calls Telegram's setWebhook API to register the public webhook URL.
// webhookURL should be the full public HTTPS URL, e.g. https://abc.ngrok.io/v1/webhooks/<token>
//
// Clears any stale webhook first: Telegram preserves last_error_message
// across setWebhook calls, so a prior 500 keeps flagging the channel
// "unhealthy" long after we fixed the underlying bug. deleteWebhook
// resets that state — the subsequent setWebhook starts clean.
func RegisterWebhook(ctx context.Context, botToken string, webhookURL string, secretToken string) error {
	// Best-effort delete: if it fails we still try to register so the
	// endpoint isn't a no-op for first-time setups.
	if err := DeleteWebhook(ctx, botToken); err != nil {
		slog.Debug("telegram: deleteWebhook before register failed (non-fatal)", "error", err)
	}

	url := fmt.Sprintf(telegramAPIBase+"/setWebhook", botToken)
	payload := map[string]any{
		"url":                  webhookURL,
		"secret_token":         secretToken,
		"drop_pending_updates": true,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: failed to create setWebhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("telegram: setWebhook request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	json.Unmarshal(respBody, &result)

	if !result.OK {
		return fmt.Errorf("telegram: setWebhook failed: %s", result.Description)
	}
	slog.Info("Telegram webhook registered", "webhook_url", webhookURL)
	return nil
}

// DeleteWebhook removes the webhook from Telegram (useful when deleting a channel).
func DeleteWebhook(ctx context.Context, botToken string) error {
	url := fmt.Sprintf(telegramAPIBase+"/deleteWebhook", botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	slog.Info("Telegram webhook deleted")
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// sendAudioMultipart downloads the audio from its (signed) source URL
// and re-uploads it to Telegram as a multipart/form-data request with
// an explicit filename. URL-based sendAudio was silently degrading to
// a document bubble on the receiver — Telegram uses the URL's path
// extension to pick the renderer, and GCS signed URLs append a large
// query string that throws off its sniff. Multipart with
// `voice.ogg`/`voice.m4a` forces the right renderer.
//
// Picks the endpoint + form field by extension:
//   - ogg/oga/opus  -> sendVoice / voice (renders as Telegram voice note)
//   - everything    -> sendAudio / audio (renders as music player)
func (a *TelegramAdapter) sendAudioMultipart(
	ctx context.Context,
	cfg *telegramConfig,
	chatID string,
	msgType string,
	msg *domain.Message,
) error {
	rawURL := deref(msg.MediaURL)
	if rawURL == "" {
		return fmt.Errorf("telegram: audio message has no media URL")
	}

	ext := mediaExt(rawURL)

	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("telegram: build media request: %w", err)
	}
	getResp, err := a.mediaClient.Do(getReq)
	if err != nil {
		return fmt.Errorf("telegram: fetch media: %w", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode >= 400 {
		return fmt.Errorf("telegram: fetch media: status %d", getResp.StatusCode)
	}
	// Telegram caps outbound media at 50MB; bound the read at one byte
	// past that so the worker can't be OOM'd by a customer who uploads a
	// 100MB blob via /media/upload and triggers a send. Anything past the
	// cap is rejected loudly so the dashboard surfaces the limit.
	const telegramAudioMaxBytes = 50 * 1024 * 1024
	audioBytes, err := io.ReadAll(io.LimitReader(getResp.Body, telegramAudioMaxBytes+1))
	if err != nil {
		return fmt.Errorf("telegram: read media: %w", err)
	}
	if int64(len(audioBytes)) > telegramAudioMaxBytes {
		return fmt.Errorf("telegram: media exceeds %d byte cap", telegramAudioMaxBytes)
	}

	var endpoint, field, filename, contentType string
	switch {
	case msgType == "voice" && (ext == "ogg" || ext == "oga" || ext == "opus"):
		// Already in the format Telegram's voice-note renderer expects.
		endpoint, field = "sendVoice", "voice"
		filename, contentType = "voice.ogg", "audio/ogg"
	case msgType == "voice":
		// Browser MediaRecorder gives us AAC/m4a; Telegram renders that
		// through sendAudio as a music-player bubble, not a voice note.
		// Transcode to OGG-Opus so the receiver sees the native
		// voice-note bubble with waveform + duration, matching what a
		// Telegram client would send.
		transcoded, err := mediautil.TranscodeToOggOpus(ctx, audioBytes)
		if err != nil {
			slog.Warn("telegram: voice transcode to ogg-opus failed, falling back to sendAudio",
				"error", err, "bytes", len(audioBytes))
			endpoint, field = "sendAudio", "audio"
			filename, contentType = "audio.m4a", "audio/mp4"
		} else {
			audioBytes = transcoded
			endpoint, field = "sendVoice", "voice"
			filename, contentType = "voice.ogg", "audio/ogg"
		}
	case ext == "mp3":
		endpoint, field = "sendAudio", "audio"
		filename, contentType = "audio.mp3", "audio/mpeg"
	case ext == "wav":
		endpoint, field = "sendAudio", "audio"
		filename, contentType = "audio.wav", "audio/wav"
	default:
		// audio message of any unknown container — Telegram prefers m4a.
		endpoint, field = "sendAudio", "audio"
		filename, contentType = "audio.m4a", "audio/mp4"
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("chat_id", chatID); err != nil {
		return fmt.Errorf("telegram: write chat_id: %w", err)
	}
	if caption := deref(msg.Content); caption != "" && msgType == "audio" {
		if err := mw.WriteField("caption", caption); err != nil {
			return fmt.Errorf("telegram: write caption: %w", err)
		}
	}
	h := make(map[string][]string)
	h["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name=%q; filename=%q`, field, filename)}
	h["Content-Type"] = []string{contentType}
	part, err := mw.CreatePart(h)
	if err != nil {
		return fmt.Errorf("telegram: create multipart part: %w", err)
	}
	if _, err := part.Write(audioBytes); err != nil {
		return fmt.Errorf("telegram: write audio bytes: %w", err)
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("telegram: close multipart: %w", err)
	}

	apiURL := fmt.Sprintf(telegramAPIBase+"/%s", cfg.BotToken, endpoint)
	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, &body)
	if err != nil {
		return fmt.Errorf("telegram: build upload request: %w", err)
	}
	postReq.Header.Set("Content-Type", mw.FormDataContentType())

	postResp, err := a.httpClient.Do(postReq)
	if err != nil {
		return fmt.Errorf("telegram: upload: %w", err)
	}
	defer postResp.Body.Close()
	if postResp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(postResp.Body)
		return fmt.Errorf("telegram: upload returned %d: %s", postResp.StatusCode, string(respBody))
	}

	slog.Info("audio uploaded via Telegram multipart",
		"endpoint", endpoint, "filename", filename, "bytes", len(audioBytes))
	return nil
}

// mediaExt extracts the lowercase file extension from a media URL,
// stripping any query string (GCS signed URLs append `?X-Goog-...`).
// Returns "" when the URL has no extension or fails to parse.
func mediaExt(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(u.Path), "."))
	return ext
}
