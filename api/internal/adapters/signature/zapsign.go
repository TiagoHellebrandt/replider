// Package signature talks to the electronic-signature provider the customer
// brings.
package signature

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// ZapSign implements domain.SignatureGateway.
//
// Chosen for a plain REST API with bearer auth and per-signer signing links, which
// is the smallest surface that does the job. It sits behind the port because the
// choice is a pricing decision the customer may revisit — swapping it must not
// reach past this file.
//
// The token is ALWAYS the customer's own (BYO). There is no platform-level
// ZapSign account and no env var for one: the signature quota, the invoice and
// the legal trail belong to them.
type ZapSign struct {
	client  *http.Client
	baseURL string
}

func NewZapSign() *ZapSign {
	return &ZapSign{
		// Generous: the create call uploads a PDF, and a scanned contract over a
		// slow link is not an error to time out at ten seconds.
		client:  &http.Client{Timeout: 60 * time.Second},
		baseURL: "https://api.zapsign.com.br/api/v1",
	}
}

// WithBaseURL points the adapter at a test server. The only way to exercise this
// without a live account: the house does the same for PNCP and the BCB series.
func (z *ZapSign) WithBaseURL(u string) *ZapSign {
	z.baseURL = strings.TrimRight(u, "/")
	return z
}

func (z *ZapSign) Name() domain.SignatureProvider { return domain.SignatureZapSign }

// zapSignerIn is one signer on the way out.
type zapSignerIn struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	// PhoneCountry and PhoneNumber are separate at the provider. We hold E.164,
	// so the split happens here rather than polluting the domain with a
	// provider's field layout.
	PhoneCountry string `json:"phone_country,omitempty"`
	PhoneNumber  string `json:"phone_number,omitempty"`
	// SendAutomaticEmail/WhatsApp let the PROVIDER do the notifying. That is the
	// point of using one: it owns delivery, retries and the legal record of
	// having notified.
	SendAutomaticEmail    bool `json:"send_automatic_email"`
	SendAutomaticWhatsApp bool `json:"send_automatic_whatsapp"`
}

type zapCreateIn struct {
	Name       string        `json:"name"`
	Base64PDF  string        `json:"base64_pdf"`
	Signers    []zapSignerIn `json:"signers"`
	Lang       string        `json:"lang"`
	ExternalID string        `json:"external_id,omitempty"`
	Sandbox    bool          `json:"sandbox,omitempty"`
	// DateLimitToSign is yyyy-MM-dd at the provider.
	DateLimitToSign string `json:"date_limit_to_sign,omitempty"`
}

type zapSignerOut struct {
	Token    string `json:"token"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Status   string `json:"status"`
	SignURL  string `json:"sign_url"`
	SignedAt string `json:"signed_at"`
}

type zapDocOut struct {
	Token      string         `json:"token"`
	Status     string         `json:"status"`
	ExternalID string         `json:"external_id"`
	SignedFile string         `json:"signed_file"`
	Signers    []zapSignerOut `json:"signers"`
}

// Create sends the document out for signature.
func (z *ZapSign) Create(
	ctx context.Context, apiToken string, doc domain.SignatureDocument,
) (*domain.SignatureResult, error) {
	if strings.TrimSpace(apiToken) == "" {
		return nil, domain.ErrSignatureNotConfigured
	}
	if len(doc.PDF) == 0 {
		return nil, fmt.Errorf("zapsign: empty document")
	}

	in := zapCreateIn{
		Name: doc.Title,
		// Bytes, not a URL: handing a provider a signed URL to our private bucket
		// puts a copy of the document in a third party's logs.
		Base64PDF:  base64.StdEncoding.EncodeToString(doc.PDF),
		Lang:       "pt-br",
		ExternalID: doc.ExternalID,
		Sandbox:    doc.Sandbox,
	}
	if doc.DeadlineOn != nil {
		in.DateLimitToSign = doc.DeadlineOn.Format("2006-01-02")
	}
	for _, s := range doc.Signers {
		signer := zapSignerIn{
			Name:               s.Name,
			Email:              strings.TrimSpace(s.Email),
			SendAutomaticEmail: strings.TrimSpace(s.Email) != "",
		}
		if country, number, ok := splitE164(s.Phone); ok {
			signer.PhoneCountry, signer.PhoneNumber = country, number
			signer.SendAutomaticWhatsApp = true
		}
		in.Signers = append(in.Signers, signer)
	}

	var out zapDocOut
	if err := z.do(ctx, http.MethodPost, "/docs/", apiToken, in, &out); err != nil {
		return nil, err
	}
	return toResult(out), nil
}

// Get is the authoritative status read.
func (z *ZapSign) Get(ctx context.Context, apiToken, docToken string) (*domain.SignatureResult, error) {
	if strings.TrimSpace(apiToken) == "" {
		return nil, domain.ErrSignatureNotConfigured
	}
	if strings.TrimSpace(docToken) == "" {
		return nil, fmt.Errorf("zapsign: empty document token")
	}
	var out zapDocOut
	if err := z.do(ctx, http.MethodGet, "/docs/"+docToken+"/", apiToken, nil, &out); err != nil {
		return nil, err
	}
	return toResult(out), nil
}

// DownloadSigned fetches the signed PDF.
//
// Plain GET with no auth header: the provider hands back a pre-signed URL, and
// sending our customer's API token to whatever host that URL points at would leak
// a credential to a CDN.
func (z *ZapSign) DownloadSigned(ctx context.Context, url string, maxBytes int64) ([]byte, error) {
	if strings.TrimSpace(url) == "" {
		return nil, fmt.Errorf("zapsign: no signed file yet")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := z.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zapsign: signed file returned %d", resp.StatusCode)
	}
	if maxBytes <= 0 {
		maxBytes = 20 << 20
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBytes))
}

func (z *ZapSign) do(ctx context.Context, method, path, apiToken string, in, out any) error {
	var body io.Reader
	if in != nil {
		encoded, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, z.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := z.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		// The customer's token is wrong or revoked. A distinct error because the
		// fix is theirs — re-paste the token in Settings — and telling them
		// "signature failed" would send them to the wrong place.
		return domain.ErrSignatureUnauthorized
	case resp.StatusCode >= 400:
		return fmt.Errorf("zapsign: %s %s returned %d: %s",
			method, path, resp.StatusCode, snippet(payload))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(payload, out)
}

func toResult(out zapDocOut) *domain.SignatureResult {
	res := &domain.SignatureResult{
		ProviderDocToken: out.Token,
		Status:           MapStatus(out.Status),
		SignedFileURL:    out.SignedFile,
	}
	for _, s := range out.Signers {
		signer := domain.SignatureSigner{
			Name: s.Name, Email: s.Email,
			ProviderToken: s.Token, SignURL: s.SignURL, Status: s.Status,
		}
		if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(s.SignedAt)); err == nil {
			signer.SignedAt = &parsed
		}
		res.Signers = append(res.Signers, signer)
	}
	return res
}

// MapStatus translates the provider's vocabulary.
//
// Exported because the webhook handler reads the same strings from a payload it
// does NOT trust — one mapping, so the two paths cannot disagree about what
// "signed" means.
func MapStatus(raw string) domain.SignatureStatus {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "signed":
		return domain.SignatureSigned
	case "refused", "rejected":
		return domain.SignatureRefused
	case "expired":
		return domain.SignatureExpired
	case "viewed":
		return domain.SignatureViewed
	case "pending", "new", "sent":
		return domain.SignatureSent
	default:
		// An unknown status is NOT treated as failure: the provider may add one,
		// and turning "we don't recognise this" into "this went wrong" would show
		// a red card for a document that is fine. It stays as sent until the
		// authoritative read says otherwise.
		return domain.SignatureSent
	}
}

// MapEventStatus translates a webhook event_type.
//
// Only the events that change our state are mapped; the rest return empty and
// the caller ignores them rather than inventing a transition.
func MapEventStatus(eventType string) domain.SignatureStatus {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "doc_signed":
		return domain.SignatureSigned
	case "doc_refused":
		return domain.SignatureRefused
	case "doc_expired":
		return domain.SignatureExpired
	case "doc_viewed":
		return domain.SignatureViewed
	default:
		return ""
	}
}

// splitE164 breaks +5565999990000 into the country code and the rest.
//
// Brazil-first because that is who this is sold to, with a generic fallback: the
// provider wants the two apart and the domain keeps one E.164 string.
func splitE164(phone string) (country, number string, ok bool) {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)
	if len(digits) < 10 {
		return "", "", false
	}
	if strings.HasPrefix(digits, "55") && len(digits) >= 12 {
		return "55", digits[2:], true
	}
	// Unknown country: hand the whole thing over as the number rather than
	// guessing a code. A wrong country code is a message that never arrives.
	return "", digits, true
}

func snippet(b []byte) string {
	const max = 300
	s := strings.TrimSpace(string(b))
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

var _ domain.SignatureGateway = (*ZapSign)(nil)
