package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

// --- ParseInboundAll: multi-message batch ---

func TestParseInboundAll_MultiMessageBatch(t *testing.T) {
	// Meta delivers up to 100 messages per webhook POST. The legacy
	// ParseInbound returned only the first; ParseInboundAll surfaces
	// every entry so a burst doesn't lose messages 2..N.
	body := buildBatchPayload([]map[string]interface{}{
		{"from": "5511111111111", "id": "wamid.A", "type": "text", "text": map[string]string{"body": "msg one"}},
		{"from": "5511111111112", "id": "wamid.B", "type": "text", "text": map[string]string{"body": "msg two"}},
		{"from": "5511111111113", "id": "wamid.C", "type": "text", "text": map[string]string{"body": "msg three"}},
	})

	all := ParseInboundAll(body)
	if len(all) != 3 {
		t.Fatalf("expected 3 parsed messages, got %d", len(all))
	}
	want := []string{"msg one", "msg two", "msg three"}
	for i, p := range all {
		if p.Text != want[i] {
			t.Errorf("message %d: expected %q, got %q", i, want[i], p.Text)
		}
	}
}

func TestParseInboundAll_EmptyOnNonMessageObject(t *testing.T) {
	out := ParseInboundAll([]byte(`{"object":"page","entry":[]}`))
	if out != nil {
		t.Fatalf("expected nil for non-whatsapp object, got %v", out)
	}
}

func TestParseInboundAll_HandlesMultipleEntries(t *testing.T) {
	// Meta can deliver messages from multiple WABAs in one POST when
	// the BSP has multiple businesses subscribed to the same app. The
	// parser must walk every entry, not stop at the first.
	payload := map[string]interface{}{
		"object": "whatsapp_business_account",
		"entry": []interface{}{
			map[string]interface{}{
				"id": "biz_A",
				"changes": []interface{}{map[string]interface{}{
					"field": "messages",
					"value": map[string]interface{}{
						"metadata": map[string]string{"phone_number_id": "pn-A"},
						"messages": []interface{}{map[string]interface{}{
							"from": "55", "id": "wamid.A", "type": "text", "timestamp": "1",
							"text": map[string]string{"body": "from A"},
						}},
					},
				}},
			},
			map[string]interface{}{
				"id": "biz_B",
				"changes": []interface{}{map[string]interface{}{
					"field": "messages",
					"value": map[string]interface{}{
						"metadata": map[string]string{"phone_number_id": "pn-B"},
						"messages": []interface{}{map[string]interface{}{
							"from": "55", "id": "wamid.B", "type": "text", "timestamp": "1",
							"text": map[string]string{"body": "from B"},
						}},
					},
				}},
			},
		},
	}
	body, _ := json.Marshal(payload)
	all := ParseInboundAll(body)
	if len(all) != 2 {
		t.Fatalf("expected 2 messages from 2 entries, got %d", len(all))
	}
	if all[0].PhoneNumberID != "pn-A" || all[1].PhoneNumberID != "pn-B" {
		t.Errorf("expected pn-A then pn-B, got %s / %s",
			all[0].PhoneNumberID, all[1].PhoneNumberID)
	}
}

func TestParseInboundAll_DropsUnrecognizedTypeButKeepsRest(t *testing.T) {
	// Recognised text + unrecognised "future_type" → first should make
	// it through, second should be silently dropped, returning length 1.
	body := buildBatchPayload([]map[string]interface{}{
		{"from": "5511111111111", "id": "wamid.A", "type": "text", "text": map[string]string{"body": "ok"}},
		{"from": "5511111111112", "id": "wamid.B", "type": "future_type"},
	})
	all := ParseInboundAll(body)
	if len(all) != 1 {
		t.Fatalf("expected 1 parsed (unknown dropped), got %d", len(all))
	}
}

// --- ParseInboundAll: referral attribution + reply-context ---

func TestParseInboundAll_ReferralAttachedToMetadata(t *testing.T) {
	body := buildBatchPayload([]map[string]interface{}{
		{
			"from": "5511111111111",
			"id":   "wamid.A",
			"type": "text",
			"text": map[string]string{"body": "vim do anúncio"},
			"referral": map[string]string{
				"source_url":  "https://fb.me/promo123",
				"source_type": "ad",
				"source_id":   "23874",
				"headline":    "20% off hoje",
				"ctwa_clid":   "abc-clid",
			},
		},
	})
	all := ParseInboundAll(body)
	if len(all) != 1 {
		t.Fatalf("expected 1 parsed, got %d", len(all))
	}
	var meta map[string]any
	if err := json.Unmarshal(all[0].Metadata, &meta); err != nil {
		t.Fatalf("metadata not valid JSON: %v", err)
	}
	ref, ok := meta["referral"].(map[string]any)
	if !ok {
		t.Fatalf("expected referral key in metadata, got %v", meta)
	}
	if ref["ctwa_clid"] != "abc-clid" {
		t.Errorf("expected ctwa_clid=abc-clid, got %v", ref["ctwa_clid"])
	}
	if ref["source_type"] != "ad" {
		t.Errorf("expected source_type=ad, got %v", ref["source_type"])
	}
}

func TestParseInboundAll_ReplyContextIDInMetadata(t *testing.T) {
	body := buildBatchPayload([]map[string]interface{}{
		{
			"from":    "5511111111111",
			"id":      "wamid.A",
			"type":    "text",
			"text":    map[string]string{"body": "reply"},
			"context": map[string]string{"id": "wamid.PARENT", "from": "5511111111112"},
		},
	})
	all := ParseInboundAll(body)
	if len(all) != 1 {
		t.Fatalf("expected 1 parsed, got %d", len(all))
	}
	var meta map[string]any
	_ = json.Unmarshal(all[0].Metadata, &meta)
	if meta["reply_to_wamid"] != "wamid.PARENT" {
		t.Errorf("expected reply_to_wamid=wamid.PARENT, got %v", meta["reply_to_wamid"])
	}
}

// --- Interactive: button id + list id surface in Text + Metadata ---

func TestParseInboundAll_ButtonReplyCarriesIDInText(t *testing.T) {
	body := buildBatchPayload([]map[string]interface{}{
		{
			"from": "5511111111111", "id": "wamid.A", "type": "interactive",
			"interactive": map[string]any{
				"type":         "button_reply",
				"button_reply": map[string]any{"id": "confirm_yes", "title": "Sim"},
			},
		},
	})
	all := ParseInboundAll(body)
	if len(all) != 1 {
		t.Fatalf("expected 1 parsed, got %d", len(all))
	}
	if !strings.Contains(all[0].Text, "confirm_yes") {
		t.Errorf("expected button_id in Text, got %q", all[0].Text)
	}
	if !strings.Contains(all[0].Text, "Sim") {
		t.Errorf("expected title in Text, got %q", all[0].Text)
	}
}

func TestParseInboundAll_ListReplyCarriesIDAndDescription(t *testing.T) {
	body := buildBatchPayload([]map[string]interface{}{
		{
			"from": "5511111111111", "id": "wamid.A", "type": "interactive",
			"interactive": map[string]any{
				"type": "list_reply",
				"list_reply": map[string]any{
					"id":          "row_42",
					"title":       "Plano Pro",
					"description": "R$ 99/mes — 100 conversas",
				},
			},
		},
	})
	all := ParseInboundAll(body)
	if len(all) != 1 {
		t.Fatalf("expected 1 parsed, got %d", len(all))
	}
	if !strings.Contains(all[0].Text, "row_42") {
		t.Errorf("expected list_id in Text, got %q", all[0].Text)
	}
	if !strings.Contains(all[0].Text, "R$ 99/mes") {
		t.Errorf("expected description in Text, got %q", all[0].Text)
	}
}

// --- normalizeWAPhone ---

func TestNormalizeWAPhone(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"+55 11 99999-9999", "5511999999999"},
		{"(11) 99999-9999", "11999999999"},
		{"5511999999999", "5511999999999"},
		{"  5511 999999999  ", "5511999999999"},
		{"", ""},
		{"+++", ""},
	}
	for _, c := range cases {
		got := normalizeWAPhone(c.in)
		if got != c.want {
			t.Errorf("normalizeWAPhone(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- containsURL drives preview_url toggle on outbound text ---

func TestContainsURL(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"olá tudo bem", false},
		{"veja https://exemplo.com agora", true},
		{"http://abc.com", true},
		{"HTTPS://CASE.COM", true},
		{"sem link mas com www.exemplo.com", false},
	}
	for _, c := range cases {
		if got := containsURL(c.in); got != c.want {
			t.Errorf("containsURL(%q)=%v, want %v", c.in, got, c.want)
		}
	}
}

// --- isAllowedMediaHost SSRF guard ---

func TestIsAllowedMediaHost(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"lookaside.fbsbx.com", true},
		{"scontent-iad3-2.fbcdn.net", true},
		{"sub.cdninstagram.com", true},
		{"evil.com", false},
		{"evil-fbcdn.net", false},
		{"fbcdn.net.evil.com", false},
		{"", false},
		{"FBCDN.net", true}, // case-insensitive
	}
	for _, c := range cases {
		if got := isAllowedMediaHost(c.host); got != c.want {
			t.Errorf("isAllowedMediaHost(%q)=%v, want %v", c.host, got, c.want)
		}
	}
}

// --- VerifySignature edge cases ---

func TestVerifySignature_ExactLengthRequired(t *testing.T) {
	a := &WhatsAppAdapter{}
	body := []byte(`{"hello":"world"}`)
	secret := "topsecret"
	// Compute correct expected sig
	good := computeHMAC(secret, body)

	cases := []struct {
		name string
		sig  string
		want bool
	}{
		{"valid", "sha256=" + good, true},
		{"valid-uppercase-prefix", "SHA256=" + good, true},
		{"empty", "", false},
		{"prefix-only", "sha256=", false},
		{"wrong-hex", "sha256=" + strings.Repeat("0", 64), false},
		{"truncated", "sha256=" + good[:32], false},
		{"oversized", "sha256=" + good + "ff", false},
		{"no-prefix", good, false},
	}
	for _, c := range cases {
		got := a.VerifySignature(body, c.sig, secret)
		if got != c.want {
			t.Errorf("%s: expected %v got %v", c.name, c.want, got)
		}
	}
}

func computeHMAC(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// --- EntryTimestampUnix replay guard ---

func TestEntryTimestampUnix_OldestEntryWins(t *testing.T) {
	body := []byte(`{"object":"whatsapp_business_account","entry":[{"time":1700000000,"changes":[]},{"time":1699999000,"changes":[]}]}`)
	got := EntryTimestampUnix(body)
	if got != 1699999000 {
		t.Errorf("expected 1699999000 (oldest), got %d", got)
	}
}

func TestEntryTimestampUnix_ZeroOnMissing(t *testing.T) {
	body := []byte(`{"object":"whatsapp_business_account","entry":[]}`)
	if got := EntryTimestampUnix(body); got != 0 {
		t.Errorf("expected 0 on empty entry, got %d", got)
	}
}

func TestEntryTimestampUnix_ZeroOnInvalidJSON(t *testing.T) {
	if got := EntryTimestampUnix([]byte("not json")); got != 0 {
		t.Errorf("expected 0 on bad JSON, got %d", got)
	}
}

// --- ParseMetaError UserMsg-only fallback ---

func TestParseMetaError_UserMsgOnlyEnvelope(t *testing.T) {
	body := []byte(`{"error":{"code":131009,"error_user_msg":"Número inválido","fbtrace_id":"trace-X"}}`)
	err := ParseMetaError(400, body)
	if err == nil {
		t.Fatal("expected typed MetaError, got nil")
	}
	meta, ok := err.(*MetaError)
	if !ok {
		t.Fatalf("expected *MetaError, got %T (%v)", err, err)
	}
	if meta.Code != 131009 {
		t.Errorf("expected code 131009, got %d", meta.Code)
	}
	if meta.UserMsg != "Número inválido" {
		t.Errorf("expected UserMsg, got %q", meta.UserMsg)
	}
}

// Real-world payload from /register on a number being migrated from
// another BSP without payment method on the destination WABA. Meta
// hides the actionable cause inside error_data.details — the parser
// must capture it so the catalog can match on the substring.
func TestParseMetaError_DetailsField(t *testing.T) {
	body := []byte(`{"error":{"message":"(#100) Invalid parameter","type":"OAuthException","code":100,"error_data":{"messaging_product":"whatsapp","details":"Cannot Migrate Phone Number: Your WhatsApp Business Account does not have payment method set up."},"fbtrace_id":"AHG_9aoFOSs9pfXntr4fs3D"}}`)
	err := ParseMetaError(400, body)
	if err == nil {
		t.Fatal("expected typed MetaError, got nil")
	}
	meta, ok := err.(*MetaError)
	if !ok {
		t.Fatalf("expected *MetaError, got %T (%v)", err, err)
	}
	if meta.Code != 100 {
		t.Errorf("expected code 100, got %d", meta.Code)
	}
	if meta.Details == "" {
		t.Errorf("expected Details populated, got empty")
	}
	if meta.FBTraceID != "AHG_9aoFOSs9pfXntr4fs3D" {
		t.Errorf("expected fbtrace_id propagated, got %q", meta.FBTraceID)
	}
	// Error() must surface Details so log lines & errors.As callers see
	// the actionable hint instead of the generic top-level message.
	if msg := meta.Error(); !strings.Contains(msg, "payment method") {
		t.Errorf("expected Error() to surface Details, got %q", msg)
	}
}

func TestParseMetaError_GenericFallback(t *testing.T) {
	// Unparseable body should NOT crash and should return a non-nil
	// generic error.
	err := ParseMetaError(500, []byte("not json"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, isMeta := err.(*MetaError); isMeta {
		t.Errorf("expected non-typed fallback error, got *MetaError")
	}
}

// --- Helper that builds a multi-message batch payload. Mirrors the
// production-shape Meta uses for burst delivery. ---

func buildBatchPayload(msgs []map[string]interface{}) []byte {
	wireMsgs := make([]interface{}, 0, len(msgs))
	for _, m := range msgs {
		if _, ok := m["timestamp"]; !ok {
			m["timestamp"] = "1700000000"
		}
		wireMsgs = append(wireMsgs, m)
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
							"messages": wireMsgs,
						},
					},
				},
			},
		},
	}
	out, _ := json.Marshal(payload)
	return out
}

// A shared contact card, in the exact shape Meta actually sends.
//
// This is the payload that produced the bug: `Message.Contacts` carried the tag
// `contacts_payload`, so the array never decoded, the `contacts` branch fell
// through its `len() > 0` guard, and the message was saved as type `text` with
// an EMPTY body. On screen that is a small blank bubble — the customer had
// shared a parish's phone number and the operator saw nothing at all.
//
// Written against the wire shape rather than against ContactCard so a future
// rename of a Go field cannot make it pass while the wire mapping is broken
// again.
func TestParseInboundAll_SharedContactCardBecomesReadableText(t *testing.T) {
	body := buildBatchPayload([]map[string]interface{}{{
		"from": "5565920000001",
		"id":   "wamid.CONTACTS",
		"type": "contacts",
		"contacts": []map[string]interface{}{{
			"name": map[string]string{
				"first_name":     "Mercearia Boa",
				"middle_name":    "José",
				"last_name":      "Centro",
				"formatted_name": "Mercearia Boa Vista Centro",
			},
			"phones": []map[string]string{{
				"phone": "+55 11 9000-0001",
				"wa_id": "551190000001",
				"type":  "Celular",
			}},
			"vcard":  "QkVHSU46VkNBUkQ=",
			"origin": "other",
		}},
	}})

	all := ParseInboundAll(body)
	if len(all) != 1 {
		t.Fatalf("expected 1 parsed message, got %d", len(all))
	}
	got := all[0]

	// The bug's signature was an empty body. Assert that first and loudest.
	if strings.TrimSpace(got.Text) == "" {
		t.Fatal("a shared contact card produced an EMPTY body — it renders as a blank bubble")
	}
	if got.MessageType != "text" {
		t.Errorf("expected type text, got %q", got.MessageType)
	}
	if !strings.Contains(got.Text, "Mercearia Boa Vista Centro") {
		t.Errorf("the shared contact's name must reach the transcript, got %q", got.Text)
	}
	if !strings.Contains(got.Text, "9000-0001") {
		t.Errorf("the phone is the point of sharing a contact, got %q", got.Text)
	}
	// The structured payload survives for automations, not just the summary.
	if !strings.Contains(string(got.Metadata), "551190000001") {
		t.Errorf("expected the contact payload in metadata, got %s", got.Metadata)
	}
}

// The guard that keeps this class of failure visible: a contacts event whose
// array we cannot read must still say what it was, never fall through as an
// empty bubble.
func TestParseInboundAll_AnUnreadableContactCardStillSaysWhatItWas(t *testing.T) {
	body := buildBatchPayload([]map[string]interface{}{{
		"from":     "5565920000001",
		"id":       "wamid.EMPTYCONTACTS",
		"type":     "contacts",
		"contacts": []map[string]interface{}{},
	}})

	all := ParseInboundAll(body)
	if len(all) != 1 {
		t.Fatalf("expected 1 parsed message, got %d", len(all))
	}
	if strings.TrimSpace(all[0].Text) == "" {
		t.Fatal("an undescribable contact card must not become a blank bubble")
	}
}

// Meta refuses to deliver some message types (poll, live location, view-once).
// It announces them as `type: "unsupported"` with a 131051 error envelope whose
// TITLE is "Message type unknown" — and that title is what the parser used to
// match on, so the branch was dead and the message was dropped outright. The
// customer's phone showed it sent; nothing reached the inbox.
//
// This pins the wire shape verbatim from Meta's own payload example.
func TestParseInboundAll_AnUndeliverableTypeReachesTheInboxInsteadOfVanishing(t *testing.T) {
	body := buildBatchPayload([]map[string]interface{}{{
		"from": "5565920000002",
		"id":   "wamid.UNSUPPORTED",
		"type": "unsupported",
		"errors": []map[string]interface{}{{
			"code":    131051,
			"title":   "Message type unknown",
			"message": "Message type unknown",
			"error_data": map[string]string{
				"details": "Message type is currently not supported.",
			},
		}},
	}})

	all := ParseInboundAll(body)
	// Dropping it is the whole bug: assert existence before anything else.
	if len(all) != 1 {
		t.Fatalf("an undeliverable message must still reach the transcript, got %d parsed", len(all))
	}
	got := all[0]

	if strings.TrimSpace(got.Text) == "" {
		t.Fatal("an undeliverable message must say what happened, not render blank")
	}
	if got.MessageType != "text" {
		t.Errorf("expected type text so it renders as a bubble, got %q", got.MessageType)
	}
	// The AI cannot answer content Meta withheld; a confident reply to an
	// invisible message is worse than silence.
	if !got.Skip {
		t.Error("the agent must not run on a message whose content never arrived")
	}
	// Ops needs the raw type to tell a poll apart from a future Meta event.
	if !strings.Contains(string(got.Metadata), "unsupported") {
		t.Errorf("expected the raw type in metadata, got %s", got.Metadata)
	}
	if !strings.Contains(string(got.Metadata), "131051") {
		t.Errorf("expected Meta's error envelope in metadata, got %s", got.Metadata)
	}
}

// Meta has renamed these strings before, so the older alias stays matched.
func TestParseInboundAll_TheLegacyUnknownAliasIsStillHandled(t *testing.T) {
	body := buildBatchPayload([]map[string]interface{}{{
		"from": "5565920000003",
		"id":   "wamid.LEGACYUNKNOWN",
		"type": "unknown",
	}})

	all := ParseInboundAll(body)
	if len(all) != 1 {
		t.Fatalf("expected 1 parsed message, got %d", len(all))
	}
	if strings.TrimSpace(all[0].Text) == "" {
		t.Fatal("the legacy alias must not become a blank bubble")
	}
}

// The name and the phone do not say what a shared number IS. The business
// description does — "Loja pertencente a Rede Exemplo Fictício do
// Bairro - SP" was sitting in the vCard of a shared card,
// arrived intact, and was thrown away with the rest of the card.
func TestParseInboundAll_TheBusinessDescriptionSurvivesTheVCard(t *testing.T) {
	body := buildBatchPayload([]map[string]interface{}{{
		"from": "5565920000004",
		"id":   "wamid.VCARDDESC",
		"type": "contacts",
		"contacts": []map[string]interface{}{{
			"name": map[string]string{
				"formatted_name": "Mercearia Boa Vista Centro",
			},
			"phones": []map[string]string{{"phone": "+55 11 9000-0001"}},
			"vcard":  base64.StdEncoding.EncodeToString([]byte(realVCard)),
		}},
	}})

	all := ParseInboundAll(body)
	if len(all) != 1 {
		t.Fatalf("expected 1 parsed message, got %d", len(all))
	}
	got := all[0]

	if !strings.Contains(got.Text, "Rede Exemplo Fictício do Bairro") {
		t.Errorf("the description must reach the transcript, got %q", got.Text)
	}
	if !strings.Contains(string(got.Metadata), "business_description") {
		t.Errorf("expected the description as a structured field, got %s", got.Metadata)
	}
	// The raw card can carry a base64 PHOTO — tens of KB duplicated onto every
	// message row. We keep what we read out of it, not the card.
	if strings.Contains(string(got.Metadata), "BEGIN:VCARD") ||
		strings.Contains(string(got.Metadata), "\"vcard\"") {
		t.Errorf("the raw vcard must not be stored, got %s", got.Metadata)
	}
}

// A card with no description must lose nothing it had before.
func TestParseInboundAll_AContactWithoutADescriptionIsUnchanged(t *testing.T) {
	body := buildBatchPayload([]map[string]interface{}{{
		"from": "5565920000005",
		"id":   "wamid.NODESC",
		"type": "contacts",
		"contacts": []map[string]interface{}{{
			"name":   map[string]string{"formatted_name": "Maria Silva"},
			"phones": []map[string]string{{"phone": "+55 65 9111-2222"}},
		}},
	}})

	all := ParseInboundAll(body)
	if len(all) != 1 {
		t.Fatalf("expected 1 parsed message, got %d", len(all))
	}
	want := "📇 Contato compartilhado: Maria Silva (+55 65 9111-2222)"
	if all[0].Text != want {
		t.Errorf("got %q, want %q", all[0].Text, want)
	}
}

// The card's FN is whatever the SHARER had saved ("Gerente Paulo celular"); the
// business name is who the number actually belongs to. When they disagree the
// attendant needs both — the nickname is how the customer refers to it, the
// business name is what it is.
func TestParseInboundAll_ADifferentBusinessNameIsShownAlongsideTheSavedOne(t *testing.T) {
	card := "BEGIN:VCARD\nVERSION:3.0\nFN:Gerente Paulo celular\n" +
		"X-WA-BIZ-NAME:Rede Exemplo Fictício do Bairro\n" +
		"X-WA-BIZ-DESCRIPTION:Rede em Cidade Exemplo - SP\nEND:VCARD"

	body := buildBatchPayload([]map[string]interface{}{{
		"from": "5565920000006",
		"id":   "wamid.BIZNAME",
		"type": "contacts",
		"contacts": []map[string]interface{}{{
			"name":   map[string]string{"formatted_name": "Gerente Paulo celular"},
			"phones": []map[string]string{{"phone": "+55 65 9333-4444"}},
			"vcard":  base64.StdEncoding.EncodeToString([]byte(card)),
		}},
	}})

	all := ParseInboundAll(body)
	if len(all) != 1 {
		t.Fatalf("expected 1 parsed message, got %d", len(all))
	}
	got := all[0]

	if !strings.Contains(got.Text, "Gerente Paulo celular") {
		t.Errorf("the saved name must stay — it is how the customer refers to it, got %q", got.Text)
	}
	if !strings.Contains(got.Text, "Rede Exemplo Fictício do Bairro") {
		t.Errorf("the business name must reach the transcript, got %q", got.Text)
	}
	if !strings.Contains(string(got.Metadata), "business_name") {
		t.Errorf("expected the business name as a structured field, got %s", got.Metadata)
	}
}

// The common case: both names are the same string. Printing it twice would
// double every contact line for no information.
func TestParseInboundAll_AnIdenticalBusinessNameIsNotPrintedTwice(t *testing.T) {
	body := buildBatchPayload([]map[string]interface{}{{
		"from": "5565920000007",
		"id":   "wamid.SAMEBIZNAME",
		"type": "contacts",
		"contacts": []map[string]interface{}{{
			"name":   map[string]string{"formatted_name": "Mercearia Boa Vista Centro"},
			"phones": []map[string]string{{"phone": "+55 11 9000-0001"}},
			"vcard":  base64.StdEncoding.EncodeToString([]byte(realVCard)),
		}},
	}})

	all := ParseInboundAll(body)
	if len(all) != 1 {
		t.Fatalf("expected 1 parsed message, got %d", len(all))
	}
	if n := strings.Count(all[0].Text, "Mercearia Boa Vista Centro"); n != 1 {
		t.Errorf("the name appears %d times, want 1: %q", n, all[0].Text)
	}
}
