package domain

import (
	"context"
	"strings"
	"time"
)

// Electronic signature (§6.11, G7).
//
// The provider is BYO — the customer's own account, their cost. We do not build
// signing: legal validity under MP 2.200-2 is a product in itself, with audit
// trails, timestamping and liability that belong to whoever sells it.
//
// What we build is the part nobody else joins up: the document goes out, the
// status lands on the card, and a SIGNATURE BECOMES A SALE — which is what makes
// it reach the portfolio, the repurchase engine and somebody's commission.

// SignatureProvider names the service. One today, behind a port, because the
// choice is a pricing decision the customer may revisit.
type SignatureProvider string

const SignatureZapSign SignatureProvider = "zapsign"

// SignatureStatus is the lifecycle.
type SignatureStatus string

const (
	// SignatureDraft exists locally and has not been sent.
	SignatureDraft SignatureStatus = "draft"
	SignatureSent  SignatureStatus = "sent"
	// SignatureViewed is the one that changes a follow-up call: they opened it
	// and did not sign.
	SignatureViewed  SignatureStatus = "viewed"
	SignatureSigned  SignatureStatus = "signed"
	SignatureRefused SignatureStatus = "refused"
	SignatureExpired SignatureStatus = "expired"
	// SignatureFailed is OUR side failing — a bad token, a provider outage. Kept
	// apart from "refused", which is the customer saying no.
	SignatureFailed SignatureStatus = "failed"
)

// Terminal reports a status that will not change on its own.
func (s SignatureStatus) Terminal() bool {
	switch s {
	case SignatureSigned, SignatureRefused, SignatureExpired:
		return true
	default:
		return false
	}
}

// SignatureSigner is one person on the document.
type SignatureSigner struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	// Phone is E.164 when present. The provider may notify by WhatsApp with it.
	Phone string `json:"phone,omitempty"`

	// ProviderToken identifies the signer remotely.
	ProviderToken string `json:"provider_token,omitempty"`
	// SignURL is the individual signing link. Handed to a person deliberately,
	// never posted to a customer by a machine: it IS the authority to sign.
	SignURL string `json:"sign_url,omitempty"`

	Status   string     `json:"status,omitempty"`
	SignedAt *time.Time `json:"signed_at,omitempty"`
}

// SignatureRequest is one document out for signature.
type SignatureRequest struct {
	ID        string `json:"id"`
	AccountID string `json:"-"`

	ContractID string `json:"contract_id,omitempty"`
	LeadID     string `json:"lead_id,omitempty"`
	// ContactID is who this is with. Required for the signature to become a
	// Sale: a sale with no customer feeds no portfolio.
	ContactID string `json:"contact_id,omitempty"`

	Provider SignatureProvider `json:"provider"`
	// ProviderDocToken is the document's id at the provider.
	ProviderDocToken string `json:"provider_doc_token,omitempty"`

	Title string `json:"title"`

	Status        SignatureStatus `json:"status"`
	FailureReason string          `json:"failure_reason,omitempty"`

	Signers []SignatureSigner `json:"signers,omitempty"`

	ObjectPath       string `json:"object_path,omitempty"`
	SignedObjectPath string `json:"signed_object_path,omitempty"`

	// ValueCents is what the signature is worth. Zero means track it and
	// register NO sale — a signed document with no value is not a sale, and
	// inventing one puts a number in somebody's commission.
	ValueCents int64  `json:"value_cents"`
	Currency   string `json:"currency"`

	// SaleID is the Sale this produced. Its presence is the idempotency guard.
	SaleID string `json:"sale_id,omitempty"`

	SentAt    *time.Time `json:"sent_at,omitempty"`
	ViewedAt  *time.Time `json:"viewed_at,omitempty"`
	SignedAt  *time.Time `json:"signed_at,omitempty"`
	RefusedAt *time.Time `json:"refused_at,omitempty"`

	CreatedByUserID string    `json:"created_by_user_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Signed reports a completed signature.
func (r *SignatureRequest) Signed() bool {
	return r != nil && r.Status == SignatureSigned
}

// BecomesSale reports a signature that should produce a Sale.
//
// Three conditions, all necessary: it was signed, it is worth something, and we
// know who signed. Any one missing and the honest answer is to archive the
// document without touching anybody's commission.
func (r *SignatureRequest) BecomesSale() bool {
	return r.Signed() && r.ValueCents > 0 &&
		strings.TrimSpace(r.ContactID) != "" &&
		strings.TrimSpace(r.SaleID) == ""
}

// Validate refuses a request nobody can have meant.
func (r *SignatureRequest) Validate() error {
	if r == nil {
		return &AppError{Code: "SIGNATURE_INVALID", Message: "pedido vazio"}
	}
	if strings.TrimSpace(r.Title) == "" {
		return &AppError{Code: "SIGNATURE_INVALID", Message: "dê um nome ao documento"}
	}
	if len(r.Signers) == 0 {
		return &AppError{Code: "SIGNATURE_INVALID", Message: "informe quem vai assinar"}
	}
	for _, s := range r.Signers {
		if strings.TrimSpace(s.Name) == "" {
			return &AppError{Code: "SIGNATURE_INVALID", Message: "signatário sem nome"}
		}
		// A signer with neither email nor phone cannot be notified, and a
		// document nobody is told about is a document nobody signs.
		if strings.TrimSpace(s.Email) == "" && strings.TrimSpace(s.Phone) == "" {
			return &AppError{
				Code:    "SIGNATURE_INVALID",
				Message: "cada signatário precisa de e-mail ou telefone para receber o link",
			}
		}
	}
	if r.ValueCents < 0 {
		return &AppError{Code: "SIGNATURE_INVALID", Message: "valor não pode ser negativo"}
	}
	return nil
}

// SignatureSettings is the customer's own provider account.
type SignatureSettings struct {
	AccountID string            `json:"-"`
	Provider  SignatureProvider `json:"provider"`

	// APIToken is decrypted on read and never serialised: it is the customer's
	// credential at their provider, and a JSON payload is a log line waiting to
	// happen.
	APIToken string `json:"-"`
	// HasToken tells a screen whether the integration is configured without
	// handing the token back.
	HasToken bool `json:"has_token"`

	// WebhookToken is the unguessable part of OUR inbound URL.
	WebhookToken string `json:"webhook_token,omitempty"`
	Sandbox      bool   `json:"sandbox"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Configured reports an account that can actually send a document.
func (s *SignatureSettings) Configured() bool {
	return s != nil && strings.TrimSpace(s.APIToken) != ""
}

// ErrSignatureNotConfigured is an account with no provider token yet. A 503-shaped
// answer: the integration is missing, not the request wrong.
var ErrSignatureNotConfigured = &AppError{
	Code:    "SIGNATURE_NOT_CONFIGURED",
	Message: "configure a conta de assinatura eletrônica em Configurações",
}

// ErrSignatureUnauthorized is the customer's token being rejected by their own
// provider. Distinct from a generic failure because the fix is theirs: re-paste
// the token. "A assinatura falhou" would send them looking in the wrong place.
var ErrSignatureUnauthorized = &AppError{
	Code:    "SIGNATURE_UNAUTHORIZED",
	Message: "o provedor recusou o token desta conta — confira a chave em Configurações",
}

// ── Ports ──────────────────────────────────────────────────────────────────

// SignatureDocument is what goes out for signature.
type SignatureDocument struct {
	Title string
	// PDF bytes. The provider takes base64 or a URL; bytes keep our private
	// bucket private — a signed URL handed to a third party is a copy of the
	// document living in somebody else's logs.
	PDF     []byte
	Signers []SignatureSigner
	// ExternalID is OUR request id, echoed back on every webhook. The only
	// reliable way to match a remote document to a local row.
	ExternalID string
	Sandbox    bool
	// DeadlineOn is the last day to sign, when the deal has one.
	DeadlineOn *time.Time
}

// SignatureResult is the provider's answer.
type SignatureResult struct {
	ProviderDocToken string
	Status           SignatureStatus
	Signers          []SignatureSigner
	// SignedFileURL is set once the document is complete, and is what we
	// archive from.
	SignedFileURL string
}

// SignatureGateway is the provider port.
//
// Three methods and no more: send, ask, fetch the signed file. Everything else a
// signature provider offers (folders, templates, branding) is theirs to own —
// wrapping it would make us responsible for a product we deliberately do not
// build.
type SignatureGateway interface {
	Name() SignatureProvider
	// Create sends the document out for signature.
	Create(ctx context.Context, apiToken string, doc SignatureDocument) (*SignatureResult, error)
	// Get is the AUTHORITATIVE status read.
	//
	// The webhook is not trusted to move money: the provider publishes no HMAC
	// and no shared secret, so anybody who learns the callback URL could claim a
	// document was signed. Every status change that matters is confirmed here,
	// against the provider's own API, with the customer's own token.
	Get(ctx context.Context, apiToken, docToken string) (*SignatureResult, error)
	// DownloadSigned fetches the signed PDF for archiving.
	DownloadSigned(ctx context.Context, url string, maxBytes int64) ([]byte, error)
}

// SignatureRepository persists the requests.
type SignatureRepository interface {
	Create(ctx context.Context, r *SignatureRequest) error
	Get(ctx context.Context, accountID, id string) (*SignatureRequest, error)
	// GetByProviderToken resolves the inbound webhook to a local row.
	GetByProviderToken(ctx context.Context, accountID, docToken string) (*SignatureRequest, error)
	List(ctx context.Context, accountID string, limit, offset int) ([]*SignatureRequest, error)
	ListByContract(ctx context.Context, accountID, contractID string) ([]*SignatureRequest, error)
	Update(ctx context.Context, r *SignatureRequest) error
	// AttachSale records the Sale a signature produced, only when there is not
	// one already — the idempotency guard against a repeated webhook.
	AttachSale(ctx context.Context, accountID, id, saleID string) (bool, error)
}

// SignatureSettingsRepository persists the BYO credential.
type SignatureSettingsRepository interface {
	Get(ctx context.Context, accountID string) (*SignatureSettings, error)
	// ByWebhookToken routes an inbound callback. Returns nil when the token
	// matches nothing — the caller answers 404 and never says why.
	ByWebhookToken(ctx context.Context, token string) (*SignatureSettings, error)
	Upsert(ctx context.Context, s *SignatureSettings) error
}
