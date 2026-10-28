// Package resend is the platform's bulk email provider (hybrid sending,
// 2026-08-25). One-by-one approvals always leave through the customer's own
// mailbox; only BATCH approvals come here, and only after the customer's
// domain is registered and verified with Resend — so the per-message cost
// exists only for accounts that asked for volume.
package resend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

const defaultBaseURL = "https://api.resend.com"

// Client is a minimal REST client for the four calls the hybrid model needs:
// register a domain, read it, ask for verification, send a mail. The official
// SDK is not used — four endpoints do not justify a dependency, and a settable
// baseURL is what lets httptest stand in for the real service.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// SetBaseURL re-points the client (httptest). Test seam only.
func (c *Client) SetBaseURL(u string) { c.baseURL = strings.TrimRight(u, "/") }

// Configured answers whether a key was provided. The feature fails closed
// without one — the caller maps false to BULK_SENDING_UNAVAILABLE.
func (c *Client) Configured() bool { return c != nil && c.apiKey != "" }

var _ domain.BulkDomainProvider = (*Client)(nil)

// CreateDomain registers the domain, or reuses the existing registration when
// Resend answers that the name is already taken by this account (enabling
// volume twice must not error). Returns the domain with its DNS records.
func (c *Client) CreateDomain(ctx context.Context, name string) (*domain.BulkDomain, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	var out domain.BulkDomain
	err := c.do(ctx, http.MethodPost, "/domains", map[string]string{"name": name}, &out)
	if err == nil {
		return &out, nil
	}
	// 409 = already registered. The create response carries no id on that
	// path, so find it in the listing — same account, same name.
	if appErr, ok := err.(*domain.AppError); ok && appErr.Code == bulkConflictCode {
		return c.findDomainByName(ctx, name)
	}
	return nil, err
}

func (c *Client) findDomainByName(ctx context.Context, name string) (*domain.BulkDomain, error) {
	var listing struct {
		Data []domain.BulkDomain `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/domains", nil, &listing); err != nil {
		return nil, err
	}
	for i := range listing.Data {
		if strings.EqualFold(listing.Data[i].Name, name) {
			// The listing omits records; fetch the full object.
			return c.GetDomain(ctx, listing.Data[i].ID)
		}
	}
	return nil, &domain.AppError{Code: "BULK_DOMAIN_NOT_FOUND",
		Message: "bulk provider reports the domain registered but it is not listed"}
}

func (c *Client) GetDomain(ctx context.Context, id string) (*domain.BulkDomain, error) {
	var out domain.BulkDomain
	if err := c.do(ctx, http.MethodGet, "/domains/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// VerifyDomain asks Resend to re-check the DNS records. The result lands
// asynchronously on the domain's status — poll GetDomain after.
func (c *Client) VerifyDomain(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/domains/"+id+"/verify", nil, nil)
}

// SendEmail delivers one message and returns Resend's email id.
func (c *Client) SendEmail(ctx context.Context, from, replyTo, to, subject, text string) (string, error) {
	body := map[string]any{
		"from":    from,
		"to":      []string{to},
		"subject": subject,
		"text":    text,
	}
	if replyTo != "" {
		body["reply_to"] = replyTo
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, http.MethodPost, "/emails", body, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// bulkConflictCode marks Resend's 409 (domain already registered), which
// CreateDomain treats as reuse, not failure.
const bulkConflictCode = "BULK_PROVIDER_CONFLICT"

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	if !c.Configured() {
		return domain.ErrBulkSendingUnavailable
	}
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, payload)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out == nil || len(data) == 0 {
			return nil
		}
		return json.Unmarshal(data, out)
	}

	// Resend errors: {"statusCode":..,"name":"...","message":"..."}
	var apiErr struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(data, &apiErr)
	msg := apiErr.Message
	if msg == "" {
		msg = strings.TrimSpace(string(data))
	}
	if resp.StatusCode == http.StatusConflict {
		return &domain.AppError{Code: bulkConflictCode, Message: msg}
	}
	return &domain.AppError{Code: "BULK_PROVIDER_ERROR",
		Message: fmt.Sprintf("bulk provider refused the call (%d): %s", resp.StatusCode, msg)}
}
