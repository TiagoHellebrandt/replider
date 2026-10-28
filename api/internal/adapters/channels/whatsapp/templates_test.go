package whatsapp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCreateTemplate_Success(t *testing.T) {
	var received map[string]interface{}
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/waba-99/message_templates") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("missing Bearer token: %q", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"tpl-1"}`))
	}))

	err := CreateTemplate(context.Background(), "waba-99", "tok", TemplatePayload{
		Name:     "boas_vindas",
		Category: "UTILITY",
		Language: "pt_BR",
		Components: []map[string]interface{}{
			{"type": "BODY", "text": "Olá"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received["name"] != "boas_vindas" || received["category"] != "UTILITY" || received["language"] != "pt_BR" {
		t.Errorf("payload not sent correctly: %+v", received)
	}
}

func TestCreateTemplate_HTTPError(t *testing.T) {
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid body component"}}`))
	}))
	err := CreateTemplate(context.Background(), "w", "t", TemplatePayload{Name: "x"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") {
		t.Errorf("expected 400, got: %v", err)
	}
}

func TestDeleteTemplate_EscapesNameInURL(t *testing.T) {
	// Security: template name must be URL-escaped to prevent query hijack.
	// We pass a name with ampersand and verify the server sees it encoded.
	var seenQuery string
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		seenQuery = r.URL.RawQuery
		// The Go server automatically decodes queries into r.URL.Query(),
		// so even an attacker-style name must show as a single name= value.
		q := r.URL.Query()
		if got := q.Get("name"); got != "evil&foo=attacker" {
			t.Errorf("name should round-trip as-is after server decode, got %q", got)
		}
		if q.Get("foo") != "" {
			t.Error("attacker-provided foo param must NOT appear as a separate query")
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))

	err := DeleteTemplate(context.Background(), "waba", "tok", "evil&foo=attacker")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Raw query should URL-encode the & as %26 (defense against injection)
	if !strings.Contains(seenQuery, "evil%26foo%3Dattacker") {
		t.Errorf("raw query should have escaped &/=, got %q", seenQuery)
	}
}

func TestDeleteTemplate_Success(t *testing.T) {
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Error("missing Authorization header")
		}
		q := r.URL.Query()
		if q.Get("name") != "hello" {
			t.Errorf("name query=%q", q.Get("name"))
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))

	err := DeleteTemplate(context.Background(), "waba", "tok", "hello")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestDeleteTemplate_HTTPError(t *testing.T) {
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	err := DeleteTemplate(context.Background(), "w", "t", "x")
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("expected 404, got: %v", err)
	}
}
