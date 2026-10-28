package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
)

// --- ParseInbound Tests ---

func TestParseInbound_TextMessage(t *testing.T) {
	payload := buildWebhookPayload("text", map[string]interface{}{
		"text": map[string]string{"body": "Hello there"},
	})

	result, ok := ParseInbound(payload)
	if !ok {
		t.Fatal("expected ParseInbound to return true for text message")
	}
	if result.From != "5511999999999" {
		t.Errorf("expected From '5511999999999', got %q", result.From)
	}
	if result.Text != "Hello there" {
		t.Errorf("expected Text 'Hello there', got %q", result.Text)
	}
	if result.MessageType != "text" {
		t.Errorf("expected MessageType 'text', got %q", result.MessageType)
	}
	if result.PhoneNumberID != "123456789" {
		t.Errorf("expected PhoneNumberID '123456789', got %q", result.PhoneNumberID)
	}
	if result.SenderName != "John Doe" {
		t.Errorf("expected SenderName 'John Doe', got %q", result.SenderName)
	}
}

func TestParseInbound_ImageMessage(t *testing.T) {
	payload := buildWebhookPayload("image", map[string]interface{}{
		"image": map[string]string{
			"id":        "media_img_001",
			"mime_type": "image/jpeg",
			"caption":   "Check this out",
		},
	})

	result, ok := ParseInbound(payload)
	if !ok {
		t.Fatal("expected ParseInbound to return true for image message")
	}
	if result.MessageType != "image" {
		t.Errorf("expected MessageType 'image', got %q", result.MessageType)
	}
	if result.MediaID != "media_img_001" {
		t.Errorf("expected MediaID 'media_img_001', got %q", result.MediaID)
	}
	if result.Text != "Check this out" {
		t.Errorf("expected Text (caption) 'Check this out', got %q", result.Text)
	}
}

func TestParseInbound_AudioMessage(t *testing.T) {
	payload := buildWebhookPayload("audio", map[string]interface{}{
		"audio": map[string]string{
			"id":        "media_audio_001",
			"mime_type": "audio/ogg",
		},
	})

	result, ok := ParseInbound(payload)
	if !ok {
		t.Fatal("expected ParseInbound to return true for audio message")
	}
	if result.MessageType != "audio" {
		t.Errorf("expected MessageType 'audio', got %q", result.MessageType)
	}
	if result.MediaID != "media_audio_001" {
		t.Errorf("expected MediaID 'media_audio_001', got %q", result.MediaID)
	}
}

func TestParseInbound_VoiceMessage(t *testing.T) {
	payload := buildWebhookPayload("voice", map[string]interface{}{
		"voice": map[string]string{
			"id":        "media_voice_001",
			"mime_type": "audio/ogg; codecs=opus",
		},
	})

	result, ok := ParseInbound(payload)
	if !ok {
		t.Fatal("expected ParseInbound to return true for voice message")
	}
	if result.MessageType != "voice" {
		t.Errorf("expected MessageType 'voice', got %q", result.MessageType)
	}
	if result.MediaID != "media_voice_001" {
		t.Errorf("expected MediaID 'media_voice_001', got %q", result.MediaID)
	}
}

func TestParseInbound_VideoMessage(t *testing.T) {
	payload := buildWebhookPayload("video", map[string]interface{}{
		"video": map[string]string{
			"id":        "media_video_001",
			"mime_type": "video/mp4",
			"caption":   "Watch this",
		},
	})

	result, ok := ParseInbound(payload)
	if !ok {
		t.Fatal("expected ParseInbound to return true for video message")
	}
	if result.MessageType != "video" {
		t.Errorf("expected MessageType 'video', got %q", result.MessageType)
	}
	if result.MediaID != "media_video_001" {
		t.Errorf("expected MediaID 'media_video_001', got %q", result.MediaID)
	}
	if result.Text != "Watch this" {
		t.Errorf("expected Text (caption) 'Watch this', got %q", result.Text)
	}
}

func TestParseInbound_DocumentMessage(t *testing.T) {
	payload := buildWebhookPayload("document", map[string]interface{}{
		"document": map[string]string{
			"id":        "media_doc_001",
			"mime_type": "application/pdf",
			"filename":  "invoice.pdf",
		},
	})

	result, ok := ParseInbound(payload)
	if !ok {
		t.Fatal("expected ParseInbound to return true for document message")
	}
	if result.MessageType != "document" {
		t.Errorf("expected MessageType 'document', got %q", result.MessageType)
	}
	if result.MediaID != "media_doc_001" {
		t.Errorf("expected MediaID 'media_doc_001', got %q", result.MediaID)
	}
	if result.Text != "invoice.pdf" {
		t.Errorf("expected Text (filename) 'invoice.pdf', got %q", result.Text)
	}
}

func TestParseInbound_LocationMessage(t *testing.T) {
	payload := buildWebhookPayload("location", map[string]interface{}{
		"location": map[string]interface{}{
			"latitude":  -23.550520,
			"longitude": -46.633308,
		},
	})

	result, ok := ParseInbound(payload)
	if !ok {
		t.Fatal("expected ParseInbound to return true for location message")
	}
	if result.MessageType != "location" {
		t.Errorf("expected MessageType 'location', got %q", result.MessageType)
	}
	if result.LatLon != "-23.550520,-46.633308" {
		t.Errorf("expected LatLon '-23.550520,-46.633308', got %q", result.LatLon)
	}
	if result.Text != result.LatLon {
		t.Errorf("expected Text to equal LatLon, got Text=%q LatLon=%q", result.Text, result.LatLon)
	}
}

func TestParseInbound_InvalidPayload(t *testing.T) {
	result, ok := ParseInbound([]byte("not json"))
	if ok {
		t.Error("expected false for invalid JSON")
	}
	if result != nil {
		t.Error("expected nil result for invalid JSON")
	}
}

func TestParseInbound_EmptyPayload(t *testing.T) {
	result, ok := ParseInbound([]byte("{}"))
	if ok {
		t.Error("expected false for empty payload")
	}
	if result != nil {
		t.Error("expected nil result for empty payload")
	}
}

func TestParseInbound_WrongObjectType(t *testing.T) {
	payload, _ := json.Marshal(map[string]interface{}{
		"object": "page",
		"entry":  []interface{}{},
	})

	result, ok := ParseInbound(payload)
	if ok {
		t.Error("expected false for non-whatsapp object type")
	}
	if result != nil {
		t.Error("expected nil result for non-whatsapp object type")
	}
}

func TestParseInbound_StatusUpdate_NotMessage(t *testing.T) {
	payload, _ := json.Marshal(map[string]interface{}{
		"object": "whatsapp_business_account",
		"entry": []interface{}{
			map[string]interface{}{
				"id": "biz_123",
				"changes": []interface{}{
					map[string]interface{}{
						"field": "messages",
						"value": map[string]interface{}{
							"messaging_product": "whatsapp",
							"metadata": map[string]string{
								"phone_number_id":      "123456789",
								"display_phone_number": "+5511999999999",
							},
							"statuses": []interface{}{
								map[string]interface{}{
									"id":           "wamid.status123",
									"status":       "delivered",
									"timestamp":    "1234567890",
									"recipient_id": "5511888888888",
								},
							},
						},
					},
				},
			},
		},
	})

	result, ok := ParseInbound(payload)
	if ok {
		t.Error("expected false for status-only update (no messages)")
	}
	if result != nil {
		t.Error("expected nil result for status-only update")
	}
}

func TestParseInbound_UnsupportedMessageType(t *testing.T) {
	// "reaction" is now parsed (handled in-thread). Use a truly unknown
	// type to assert the default-skip path still rejects what it can't
	// recognize.
	payload := buildWebhookPayload("totally_made_up_type", map[string]interface{}{})

	result, ok := ParseInbound(payload)
	if ok {
		t.Error("expected false for unrecognized message type")
	}
	if result != nil {
		t.Error("expected nil result for unrecognized message type")
	}
}

func TestParseInbound_Reaction(t *testing.T) {
	payload := buildWebhookPayload("reaction", map[string]interface{}{
		"reaction": map[string]string{
			"message_id": "wamid.HBgL_orig",
			"emoji":      "👍",
		},
	})

	result, ok := ParseInbound(payload)
	if !ok || result == nil {
		t.Fatal("expected reaction message to parse")
	}
	if result.MessageType != "text" {
		t.Errorf("expected MessageType=text, got %q", result.MessageType)
	}
	if !strings.Contains(result.Text, "👍") {
		t.Errorf("expected reaction text to include emoji, got %q", result.Text)
	}
}

func TestParseInbound_SystemEventSkipsAI(t *testing.T) {
	payload := buildWebhookPayload("system", map[string]interface{}{
		"system": map[string]string{
			"type": "customer_changed_number",
			"body": "User changed number from 5511999999999 to 5511888888888",
		},
	})

	result, ok := ParseInbound(payload)
	if !ok || result == nil {
		t.Fatal("expected system event to parse")
	}
	if !result.Skip {
		t.Error("expected Skip=true on system event so AI/billing fanout is bypassed")
	}
}

// --- VerifySignature Tests ---

func TestVerifySignature_ValidSignature(t *testing.T) {
	adapter := NewWhatsAppAdapter("")
	appSecret := "my_app_secret"
	payload := []byte(`{"object":"whatsapp_business_account"}`)

	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !adapter.VerifySignature(payload, signature, appSecret) {
		t.Error("expected VerifySignature to return true for valid signature")
	}
}

func TestVerifySignature_InvalidSignature(t *testing.T) {
	adapter := NewWhatsAppAdapter("")
	appSecret := "my_app_secret"
	payload := []byte(`{"object":"whatsapp_business_account"}`)

	if adapter.VerifySignature(payload, "sha256=deadbeefdeadbeefdeadbeef", appSecret) {
		t.Error("expected VerifySignature to return false for invalid signature")
	}
}

func TestVerifySignature_MissingSignatureHeader(t *testing.T) {
	adapter := NewWhatsAppAdapter("")

	if adapter.VerifySignature([]byte(`{}`), "", "secret") {
		t.Error("expected VerifySignature to return false for empty signature header")
	}
}

func TestVerifySignature_MalformedPrefix(t *testing.T) {
	adapter := NewWhatsAppAdapter("")

	if adapter.VerifySignature([]byte(`{}`), "md5=abc123", "secret") {
		t.Error("expected VerifySignature to return false for non-sha256 prefix")
	}
}

func TestVerifySignature_ShortHeader(t *testing.T) {
	adapter := NewWhatsAppAdapter("")

	if adapter.VerifySignature([]byte(`{}`), "sha25", "secret") {
		t.Error("expected VerifySignature to return false for header shorter than 7 chars")
	}
}

func TestVerifySignature_WrongSecret(t *testing.T) {
	adapter := NewWhatsAppAdapter("")
	payload := []byte(`{"test":true}`)

	mac := hmac.New(sha256.New, []byte("correct_secret"))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if adapter.VerifySignature(payload, signature, "wrong_secret") {
		t.Error("expected VerifySignature to return false when using wrong secret")
	}
}

// --- parseConfig Tests ---

func TestParseConfig_ValidByteSlice(t *testing.T) {
	channel := &domain.Channel{
		ID:     "ch_001",
		Config: []byte(`{"phone_number_id":"12345","access_token":"tok_abc"}`),
	}

	cfg, err := parseConfig(channel)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.PhoneNumberID != "12345" {
		t.Errorf("expected PhoneNumberID '12345', got %q", cfg.PhoneNumberID)
	}
	if cfg.AccessToken != "tok_abc" {
		t.Errorf("expected AccessToken 'tok_abc', got %q", cfg.AccessToken)
	}
}

func TestParseConfig_ValidJsonRawMessage(t *testing.T) {
	raw := json.RawMessage(`{"phone_number_id":"99999","access_token":"tok_xyz"}`)
	channel := &domain.Channel{
		ID:     "ch_002",
		Config: raw,
	}

	cfg, err := parseConfig(channel)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.PhoneNumberID != "99999" {
		t.Errorf("expected PhoneNumberID '99999', got %q", cfg.PhoneNumberID)
	}
	if cfg.AccessToken != "tok_xyz" {
		t.Errorf("expected AccessToken 'tok_xyz', got %q", cfg.AccessToken)
	}
}

func TestParseConfig_ValidMapInterface(t *testing.T) {
	channel := &domain.Channel{
		ID: "ch_003",
		Config: map[string]interface{}{
			"phone_number_id": "55555",
			"access_token":    "tok_map",
		},
	}

	cfg, err := parseConfig(channel)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.PhoneNumberID != "55555" {
		t.Errorf("expected PhoneNumberID '55555', got %q", cfg.PhoneNumberID)
	}
	if cfg.AccessToken != "tok_map" {
		t.Errorf("expected AccessToken 'tok_map', got %q", cfg.AccessToken)
	}
}

func TestParseConfig_InvalidJSON(t *testing.T) {
	channel := &domain.Channel{
		ID:     "ch_004",
		Config: []byte(`not valid json`),
	}

	_, err := parseConfig(channel)
	if err == nil {
		t.Fatal("expected error for invalid JSON config")
	}
}

func TestParseConfig_NilConfig(t *testing.T) {
	channel := &domain.Channel{
		ID:     "ch_005",
		Config: nil,
	}

	_, err := parseConfig(channel)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestParseConfig_MissingPhoneNumberID(t *testing.T) {
	channel := &domain.Channel{
		ID:     "ch_006",
		Config: []byte(`{"access_token":"tok_abc"}`),
	}

	_, err := parseConfig(channel)
	if err == nil {
		t.Fatal("expected error when phone_number_id is missing")
	}
}

func TestParseConfig_MissingAccessToken_OK(t *testing.T) {
	// access_token is now optional (BSP mode uses app-level token)
	channel := &domain.Channel{
		ID:     "ch_007",
		Config: []byte(`{"phone_number_id":"12345"}`),
	}

	cfg, err := parseConfig(channel)
	if err != nil {
		t.Fatalf("expected no error when access_token is missing (BSP mode), got: %v", err)
	}
	if cfg.PhoneNumberID != "12345" {
		t.Fatalf("expected phone_number_id=12345, got %s", cfg.PhoneNumberID)
	}
	if cfg.AccessToken != "" {
		t.Fatalf("expected empty access_token, got %s", cfg.AccessToken)
	}
}

func TestResolveAccessToken_ChannelConfigTakesPrecedence(t *testing.T) {
	adapter := NewWhatsAppAdapter("app-level-token")
	cfg := &whatsappConfig{PhoneNumberID: "12345", AccessToken: "channel-token"}

	token, err := adapter.resolveAccessToken(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "channel-token" {
		t.Fatalf("expected channel-token, got %s", token)
	}
}

func TestResolveAccessToken_FallbackToAppToken(t *testing.T) {
	adapter := NewWhatsAppAdapter("app-level-token")
	cfg := &whatsappConfig{PhoneNumberID: "12345"}

	token, err := adapter.resolveAccessToken(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "app-level-token" {
		t.Fatalf("expected app-level-token, got %s", token)
	}
}

func TestResolveAccessToken_NoTokenAvailable(t *testing.T) {
	adapter := NewWhatsAppAdapter("")
	cfg := &whatsappConfig{PhoneNumberID: "12345"}

	_, err := adapter.resolveAccessToken(cfg)
	if err == nil {
		t.Fatal("expected error when no access token is available")
	}
}

// --- Helper ---

// buildWebhookPayload constructs a realistic WhatsApp Cloud API webhook JSON payload.
func buildWebhookPayload(msgType string, extras map[string]interface{}) []byte {
	msg := map[string]interface{}{
		"from":      "5511999999999",
		"id":        "wamid.test123",
		"timestamp": "1234567890",
		"type":      msgType,
	}
	for k, v := range extras {
		msg[k] = v
	}

	payload := map[string]interface{}{
		"object": "whatsapp_business_account",
		"entry": []interface{}{
			map[string]interface{}{
				"id": "biz_123",
				"changes": []interface{}{
					map[string]interface{}{
						"field": "messages",
						"value": map[string]interface{}{
							"messaging_product": "whatsapp",
							"metadata": map[string]string{
								"phone_number_id":      "123456789",
								"display_phone_number": "+5511999999999",
							},
							"contacts": []interface{}{
								map[string]interface{}{
									"profile": map[string]string{"name": "John Doe"},
									"wa_id":   "5511999999999",
								},
							},
							"messages": []interface{}{msg},
						},
					},
				},
			},
		},
	}

	data, _ := json.Marshal(payload)
	return data
}
