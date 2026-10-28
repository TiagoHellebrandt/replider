package whatsapp

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

// defaultTemplateLanguage is the language code assumed when a template
// send omits one. pt_BR is the platform's primary market.
const defaultTemplateLanguage = "pt_BR"

// ErrTemplateMissingOptOut is the anti-ban guard error: a MARKETING /
// approach (abordagem) template was built without an opt-out quick-reply
// button (e.g. "Parar de receber"). Sending unsolicited marketing
// templates without a documented opt-out is the fastest way to get a
// WhatsApp number flagged and banned, so we reject the build BEFORE any
// network call. Callers assert with errors.Is.
var ErrTemplateMissingOptOut = errors.New("whatsapp: marketing/approach template requires an opt-out quick-reply button (anti-ban protection)")

// templateButton is a button as declared on the approved template
// definition. Text is the fixed caption the recipient sees (used for
// opt-out detection); Payload is only set for dynamic quick-reply
// buttons that carry a per-send payload parameter.
type templateButton struct {
	SubType string // quick_reply | url | phone_number
	Text    string
	Payload string
}

// templateSpec is the resolved description of a template send: the
// approved template's identity plus the per-recipient variable values
// (filled from lead/contact data by the caller). It is the single input
// to buildTemplatePayload.
type templateSpec struct {
	Name     string
	Language string
	Category string // MARKETING | UTILITY | AUTHENTICATION

	// BodyParams are the positional {{1}}..{{n}} BODY values, already
	// resolved from lead/contact data (see fillTemplateParams).
	BodyParams []string

	// Buttons is the template's declared button set, used both to bind
	// dynamic quick-reply payloads and to enforce the opt-out guard.
	Buttons []templateButton

	// Components, when non-empty, is a pre-built Cloud API components
	// array that overrides BodyParams/Buttons assembly. The opt-out
	// guard still runs off Buttons, so callers using this override must
	// still populate Buttons for marketing templates.
	Components []interface{}
}

// optOutPhrases are the substrings (lowercased) that mark a quick-reply
// button as an opt-out control. Kept deliberately broad — a false
// positive here only relaxes the guard for a button the operator clearly
// meant as an opt-out, while a miss would wrongly block a compliant send.
var optOutPhrases = []string{
	"parar", "sair", "cancelar", "descadastr", "remover",
	"não receber", "nao receber", "deixar de receber", "não quero", "nao quero",
	"stop", "unsubscribe", "opt out", "opt-out", "optout",
}

// IsOptOutText exposes the opt-out caption check to other layers — the
// MCP create-template tool enforces at creation time the same
// MARKETING rule the send guard enforces at send time.
func IsOptOutText(text string) bool { return isOptOutText(text) }

// isOptOutText reports whether a button caption reads as an opt-out
// control in PT-BR or EN.
func isOptOutText(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return false
	}
	for _, p := range optOutPhrases {
		if strings.Contains(t, p) {
			return true
		}
	}
	return false
}

// templateRequiresOptOut reports whether the category demands an opt-out
// button. MARKETING covers Meta's outbound approach/abordagem templates;
// UTILITY and AUTHENTICATION are transactional and exempt.
func templateRequiresOptOut(category string) bool {
	return strings.EqualFold(strings.TrimSpace(category), "MARKETING")
}

// hasOptOutButton reports whether the spec declares at least one
// opt-out quick-reply button.
func (s templateSpec) hasOptOutButton() bool {
	for _, b := range s.Buttons {
		if strings.EqualFold(b.SubType, "quick_reply") && isOptOutText(b.Text) {
			return true
		}
	}
	return false
}

// buildComponents assembles the Cloud API `components` array from the
// resolved BODY params and any dynamic button payloads.
func (s templateSpec) buildComponents() []interface{} {
	var comps []interface{}
	if len(s.BodyParams) > 0 {
		params := make([]map[string]interface{}, 0, len(s.BodyParams))
		for _, p := range s.BodyParams {
			params = append(params, map[string]interface{}{"type": "text", "text": p})
		}
		comps = append(comps, map[string]interface{}{
			"type":       "body",
			"parameters": params,
		})
	}
	for i, b := range s.Buttons {
		// Only quick-reply buttons carrying a dynamic payload need a
		// per-send button component; static opt-out buttons do not.
		if strings.EqualFold(b.SubType, "quick_reply") && b.Payload != "" {
			comps = append(comps, map[string]interface{}{
				"type":     "button",
				"sub_type": "quick_reply",
				"index":    strconv.Itoa(i),
				"parameters": []map[string]interface{}{
					{"type": "payload", "payload": b.Payload},
				},
			})
		}
	}
	return comps
}

// buildTemplatePayload assembles the Cloud API `template` object from a
// resolved spec, enforcing the anti-ban opt-out guard. Returns
// ErrTemplateMissingOptOut for a marketing/approach template that lacks
// an opt-out quick-reply button — before any network call.
func buildTemplatePayload(spec templateSpec) (map[string]interface{}, error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return nil, fmt.Errorf("whatsapp: template name is required")
	}
	if templateRequiresOptOut(spec.Category) && !spec.hasOptOutButton() {
		return nil, ErrTemplateMissingOptOut
	}

	lang := strings.TrimSpace(spec.Language)
	if lang == "" {
		lang = defaultTemplateLanguage
	}

	components := spec.Components
	if len(components) == 0 {
		components = spec.buildComponents()
	}

	tmpl := map[string]interface{}{
		"name":     name,
		"language": map[string]string{"code": lang},
	}
	if len(components) > 0 {
		tmpl["components"] = components
	}
	return tmpl, nil
}

// fillTemplateParams resolves an ordered list of lead/contact field keys
// into positional BODY parameter values from a data map. Missing keys
// resolve to "" so the positional ordering Meta expects is preserved.
func fillTemplateParams(variables []string, data map[string]string) []string {
	out := make([]string, 0, len(variables))
	for _, key := range variables {
		out = append(out, data[key])
	}
	return out
}

// parseTemplateSpecFromMeta reads a templateSpec out of a domain.Message
// metadata map (the shape the outbound send path carries for a
// Type=="template" message). Recognized keys:
//
//	template_name        string   (required)
//	template_language    string   (default pt_BR)
//	template_category    string   (MARKETING|UTILITY|AUTHENTICATION)
//	template_body_params []string positional, pre-resolved BODY values
//	template_variables   []string ordered lead/contact field keys
//	template_data        object   lead/contact field -> value
//	template_buttons     []object {sub_type,text,payload}
//
// When template_body_params is absent, BODY values are resolved from
// template_variables + template_data via fillTemplateParams.
func parseTemplateSpecFromMeta(meta map[string]interface{}) templateSpec {
	spec := templateSpec{
		Name:     metaString(meta, "template_name"),
		Language: metaString(meta, "template_language"),
		Category: metaString(meta, "template_category"),
	}

	if params := metaStringSlice(meta, "template_body_params"); len(params) > 0 {
		spec.BodyParams = params
	} else if vars := metaStringSlice(meta, "template_variables"); len(vars) > 0 {
		spec.BodyParams = fillTemplateParams(vars, metaStringMap(meta, "template_data"))
	}

	spec.Buttons = parseTemplateButtons(meta["template_buttons"])
	return spec
}

// metaStringSlice reads a []string out of a metadata value that may be a
// []string, []interface{} of strings, or absent.
func metaStringSlice(m map[string]interface{}, key string) []string {
	if m == nil {
		return nil
	}
	switch v := m[key].(type) {
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// metaStringMap reads a map[string]string out of a metadata value that
// may be a map[string]string or map[string]interface{}.
func metaStringMap(m map[string]interface{}, key string) map[string]string {
	if m == nil {
		return nil
	}
	switch v := m[key].(type) {
	case map[string]string:
		return v
	case map[string]interface{}:
		out := make(map[string]string, len(v))
		for k, item := range v {
			if s, ok := item.(string); ok {
				out[k] = s
			}
		}
		return out
	}
	return nil
}

// parseTemplateButtons decodes the template_buttons metadata value into
// []templateButton. Accepts a slice of maps (the JSON shape).
func parseTemplateButtons(v interface{}) []templateButton {
	items, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]templateButton, 0, len(items))
	for _, item := range items {
		bm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		btn := templateButton{
			SubType: stringFromAny(bm["sub_type"]),
			Text:    stringFromAny(bm["text"]),
			Payload: stringFromAny(bm["payload"]),
		}
		out = append(out, btn)
	}
	return out
}

func stringFromAny(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// logMetaSendError logs a failed Cloud API send with the Meta error code
// and Kind so on-call can triage from logs alone. Quality / opt-out
// throttles (131049 marketing-quality limit, 131050 user opted out) are
// expected back-pressure, not faults, so they log at WARN; everything
// else is an ERROR.
func logMetaSendError(channelID, phoneNumberID, msgType string, err error, respBody []byte) {
	var me *MetaError
	if errors.As(err, &me) {
		attrs := []interface{}{
			"status", me.HTTPStatus,
			"code", me.Code,
			"kind", me.Kind(),
			"channel_id", channelID,
			"phone_number_id", phoneNumberID,
			"type", msgType,
			"body", string(respBody),
		}
		if me.Code == 131049 || me.Code == 131050 {
			slog.Warn("WhatsApp send blocked by quality/opt-out limit", attrs...)
			return
		}
		slog.Error("WhatsApp API error", attrs...)
		return
	}
	slog.Error("WhatsApp API error",
		"channel_id", channelID,
		"phone_number_id", phoneNumberID,
		"type", msgType,
		"body", string(respBody))
}
