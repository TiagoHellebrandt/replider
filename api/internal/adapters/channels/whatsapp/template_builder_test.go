package whatsapp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replider/api/internal/core/domain"
)

// A marketing/approach template WITH an opt-out quick-reply button builds
// cleanly: type=template, name/language wired, BODY params bound from the
// resolved lead/contact values.
func TestBuildTemplatePayload_MarketingWithOptOut(t *testing.T) {
	spec := templateSpec{
		Name:       "abordagem_inicial",
		Language:   "pt_BR",
		Category:   "MARKETING",
		BodyParams: []string{"João", "Plano Pro"},
		Buttons: []templateButton{
			{SubType: "quick_reply", Text: "Quero saber mais", Payload: "MORE"},
			{SubType: "quick_reply", Text: "Parar de receber"},
		},
	}

	tmpl, err := buildTemplatePayload(spec)
	if err != nil {
		t.Fatalf("expected build to succeed, got %v", err)
	}
	if tmpl["name"] != "abordagem_inicial" {
		t.Errorf("name mismatch: %v", tmpl["name"])
	}
	lang, _ := tmpl["language"].(map[string]string)
	if lang["code"] != "pt_BR" {
		t.Errorf("language code mismatch: %v", tmpl["language"])
	}

	comps, ok := tmpl["components"].([]interface{})
	if !ok || len(comps) == 0 {
		t.Fatalf("expected components, got %v", tmpl["components"])
	}
	body, _ := comps[0].(map[string]interface{})
	if body["type"] != "body" {
		t.Fatalf("expected first component to be body, got %v", body["type"])
	}
	params, _ := body["parameters"].([]map[string]interface{})
	if len(params) != 2 || params[0]["text"] != "João" || params[1]["text"] != "Plano Pro" {
		t.Errorf("body params not bound from lead data: %v", params)
	}
}

// Anti-ban guard: a MARKETING template without any opt-out button is
// rejected before any network call.
func TestBuildTemplatePayload_MarketingWithoutOptOutRejected(t *testing.T) {
	spec := templateSpec{
		Name:       "abordagem_inicial",
		Category:   "MARKETING",
		BodyParams: []string{"João"},
		Buttons: []templateButton{
			{SubType: "quick_reply", Text: "Quero saber mais", Payload: "MORE"},
		},
	}

	_, err := buildTemplatePayload(spec)
	if !errors.Is(err, ErrTemplateMissingOptOut) {
		t.Fatalf("expected ErrTemplateMissingOptOut, got %v", err)
	}
}

// A non-marketing (UTILITY) template is exempt from the opt-out guard.
func TestBuildTemplatePayload_UtilityExemptFromOptOut(t *testing.T) {
	spec := templateSpec{
		Name:       "lembrete_consulta",
		Category:   "UTILITY",
		BodyParams: []string{"10/05 às 14h"},
	}
	tmpl, err := buildTemplatePayload(spec)
	if err != nil {
		t.Fatalf("expected utility template to build, got %v", err)
	}
	if tmpl["name"] != "lembrete_consulta" {
		t.Errorf("name mismatch: %v", tmpl["name"])
	}
}

// Default language is applied when the spec omits one.
func TestBuildTemplatePayload_DefaultsLanguage(t *testing.T) {
	tmpl, err := buildTemplatePayload(templateSpec{Name: "x", Category: "UTILITY"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lang, _ := tmpl["language"].(map[string]string)
	if lang["code"] != defaultTemplateLanguage {
		t.Errorf("expected default language %q, got %v", defaultTemplateLanguage, lang)
	}
}

func TestBuildTemplatePayload_MissingNameRejected(t *testing.T) {
	if _, err := buildTemplatePayload(templateSpec{Category: "UTILITY"}); err == nil {
		t.Fatal("expected error for empty template name")
	}
}

// fillTemplateParams resolves ordered lead/contact field keys into
// positional BODY values, preserving order and tolerating missing keys.
func TestFillTemplateParams_FromLeadData(t *testing.T) {
	data := map[string]string{"contact_name": "Maria", "product_name": "Plano X"}
	got := fillTemplateParams([]string{"contact_name", "product_name", "missing"}, data)
	if len(got) != 3 || got[0] != "Maria" || got[1] != "Plano X" || got[2] != "" {
		t.Errorf("unexpected resolved params: %v", got)
	}
}

func TestIsOptOutText(t *testing.T) {
	cases := map[string]bool{
		"Parar de receber":   true,
		"PARAR":              true,
		"Cancelar inscrição": true,
		"Stop":               true,
		"Unsubscribe":        true,
		"Quero saber mais":   false,
		"Comprar agora":      false,
		"":                   false,
	}
	for text, want := range cases {
		if got := isOptOutText(text); got != want {
			t.Errorf("isOptOutText(%q)=%v, want %v", text, got, want)
		}
	}
}

// parseTemplateSpecFromMeta resolves BODY params from template_variables +
// template_data when template_body_params is absent (variables filled
// from lead/contact data), and parses the button definitions.
func TestParseTemplateSpecFromMeta_ResolvesVariables(t *testing.T) {
	meta := map[string]interface{}{
		"template_name":      "abordagem",
		"template_language":  "pt_BR",
		"template_category":  "MARKETING",
		"template_variables": []interface{}{"contact_name"},
		"template_data":      map[string]interface{}{"contact_name": "Ana"},
		"template_buttons": []interface{}{
			map[string]interface{}{"sub_type": "quick_reply", "text": "Parar de receber"},
		},
	}
	spec := parseTemplateSpecFromMeta(meta)
	if len(spec.BodyParams) != 1 || spec.BodyParams[0] != "Ana" {
		t.Errorf("expected BodyParams resolved from lead data, got %v", spec.BodyParams)
	}
	if !spec.hasOptOutButton() {
		t.Errorf("expected opt-out button parsed from metadata")
	}
}

// End-to-end through SendMessageAndGetID: a Type=="template" message with
// metadata produces a type:template Cloud API payload and returns the wamid.
func TestSendMessage_TemplateFromMetadata(t *testing.T) {
	var sent map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &sent)
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.TPL2"}]}`))
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
	msg := &domain.Message{
		Type: "template",
		Metadata: map[string]interface{}{
			"template_name":        "abordagem",
			"template_language":    "pt_BR",
			"template_category":    "MARKETING",
			"template_body_params": []interface{}{"João"},
			"template_buttons": []interface{}{
				map[string]interface{}{"sub_type": "quick_reply", "text": "Parar de receber"},
			},
		},
	}
	wamid, err := adapter.SendMessageAndGetID(context.Background(), channel, "+55 11 99999-9999", msg)
	if err != nil {
		t.Fatalf("template send via metadata failed: %v", err)
	}
	if wamid != "wamid.TPL2" {
		t.Errorf("expected wamid.TPL2, got %q", wamid)
	}
	if sent["type"] != "template" {
		t.Errorf("expected type=template, got %v", sent["type"])
	}
	tmpl, ok := sent["template"].(map[string]any)
	if !ok || tmpl["name"] != "abordagem" {
		t.Fatalf("expected template name=abordagem, got %v", sent["template"])
	}
}

// A marketing template missing its opt-out button is rejected by
// SendMessageAndGetID before any HTTP call is made.
func TestSendMessage_TemplateMarketingWithoutOptOutRejected(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.X"}]}`))
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
	msg := &domain.Message{
		Type: "template",
		Metadata: map[string]interface{}{
			"template_name":        "abordagem",
			"template_category":    "MARKETING",
			"template_body_params": []interface{}{"João"},
		},
	}
	_, err := adapter.SendMessageAndGetID(context.Background(), channel, "+55 11 99999-9999", msg)
	if !errors.Is(err, ErrTemplateMissingOptOut) {
		t.Fatalf("expected ErrTemplateMissingOptOut, got %v", err)
	}
	if called {
		t.Error("expected NO HTTP call when opt-out guard rejects the send")
	}
}
