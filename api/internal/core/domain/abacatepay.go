package domain

import "context"

// AbacatePay is the Brazilian PIX + card payment gateway used for BRL
// accounts (cheaper %, accepts Elo — which Stripe BR does not). It is a
// SEPARATE, narrow port from the Stripe-shaped PaymentAdapter so the two
// providers never leak into each other's use cases: the transparent
// checkout routes BRL→AbacatePay and non-BRL→Stripe.
//
// PIX is truly transparent — CreatePixCharge returns a copy-paste code +
// QR image the app renders inline, and confirmation arrives via webhook
// (billing.paid) or the GetChargeStatus poll. Card is NOT embeddable via
// AbacatePay's public v2 API (only PIX/boleto are), so the card path is a
// hosted checkout URL the app opens (AbacatePay's own single-page
// transparent checkout, Elo + installments).
type AbacatePayAdapter interface {
	// Enabled reports whether an API key is configured. When false the
	// transparent-checkout use case falls back to Stripe even for BRL.
	Enabled() bool

	// CreatePixCharge mints a transparent PIX charge for [amountCents]
	// (BRL). [externalID] is our idempotency key and carries the
	// account/plan so the webhook can resolve what was paid.
	CreatePixCharge(ctx context.Context, in PixChargeInput) (*PixCharge, error)

	// CreateCardCheckout creates a hosted card checkout for a plan and
	// returns the redirect URL. AbacatePay checkouts bill a Product, so
	// the adapter provisions an ad-hoc product priced at the plan amount.
	CreateCardCheckout(ctx context.Context, in CardCheckoutInput) (string, error)

	// GetChargeStatus polls a transparent charge's status
	// ("PENDING"|"PAID"|"EXPIRED"). Used by the app to detect a PIX
	// payment without waiting on the webhook.
	GetChargeStatus(ctx context.Context, chargeID string) (string, error)

	// VerifySignature checks the `X-Webhook-Signature` header (base64
	// HMAC-SHA256 over the raw body, AbacatePay's documented PUBLIC key).
	// Integrity only — the key is public, so this proves the bytes were
	// not altered, never who sent them.
	VerifySignature(payload []byte, signatureHeader string) bool

	// VerifyWebhookSecret compares the `?webhookSecret=` query value
	// against the configured per-merchant secret. THIS is what
	// authenticates a delivery.
	VerifyWebhookSecret(provided string) bool

	// ParseWebhook decodes an already-verified body.
	ParseWebhook(payload []byte) (*AbacateWebhookEvent, error)
}

// PixChargeInput is the request to mint a transparent PIX charge.
type PixChargeInput struct {
	AmountCents int64
	Description string
	// ExternalID is our idempotency key (e.g. "acc:<id>:plan:<id>:<ts>").
	ExternalID string
	// Metadata is echoed back on the webhook so activation can resolve
	// the account + plan without a lookup table.
	Metadata map[string]string
	Customer *AbacateCustomer
}

// PixCharge is a minted transparent PIX charge.
type PixCharge struct {
	ID string `json:"id"`
	// BRCode is the PIX copy-paste string (copia-e-cola).
	BRCode string `json:"br_code"`
	// BRCodeBase64 is the QR code PNG as a data: URI.
	BRCodeBase64 string `json:"br_code_base64"`
	AmountCents  int64  `json:"amount"`
	Status       string `json:"status"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

// CardCheckoutInput is the request to create a hosted card checkout.
type CardCheckoutInput struct {
	PlanName    string
	AmountCents int64
	ExternalID  string
	Metadata    map[string]string
	SuccessURL  string
	CancelURL   string
	Customer    *AbacateCustomer
}

// AbacateCustomer is the optional payer identity.
type AbacateCustomer struct {
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	TaxID    string `json:"taxId,omitempty"`
	Cellular string `json:"cellphone,omitempty"`
}

// AbacateWebhookEvent is the decoded, signature-verified webhook body.
// AbacatePay fires `billing.paid` on a completed PIX/card charge; the
// metadata we set at charge creation rides back on Charge.Metadata.
type AbacateWebhookEvent struct {
	// ID is the delivery's own id ("log_abc123xyz"), used as the
	// idempotency key: AbacatePay retries, and activating a plan twice
	// would reset the customer's usage twice.
	ID    string        `json:"id"`
	Event string        `json:"event"`
	Data  AbacateCharge `json:"data"`
}

// AbacateCharge is the charge payload carried on a webhook event.
type AbacateCharge struct {
	ID         string            `json:"id"`
	Status     string            `json:"status"`
	ExternalID string            `json:"externalId"`
	Metadata   map[string]string `json:"metadata"`
}
