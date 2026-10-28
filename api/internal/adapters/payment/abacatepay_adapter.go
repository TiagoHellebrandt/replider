package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// abacatePayBaseURL is the v2 API root. Values are always in BRL cents.
const abacatePayBaseURL = "https://api.abacatepay.com/v2"

// abacatePublicHMACKey is the key AbacatePay documents publicly for the
// X-Webhook-Signature HMAC.
//
// Read that again: the key is PUBLIC, printed in their docs. The signature
// therefore proves the body was not altered in transit — it proves NOTHING
// about who sent it, because anyone can compute it. The actual
// authentication is the ?webhookSecret= query parameter, which is the
// per-merchant secret. Both are checked, in that order of importance.
const abacatePublicHMACKey = "t9dXRhHHo3yDEj5pVDYz0frf7q6bMKyMRmxxCPIPp3RCplBfXRxqlC6ZpiWmOqj4L63qEaeUOtrCI8P0VMUgo6iIga2ri9ogaHFs0WIIywSMg0q7RmBfybe1E5XJcfC4IW3alNqym0tXoAKkzvfEjZxV6bE0oG2zJrNNYmUCKZyV0KZ3JS8Votf9EAWWYdiDkMkpbMdPggfh1EqHlVkMiTady6jOR3hyzGEHrIz2Ret0xHKMbiqkr9HS1JhNHDX9"

// abacateWebhookTolerance bounds how old a signed webhook timestamp may
// be before it is rejected as a replay.
const abacateWebhookTolerance = 5 * time.Minute

// AbacatePayAdapter talks to AbacatePay's v2 REST API (Brazilian PIX +
// card gateway). It implements domain.AbacatePayAdapter. A zero API key
// disables it (Enabled reports false) so the transparent-checkout use
// case can fall back to Stripe.
type AbacatePayAdapter struct {
	apiKey        string
	webhookSecret string
	// hmacKey overrides the documented public HMAC key (tests, or the day
	// AbacatePay rotates it without a release on our side).
	hmacKey string
	baseURL string // settable so tests can point at an httptest server
	http    *http.Client
	// now is injectable for deterministic webhook-timestamp tests.
	now func() time.Time
}

// NewAbacatePayAdapter builds the adapter. An empty apiKey yields a
// disabled adapter (every call returns ErrAbacatePayDisabled).
func NewAbacatePayAdapter(apiKey, webhookSecret string) *AbacatePayAdapter {
	return &AbacatePayAdapter{
		apiKey:        apiKey,
		webhookSecret: webhookSecret,
		baseURL:       abacatePayBaseURL,
		http:          &http.Client{Timeout: 30 * time.Second},
		now:           time.Now,
	}
}

// WithBaseURL repoints the adapter at a test server. Returns the adapter
// for chaining.
func (a *AbacatePayAdapter) WithBaseURL(u string) *AbacatePayAdapter {
	a.baseURL = strings.TrimRight(u, "/")
	return a
}

var (
	// ErrAbacatePayDisabled is returned when no API key is configured.
	ErrAbacatePayDisabled = &domain.AppError{Code: "ABACATEPAY_DISABLED", Message: "AbacatePay is not configured"}
	// ErrAbacateBadSignature marks a webhook whose HMAC did not verify.
	ErrAbacateBadSignature = &domain.AppError{Code: "ABACATEPAY_BAD_SIGNATURE", Message: "invalid AbacatePay webhook signature"}
)

func (a *AbacatePayAdapter) Enabled() bool { return a != nil && a.apiKey != "" }

// envelope is AbacatePay's uniform response wrapper: {data, error, success}.
type abacateEnvelope struct {
	Data    json.RawMessage `json:"data"`
	Error   *string         `json:"error"`
	Success json.RawMessage `json:"success"`
}

// do issues a JSON request to the v2 API, unwraps the envelope, and
// decodes `data` into out. A non-2xx status or a non-null `error` field
// becomes an error.
func (a *AbacatePayAdapter) do(ctx context.Context, method, path string, body any, out any) error {
	if !a.Enabled() {
		return ErrAbacatePayDisabled
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("abacatepay: marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("abacatepay: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("abacatepay: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("abacatepay: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("abacatepay: %s %s returned %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var env abacateEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("abacatepay: decode envelope: %w", err)
	}
	if env.Error != nil && *env.Error != "" {
		return fmt.Errorf("abacatepay: api error: %s", *env.Error)
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("abacatepay: decode data: %w", err)
		}
	}
	return nil
}

// ── PIX (transparent) ──────────────────────────────────────────────────

func (a *AbacatePayAdapter) CreatePixCharge(ctx context.Context, in domain.PixChargeInput) (*domain.PixCharge, error) {
	data := map[string]any{
		"amount": in.AmountCents,
	}
	if in.Description != "" {
		data["description"] = in.Description
	}
	if in.ExternalID != "" {
		data["externalId"] = in.ExternalID
	}
	if len(in.Metadata) > 0 {
		data["metadata"] = in.Metadata
	}
	if in.Customer != nil {
		data["customer"] = in.Customer
	}

	req := map[string]any{"method": "PIX", "data": data}

	var out struct {
		ID           string `json:"id"`
		Amount       int64  `json:"amount"`
		Status       string `json:"status"`
		BRCode       string `json:"brCode"`
		BRCodeBase64 string `json:"brCodeBase64"`
		ExpiresAt    string `json:"expiresAt"`
	}
	if err := a.do(ctx, http.MethodPost, "/transparents/create", req, &out); err != nil {
		return nil, err
	}
	return &domain.PixCharge{
		ID:           out.ID,
		BRCode:       out.BRCode,
		BRCodeBase64: out.BRCodeBase64,
		AmountCents:  out.Amount,
		Status:       out.Status,
		ExpiresAt:    out.ExpiresAt,
	}, nil
}

func (a *AbacatePayAdapter) GetChargeStatus(ctx context.Context, chargeID string) (string, error) {
	var out struct {
		Status string `json:"status"`
	}
	if err := a.do(ctx, http.MethodGet, "/transparents/check?id="+chargeID, nil, &out); err != nil {
		return "", err
	}
	return out.Status, nil
}

// ── Card (hosted checkout) ─────────────────────────────────────────────

// CreateCardCheckout provisions an ad-hoc product priced at the plan
// amount, then opens a card-only hosted checkout for it. AbacatePay
// checkouts bill a Product, so a product is required per checkout; the
// externalId keeps product creation idempotent per (plan, amount).
func (a *AbacatePayAdapter) CreateCardCheckout(ctx context.Context, in domain.CardCheckoutInput) (string, error) {
	productExternalID := "plan_" + strconv.FormatInt(in.AmountCents, 10) + "_" + sanitizeExternalID(in.PlanName)

	var product struct {
		ID string `json:"id"`
	}
	createProduct := map[string]any{
		"externalId": productExternalID,
		"name":       in.PlanName,
		"price":      in.AmountCents,
	}
	if err := a.do(ctx, http.MethodPost, "/products/create", createProduct, &product); err != nil {
		return "", err
	}
	if product.ID == "" {
		return "", fmt.Errorf("abacatepay: product create returned no id")
	}

	checkout := map[string]any{
		"items":         []map[string]any{{"id": product.ID, "quantity": 1}},
		"methods":       []string{"CARD"},
		"returnUrl":     in.CancelURL,
		"completionUrl": in.SuccessURL,
	}
	if in.ExternalID != "" {
		checkout["externalId"] = in.ExternalID
	}
	if len(in.Metadata) > 0 {
		checkout["metadata"] = in.Metadata
	}

	var out struct {
		URL string `json:"url"`
	}
	if err := a.do(ctx, http.MethodPost, "/checkouts/create", checkout, &out); err != nil {
		return "", err
	}
	if out.URL == "" {
		return "", fmt.Errorf("abacatepay: checkout create returned no url")
	}
	return out.URL, nil
}

// ── Webhook verification ───────────────────────────────────────────────

// VerifySignature checks the `X-Webhook-Signature` header: base64 of
// HMAC-SHA256 over the RAW BODY, keyed with AbacatePay's documented public
// key. Constant-time comparison.
//
// This is an INTEGRITY check only. The key is public (see
// abacatePublicHMACKey), so a valid signature says the bytes arrived
// unaltered — not that AbacatePay sent them. Authentication is
// VerifyWebhookSecret; a caller that checks only this one has an open
// endpoint.
//
// Corrected against the published scheme (US-M014). The first pass guessed
// a Stripe-style `t=<ts>,v1=<hex>` header over `<ts>.<body>` with the
// merchant secret — wrong header, wrong signed bytes, wrong encoding, wrong
// key. It would have rejected every real delivery.
func (a *AbacatePayAdapter) VerifySignature(payload []byte, signatureHeader string) bool {
	if signatureHeader == "" {
		return false
	}
	key := a.hmacKey
	if key == "" {
		key = abacatePublicHMACKey
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(payload)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signatureHeader)))
}

// VerifyWebhookSecret compares the `?webhookSecret=` query value against
// the configured secret in constant time. This is what authenticates the
// delivery.
//
// An unconfigured secret returns false rather than allowing everything:
// the endpoint activates paid plans, so "not configured" must mean closed.
func (a *AbacatePayAdapter) VerifyWebhookSecret(provided string) bool {
	if a.webhookSecret == "" || provided == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a.webhookSecret), []byte(provided)) == 1
}

// ParseWebhook decodes a verified body into the event.
func (a *AbacatePayAdapter) ParseWebhook(payload []byte) (*domain.AbacateWebhookEvent, error) {
	var event domain.AbacateWebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("abacatepay: decode webhook body: %w", err)
	}
	return &event, nil
}

// sanitizeExternalID keeps an externalId to a safe slug.
func sanitizeExternalID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "plan"
	}
	return b.String()
}
