package whatsapp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
)

// captureSend stands up a fake Graph endpoint, swaps metaGraphBaseURL,
// and returns the body the SendMessage path POSTed to /messages so we
// can assert the wire-shape per message type.
func captureSend(t *testing.T, msg *domain.Message, response string) (sent map[string]any, wamid string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/messages") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &sent)
		if response == "" {
			response = `{"messages":[{"id":"wamid.OUT-1"}]}`
		}
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(srv.Close)

	prev := metaGraphBaseURL
	metaGraphBaseURL = srv.URL
	t.Cleanup(func() { metaGraphBaseURL = prev })

	adapter := NewWhatsAppAdapter("")
	channel := &domain.Channel{
		ID:     "ch-test",
		Type:   "whatsapp",
		Config: []byte(`{"phone_number_id":"pn-1","access_token":"tok"}`),
	}
	id, err := adapter.SendMessageAndGetID(context.Background(), channel, "+55 11 99999-9999", msg)
	if err != nil {
		t.Fatalf("SendMessageAndGetID returned error: %v", err)
	}
	return sent, id
}

func TestSendMessage_TextWithURLSetsPreviewURL(t *testing.T) {
	body := "veja https://exemplo.com agora"
	sent, _ := captureSend(t, &domain.Message{Type: "text", Content: &body}, "")
	textPayload, ok := sent["text"].(map[string]any)
	if !ok {
		t.Fatalf("expected text payload, got %v", sent)
	}
	if textPayload["preview_url"] != true {
		t.Errorf("expected preview_url=true on text with URL, got %v", textPayload["preview_url"])
	}
}

func TestSendMessage_TextWithoutURLOmitsPreviewURL(t *testing.T) {
	body := "olá tudo bem"
	sent, _ := captureSend(t, &domain.Message{Type: "text", Content: &body}, "")
	textPayload, _ := sent["text"].(map[string]any)
	if _, has := textPayload["preview_url"]; has {
		t.Errorf("expected preview_url omitted on plain text, got %v", textPayload)
	}
}

func TestSendMessage_RecipientPhoneNormalized(t *testing.T) {
	body := "oi"
	sent, _ := captureSend(t, &domain.Message{Type: "text", Content: &body}, "")
	if sent["to"] != "5511999999999" {
		t.Errorf("expected to=5511999999999 (normalized), got %v", sent["to"])
	}
}

func TestSendMessage_WamidExtracted(t *testing.T) {
	body := "oi"
	_, wamid := captureSend(t, &domain.Message{Type: "text", Content: &body},
		`{"messages":[{"id":"wamid.ABC123"}]}`)
	if wamid != "wamid.ABC123" {
		t.Errorf("expected wamid.ABC123, got %q", wamid)
	}
}

func TestSendMessage_DocumentUsesMetadataFilename(t *testing.T) {
	caption := "Olá, segue o PDF."
	mediaURL := "https://media.example/doc.pdf"
	msg := &domain.Message{
		Type:     "document",
		Content:  &caption,
		MediaURL: &mediaURL,
		Metadata: map[string]string{"filename": "contrato_2026.pdf"},
	}
	sent, _ := captureSend(t, msg, "")
	doc, ok := sent["document"].(map[string]any)
	if !ok {
		t.Fatalf("expected document payload, got %v", sent)
	}
	if doc["filename"] != "contrato_2026.pdf" {
		t.Errorf("expected filename from metadata, got %v", doc["filename"])
	}
	if doc["caption"] != caption {
		t.Errorf("expected caption from Content, got %v", doc["caption"])
	}
	if doc["link"] != mediaURL {
		t.Errorf("expected link from MediaURL, got %v", doc["link"])
	}
}

// Audio sends try to transcode + upload via /{phone-number-id}/media
// so Meta validates a media_id (audio.id) rather than fetching a link
// whose codec/container often mismatches what the dashboard uploaded.
// When the local fetch/transcode/upload chain fails (unreachable URL,
// ffmpeg missing in dev, Meta upload error), the adapter falls back to
// the legacy audio.link path so we don't regress senders that already
// work (mobile-recorded m4a, hosted CDN URLs Meta accepts directly).
func TestSendMessage_AudioFallsBackToLinkWhenTranscodeFails(t *testing.T) {
	// 404 from the fetch leg fast-fails the chain (transcode + upload
	// never run) without paying the retry/backoff cost an unrouted host
	// would impose.
	mediaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(mediaSrv.Close)
	mediaURL := mediaSrv.URL + "/audio.m4a"
	msg := &domain.Message{
		Type:     "voice",
		MediaURL: &mediaURL,
	}
	sent, _ := captureSend(t, msg, "")
	audio, ok := sent["audio"].(map[string]any)
	if !ok {
		t.Fatalf("expected audio payload, got %v", sent)
	}
	if audio["link"] != mediaURL {
		t.Errorf("expected fallback link=%q, got %v", mediaURL, audio["link"])
	}
	if _, has := audio["id"]; has {
		t.Errorf("expected no audio.id in fallback path, got %v", audio["id"])
	}
}

func TestSendMessage_ReactionUsesMetadataTarget(t *testing.T) {
	emoji := "👍"
	msg := &domain.Message{
		Type:     "reaction",
		Content:  &emoji,
		Metadata: map[string]string{"wa_reaction_to": "wamid.PARENT"},
	}
	sent, _ := captureSend(t, msg, "")
	r, ok := sent["reaction"].(map[string]any)
	if !ok {
		t.Fatalf("expected reaction payload, got %v", sent)
	}
	if r["message_id"] != "wamid.PARENT" {
		t.Errorf("expected message_id=wamid.PARENT, got %v", r["message_id"])
	}
	if r["emoji"] != emoji {
		t.Errorf("expected emoji propagated, got %v", r["emoji"])
	}
}

func TestSendMessage_ReplyContextWired(t *testing.T) {
	body := "claro"
	msg := &domain.Message{
		Type:     "text",
		Content:  &body,
		Metadata: map[string]string{"wa_reply_to": "wamid.PARENT"},
	}
	sent, _ := captureSend(t, msg, "")
	ctx, ok := sent["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context payload, got %v", sent)
	}
	if ctx["message_id"] != "wamid.PARENT" {
		t.Errorf("expected context.message_id=wamid.PARENT, got %v", ctx["message_id"])
	}
}

func TestSubscribeApp_TreatsAny2xxAsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulates the idempotent re-subscribe path: 200 with a body
		// that does NOT have "success":true. Old code treated this as
		// failure; we want it to pass.
		_, _ = w.Write([]byte(`{"data":[{"whatsapp_business_api_data":{"id":"123"}}]}`))
	}))
	t.Cleanup(srv.Close)
	prev := metaGraphBaseURL
	metaGraphBaseURL = srv.URL
	t.Cleanup(func() { metaGraphBaseURL = prev })

	if err := SubscribeApp(context.Background(), "waba-1", "tok"); err != nil {
		t.Fatalf("expected nil error on 2xx, got %v", err)
	}
}

// --- SendTemplateAndGetID: components forwarded as-is + phone normalized ---

func TestSendTemplate_ComponentsForwarded(t *testing.T) {
	var sent map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &sent)
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.TPL"}]}`))
	}))
	t.Cleanup(srv.Close)
	prev := metaGraphBaseURL
	metaGraphBaseURL = srv.URL
	t.Cleanup(func() { metaGraphBaseURL = prev })

	adapter := NewWhatsAppAdapter("")
	channel := &domain.Channel{
		ID: "ch", Type: "whatsapp",
		Config: []byte(`{"phone_number_id":"pn-1","access_token":"tok"}`),
	}
	components := []interface{}{
		map[string]any{
			"type": "body",
			"parameters": []map[string]any{
				{"type": "text", "text": "João"},
				{"type": "text", "text": "10/05"},
			},
		},
	}
	wamid, err := adapter.SendTemplateAndGetID(context.Background(), channel,
		"+55 11 99999-9999", "appointment_reminder", "pt_BR", components)
	if err != nil {
		t.Fatalf("template send failed: %v", err)
	}
	if wamid != "wamid.TPL" {
		t.Errorf("expected wamid.TPL, got %q", wamid)
	}
	if sent["to"] != "5511999999999" {
		t.Errorf("expected normalized phone, got %v", sent["to"])
	}
	tmpl, ok := sent["template"].(map[string]any)
	if !ok {
		t.Fatalf("expected template payload, got %v", sent)
	}
	if tmpl["name"] != "appointment_reminder" {
		t.Errorf("template name mismatch: %v", tmpl["name"])
	}
	lang, _ := tmpl["language"].(map[string]any)
	if lang["code"] != "pt_BR" {
		t.Errorf("language code mismatch: %v", lang)
	}
	comps, ok := tmpl["components"].([]any)
	if !ok || len(comps) != 1 {
		t.Fatalf("expected 1 component forwarded, got %v", tmpl["components"])
	}
}

// --- ListTemplates pagination ---

func TestListTemplates_FollowsPagingNext(t *testing.T) {
	var calls int
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	prev := metaGraphBaseURL
	metaGraphBaseURL = srv.URL
	t.Cleanup(func() { metaGraphBaseURL = prev })

	mux.HandleFunc("/v25.0/waba-1/message_templates", func(w http.ResponseWriter, r *http.Request) {
		calls++
		// First page links to the second; second page has no `next`.
		if r.URL.Query().Get("after") == "p2" {
			_, _ = w.Write([]byte(`{"data":[{"name":"t3","status":"APPROVED"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{
		  "data":[{"name":"t1","status":"APPROVED"},{"name":"t2","status":"APPROVED"}],
		  "paging":{"next":"` + srv.URL + `/v25.0/waba-1/message_templates?after=p2"}
		}`))
	})

	all, err := ListTemplates(context.Background(), "waba-1", "tok")
	if err != nil {
		t.Fatalf("ListTemplates failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 templates across 2 pages, got %d", len(all))
	}
	if calls != 2 {
		t.Errorf("expected 2 HTTP calls (page 1 + page 2), got %d", calls)
	}
}

func TestSubscribeApp_ErrorOn4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":190,"message":"token expired"}}`))
	}))
	t.Cleanup(srv.Close)
	prev := metaGraphBaseURL
	metaGraphBaseURL = srv.URL
	t.Cleanup(func() { metaGraphBaseURL = prev })

	err := SubscribeApp(context.Background(), "waba-1", "tok")
	if err == nil {
		t.Fatal("expected error on 4xx")
	}
	meta, ok := err.(*MetaError)
	if !ok {
		t.Fatalf("expected typed MetaError, got %T", err)
	}
	if meta.Code != 190 {
		t.Errorf("expected meta code 190, got %d", meta.Code)
	}
}
