package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// PrevalidateComponents catches the most common Meta 2388299 triggers
// before the outbound call. Meta's validator returns the same "Invalid
// parameter" for dozens of content issues; we short-circuit the
// known-bad ones so the user gets an actionable message. Returns "" when
// the components pass.
func PrevalidateComponents(components []map[string]interface{}) string {
	for _, c := range components {
		t, _ := c["type"].(string)
		if t != "BODY" {
			continue
		}
		text, _ := c["text"].(string)
		// Double-asterisk markdown: WhatsApp uses *single* asterisks
		// for bold. The double form ships as literal characters, and
		// Meta's v25 validator rejects templates containing it.
		if strings.Contains(text, "**") {
			return "remova os ** do texto — WhatsApp usa *texto* (um asterisco só) para negrito"
		}
		// Leading/trailing variables: Meta rejects a BODY whose text
		// starts or ends with {{N}} ("Leading or Trailing Params Not
		// Allowed").
		trimmed := strings.TrimSpace(text)
		if leadingVarRegex.MatchString(trimmed) {
			return "texto não pode começar com variável — adicione uma palavra antes de {{1}}"
		}
		if trailingVarRegex.MatchString(trimmed) {
			return "texto não pode terminar com variável — adicione uma palavra ou pontuação depois da última {{N}}"
		}
	}
	return ""
}

var (
	leadingVarRegex  = regexp.MustCompile(`^\s*\{\{\d+\}\}`)
	trailingVarRegex = regexp.MustCompile(`\{\{\d+\}\}\s*$`)
)

// TemplateNameRegex enforces Meta's allowed template name format.
var TemplateNameRegex = regexp.MustCompile(`^[a-z0-9_]{1,512}$`)

// TemplatePayload is the Meta Graph API body for creating a message template.
type TemplatePayload struct {
	Name                string                   `json:"name"`
	Category            string                   `json:"category"` // MARKETING | UTILITY | AUTHENTICATION
	Language            string                   `json:"language"` // e.g. pt_BR, en_US
	Components          []map[string]interface{} `json:"components"`
	AllowCategoryChange bool                     `json:"allow_category_change"` // let Meta auto-correct category instead of rejecting
}

// CreateTemplate submits a new message template to Meta for approval.
func CreateTemplate(ctx context.Context, wabaID, accessToken string, payload TemplatePayload) error {
	apiURL := fmt.Sprintf(metaGraphBaseURL+"/%s/%s/message_templates", metaAPIVersion, wabaID)

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return ParseMetaError(resp.StatusCode, respBody)
	}
	return nil
}

// DeleteTemplate removes a template (or all templates with that name across languages).
func DeleteTemplate(ctx context.Context, wabaID, accessToken, name string) error {
	apiURL := fmt.Sprintf(metaGraphBaseURL+"/%s/%s/message_templates?name=%s", metaAPIVersion, wabaID, url.QueryEscape(name))

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := metaHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return ParseMetaError(resp.StatusCode, respBody)
	}
	return nil
}
