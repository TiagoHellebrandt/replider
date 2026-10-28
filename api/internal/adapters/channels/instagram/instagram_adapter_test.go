package instagram

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realistic Instagram messaging webhook body (Meta App Dashboard →
// Webhooks → Instagram object, `messages` field subscription).
const igTextPayload = `{
	"object": "instagram",
	"entry": [{
		"id": "17841400000000001",
		"time": 1720000000,
		"messaging": [{
			"sender": {"id": "1254459081234567"},
			"recipient": {"id": "17841400000000001"},
			"timestamp": 1720000000123,
			"message": {
				"mid": "aWdfZAG1faXRlbTox",
				"text": "Oi! Vocês têm horário amanhã?"
			}
		}]
	}]
}`

func TestParseWebhookPayload_TextMessage(t *testing.T) {
	messages, err := ParseWebhookPayload([]byte(igTextPayload))
	require.NoError(t, err)
	require.Len(t, messages, 1)

	m := messages[0]
	assert.Equal(t, "1254459081234567", m.SenderID)
	assert.Equal(t, "aWdfZAG1faXRlbTox", m.MessageID)
	assert.Equal(t, "Oi! Vocês têm horário amanhã?", m.Text)
	assert.Equal(t, "text", m.MediaType)
	assert.Empty(t, m.MediaURL)
}

func TestParseWebhookPayload_EchoSkipped(t *testing.T) {
	// Meta reflects the page's own outbound (including our bot replies)
	// back to the webhook with message.is_echo=true. Feeding those into
	// the pipeline would loop the bot on its own replies.
	echo := `{
		"object": "instagram",
		"entry": [{
			"id": "17841400000000001",
			"time": 1720000000,
			"messaging": [{
				"sender": {"id": "17841400000000001"},
				"recipient": {"id": "1254459081234567"},
				"timestamp": 1720000000456,
				"message": {
					"mid": "aWdfZAG1fZWNobzoy",
					"text": "Temos sim! Qual horário prefere?",
					"is_echo": true
				}
			}]
		}]
	}`
	messages, err := ParseWebhookPayload([]byte(echo))
	require.NoError(t, err)
	assert.Empty(t, messages)
}

func TestParseWebhookPayload_EchoInBatchSkippedOthersKept(t *testing.T) {
	batch := `{
		"object": "instagram",
		"entry": [{
			"id": "17841400000000001",
			"messaging": [
				{
					"sender": {"id": "17841400000000001"},
					"recipient": {"id": "1254459081234567"},
					"message": {"mid": "echo-1", "text": "bot reply", "is_echo": true}
				},
				{
					"sender": {"id": "1254459081234567"},
					"recipient": {"id": "17841400000000001"},
					"message": {"mid": "real-1", "text": "customer text"}
				}
			]
		}]
	}`
	messages, err := ParseWebhookPayload([]byte(batch))
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "real-1", messages[0].MessageID)
}

func TestParseWebhookPayload_Attachment(t *testing.T) {
	att := `{
		"object": "instagram",
		"entry": [{
			"id": "17841400000000001",
			"messaging": [{
				"sender": {"id": "1254459081234567"},
				"recipient": {"id": "17841400000000001"},
				"message": {
					"mid": "aWdfbWVkaWE6Mw",
					"attachments": [{
						"type": "image",
						"payload": {"url": "https://lookaside.fbsbx.com/ig_messaging_cdn/?asset_id=123"}
					}]
				}
			}]
		}]
	}`
	messages, err := ParseWebhookPayload([]byte(att))
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "image", messages[0].MediaType)
	assert.Equal(t, "https://lookaside.fbsbx.com/ig_messaging_cdn/?asset_id=123", messages[0].MediaURL)
	assert.Equal(t, "[image]", messages[0].Text)
}

func TestParseWebhookPayload_NonMessageEventIgnored(t *testing.T) {
	// Read receipts / reactions arrive without a `message` object.
	read := `{
		"object": "instagram",
		"entry": [{
			"id": "17841400000000001",
			"messaging": [{
				"sender": {"id": "1254459081234567"},
				"recipient": {"id": "17841400000000001"},
				"read": {"mid": "aWdfZAG1faXRlbTox"}
			}]
		}]
	}`
	messages, err := ParseWebhookPayload([]byte(read))
	require.NoError(t, err)
	assert.Empty(t, messages)
}

func TestParseWebhookPayload_InvalidJSON(t *testing.T) {
	_, err := ParseWebhookPayload([]byte("not json{"))
	assert.Error(t, err)
}

// ────────────── SendMessage — media type mapping (US-011) ──────────────

func strPtr(s string) *string { return &s }

func igTestChannel() *domain.Channel {
	return &domain.Channel{
		ID:     "ch-ig-1",
		Type:   "instagram",
		Config: json.RawMessage(`{"page_id":"17841400000000001","access_token":"IGQVJ..."}`),
	}
}

func TestSendMessage_VideoAndAudioAsAttachment(t *testing.T) {
	for _, mediaType := range []string{"video", "audio", "image"} {
		var captured map[string]interface{}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &captured)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"message_id":"m1"}`))
		}))

		a := NewInstagramAdapter().WithBaseURL(srv.URL)
		err := a.SendMessage(context.Background(), igTestChannel(), "1254459081234567", &domain.Message{
			Type:     mediaType,
			MediaURL: strPtr("https://cdn.example/file"),
		})
		srv.Close()

		require.NoError(t, err, mediaType)
		msg, _ := captured["message"].(map[string]interface{})
		require.NotNil(t, msg, mediaType)
		// Non-text media must ride as an attachment — falling through to
		// the text branch produces {"text":""} which Meta rejects.
		att, _ := msg["attachment"].(map[string]interface{})
		require.NotNil(t, att, "media type %s must be sent as attachment, got %v", mediaType, msg)
		assert.Equal(t, mediaType, att["type"], mediaType)
	}
}

func TestSendMessage_UnsupportedMediaWithoutTextErrors(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	a := NewInstagramAdapter().WithBaseURL(srv.URL)
	// "document" has no Instagram attachment mapping and there is no
	// caption to fall back on: sending {"text":""} would die at Meta
	// with an opaque 400, so the adapter must fail fast instead.
	err := a.SendMessage(context.Background(), igTestChannel(), "1254459081234567", &domain.Message{
		Type:     "document",
		MediaURL: strPtr("https://cdn.example/contract.pdf"),
	})
	require.Error(t, err)
	assert.False(t, called, "no HTTP call may be made for an unsendable message")
}

func TestSendMessage_UnsupportedMediaWithCaptionFallsBackToText(t *testing.T) {
	var captured map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		_, _ = w.Write([]byte(`{"message_id":"m1"}`))
	}))
	defer srv.Close()

	a := NewInstagramAdapter().WithBaseURL(srv.URL)
	err := a.SendMessage(context.Background(), igTestChannel(), "1254459081234567", &domain.Message{
		Type:     "document",
		Content:  strPtr("segue o contrato: https://cdn.example/contract.pdf"),
		MediaURL: strPtr("https://cdn.example/contract.pdf"),
	})
	require.NoError(t, err)
	msg, _ := captured["message"].(map[string]interface{})
	require.NotNil(t, msg)
	assert.Equal(t, "segue o contrato: https://cdn.example/contract.pdf", msg["text"])
}

// ────────────── FetchSenderName (US-011) ──────────────

func TestFetchSenderName_PrefersNameFallsBackToUsername(t *testing.T) {
	cases := []struct {
		body string
		want string
	}{
		{`{"name":"Maria Silva","username":"maria.silva"}`, "Maria Silva"},
		{`{"username":"maria.silva"}`, "maria.silva"},
		{`{}`, ""},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/1254459081234567", r.URL.Path)
			assert.Equal(t, "name,username", r.URL.Query().Get("fields"))
			assert.Equal(t, "Bearer IGQVJ...", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(tc.body))
		}))

		a := NewInstagramAdapter().WithBaseURL(srv.URL)
		name, err := a.FetchSenderName(context.Background(), igTestChannel(), "1254459081234567")
		srv.Close()

		require.NoError(t, err, tc.body)
		assert.Equal(t, tc.want, name, tc.body)
	}
}

func TestFetchSenderName_APIErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"expired token"}}`))
	}))
	defer srv.Close()

	a := NewInstagramAdapter().WithBaseURL(srv.URL)
	_, err := a.FetchSenderName(context.Background(), igTestChannel(), "1254459081234567")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func signIG(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifySignature_Valid(t *testing.T) {
	body := []byte(igTextPayload)
	sig := signIG(body, "app-secret")
	assert.True(t, VerifySignature(body, sig, "app-secret"))
}

func TestVerifySignature_UppercasePrefixAccepted(t *testing.T) {
	body := []byte(igTextPayload)
	sig := signIG(body, "app-secret")
	sig = "SHA256=" + sig[len("sha256="):]
	assert.True(t, VerifySignature(body, sig, "app-secret"))
}

func TestVerifySignature_Rejections(t *testing.T) {
	body := []byte(igTextPayload)
	valid := signIG(body, "app-secret")

	assert.False(t, VerifySignature(body, valid, "other-secret"), "wrong secret")
	assert.False(t, VerifySignature([]byte("tampered"), valid, "app-secret"), "tampered body")
	assert.False(t, VerifySignature(body, "", "app-secret"), "empty header")
	assert.False(t, VerifySignature(body, "sha256=", "app-secret"), "degenerate header")
	assert.False(t, VerifySignature(body, valid+"00", "app-secret"), "oversized header")
	assert.False(t, VerifySignature(body, "sha257="+strings.Repeat("a", 64), "app-secret"), "wrong prefix, right length")
}
