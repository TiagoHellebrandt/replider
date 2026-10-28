package signature

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// The ZapSign adapter, against a fake provider.
//
// There is no live account to test with, so the contract under test is the one we
// read off the documentation: bearer auth, base64 PDF, per-signer sign_url. The
// house does the same for PNCP and the BCB series — a request shape pinned by a
// test beats a request shape pinned by hope.

func TestZapSign_CreateSendsTheDocumentAndReturnsTheSigningLinks(t *testing.T) {
	var gotAuth, gotPath, gotMethod string
	var gotBody zapCreateIn

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath, gotMethod = r.Header.Get("Authorization"), r.URL.Path, r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{
          "token":"doc-abc","status":"pending","external_id":"req-1",
          "signers":[
            {"token":"s1","name":"Dra. Ana","email":"ana@clinica.com",
             "status":"new","sign_url":"https://app.zapsign.com.br/verificar/s1"}
          ]
        }`))
	}))
	defer srv.Close()

	deadline := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	res, err := NewZapSign().WithBaseURL(srv.URL).Create(context.Background(), "tok-123",
		domain.SignatureDocument{
			Title:      "Contrato de fornecimento",
			PDF:        []byte("%PDF-1.7 contract"),
			ExternalID: "req-1",
			DeadlineOn: &deadline,
			Signers: []domain.SignatureSigner{
				{Name: "Dra. Ana", Email: "ana@clinica.com", Phone: "+5565999990000"},
			},
		})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if gotMethod != http.MethodPost || gotPath != "/docs/" {
		t.Fatalf("called %s %s, want POST /docs/", gotMethod, gotPath)
	}
	// The customer's own token, as a bearer. There is no platform account.
	if gotAuth != "Bearer tok-123" {
		t.Fatalf("auth = %q, want the account's own bearer token", gotAuth)
	}
	// Bytes, not a URL: a signed URL to our private bucket would put a copy of
	// the document in a third party's logs.
	decoded, decErr := base64.StdEncoding.DecodeString(gotBody.Base64PDF)
	if decErr != nil || string(decoded) != "%PDF-1.7 contract" {
		t.Fatalf("pdf not sent as base64 bytes: %v", decErr)
	}
	// external_id is the only reliable way to match a webhook back to our row.
	if gotBody.ExternalID != "req-1" {
		t.Fatalf("external_id = %q, want req-1", gotBody.ExternalID)
	}
	if gotBody.DateLimitToSign != "2026-09-30" {
		t.Fatalf("deadline = %q, want 2026-09-30", gotBody.DateLimitToSign)
	}

	// E.164 is split for the provider, which wants the parts apart — and the
	// WhatsApp notification is only switched on when there IS a number.
	signer := gotBody.Signers[0]
	if signer.PhoneCountry != "55" || signer.PhoneNumber != "65999990000" {
		t.Fatalf("phone split wrong: %+v", signer)
	}
	if !signer.SendAutomaticWhatsApp || !signer.SendAutomaticEmail {
		t.Fatalf("the provider must do the notifying: %+v", signer)
	}

	if res.ProviderDocToken != "doc-abc" || res.Status != domain.SignatureSent {
		t.Fatalf("result = %+v", res)
	}
	// The signing link is per signer and is what a person hands over.
	if len(res.Signers) != 1 || res.Signers[0].SignURL == "" {
		t.Fatalf("signers = %+v", res.Signers)
	}
}

func TestZapSign_CreateWithoutPhoneDoesNotClaimWhatsApp(t *testing.T) {
	var gotBody zapCreateIn
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"token":"doc-1","status":"pending"}`))
	}))
	defer srv.Close()

	if _, err := NewZapSign().WithBaseURL(srv.URL).Create(context.Background(), "tok",
		domain.SignatureDocument{
			Title: "Proposta", PDF: []byte("%PDF"),
			Signers: []domain.SignatureSigner{{Name: "Ana", Email: "ana@x.com"}},
		}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if gotBody.Signers[0].SendAutomaticWhatsApp {
		t.Fatal("no phone means no WhatsApp notification to promise")
	}
	if gotBody.Signers[0].PhoneCountry != "" {
		t.Fatalf("no phone must send no country code: %+v", gotBody.Signers[0])
	}
}

func TestZapSign_GetIsTheAuthoritativeRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/docs/doc-abc/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
          "token":"doc-abc","status":"signed",
          "signed_file":"https://cdn.zapsign.com.br/signed/doc-abc.pdf",
          "signers":[{"token":"s1","name":"Ana","status":"signed",
                      "signed_at":"2026-08-01T12:00:00Z"}]
        }`))
	}))
	defer srv.Close()

	res, err := NewZapSign().WithBaseURL(srv.URL).
		Get(context.Background(), "tok", "doc-abc")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if res.Status != domain.SignatureSigned {
		t.Fatalf("status = %q, want signed", res.Status)
	}
	if res.SignedFileURL == "" {
		t.Fatal("a signed document must carry the file to archive")
	}
	if len(res.Signers) != 1 || res.Signers[0].SignedAt == nil {
		t.Fatalf("the signature timestamp must survive: %+v", res.Signers)
	}
}

// A rejected token is the customer's to fix, and the error has to say so — "a
// assinatura falhou" would send them looking in the wrong place.
func TestZapSign_RejectedTokenIsItsOwnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"Invalid token."}`))
	}))
	defer srv.Close()

	_, err := NewZapSign().WithBaseURL(srv.URL).Create(context.Background(), "bad",
		domain.SignatureDocument{
			Title: "x", PDF: []byte("%PDF"),
			Signers: []domain.SignatureSigner{{Name: "Ana", Email: "a@b.com"}},
		})
	if !errors.Is(err, domain.ErrSignatureUnauthorized) {
		t.Fatalf("err = %v, want ErrSignatureUnauthorized", err)
	}

	// And no token at all never reaches the network.
	if _, err := NewZapSign().Get(context.Background(), "", "doc"); !errors.Is(err, domain.ErrSignatureNotConfigured) {
		t.Fatalf("err = %v, want ErrSignatureNotConfigured", err)
	}
}

func TestZapSign_UpstreamErrorCarriesTheReason(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"signers":["This field is required."]}`))
	}))
	defer srv.Close()

	_, err := NewZapSign().WithBaseURL(srv.URL).Create(context.Background(), "tok",
		domain.SignatureDocument{
			Title: "x", PDF: []byte("%PDF"),
			Signers: []domain.SignatureSigner{{Name: "Ana", Email: "a@b.com"}},
		})
	if err == nil {
		t.Fatal("a 400 must surface")
	}
	// The provider's own words, so somebody can act on them instead of guessing.
	if !strings.Contains(err.Error(), "This field is required") {
		t.Fatalf("err = %v, want the provider's message", err)
	}
}

func TestZapSign_DownloadSignedIsBoundedAndUnauthenticated(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("%PDF-1.7 signed copy"))
	}))
	defer srv.Close()

	data, err := NewZapSign().DownloadSigned(context.Background(), srv.URL+"/signed.pdf", 1024)
	if err != nil {
		t.Fatalf("DownloadSigned: %v", err)
	}
	if string(data) != "%PDF-1.7 signed copy" {
		t.Fatalf("got %q", data)
	}
	// The URL is pre-signed by the provider: sending the customer's API token to
	// whatever CDN it points at would leak a credential.
	if gotAuth != "" {
		t.Fatalf("no credential may travel to the file host, got %q", gotAuth)
	}

	if _, err := NewZapSign().DownloadSigned(context.Background(), "", 1024); err == nil {
		t.Fatal("no signed file yet must be an error, not empty bytes")
	}
}

func TestMapStatusAndEvents(t *testing.T) {
	for raw, want := range map[string]domain.SignatureStatus{
		"signed":   domain.SignatureSigned,
		"refused":  domain.SignatureRefused,
		"rejected": domain.SignatureRefused,
		"expired":  domain.SignatureExpired,
		"viewed":   domain.SignatureViewed,
		"pending":  domain.SignatureSent,
		"new":      domain.SignatureSent,
		// An unknown status must NOT read as failure: the provider may add one,
		// and a red card on a healthy document is worse than a stale one.
		"whatever_new_status": domain.SignatureSent,
	} {
		if got := MapStatus(raw); got != want {
			t.Errorf("MapStatus(%q) = %q, want %q", raw, got, want)
		}
	}

	for event, want := range map[string]domain.SignatureStatus{
		"doc_signed":  domain.SignatureSigned,
		"doc_refused": domain.SignatureRefused,
		"doc_expired": domain.SignatureExpired,
		"doc_viewed":  domain.SignatureViewed,
		// Events we do not act on map to nothing rather than inventing a
		// transition.
		"doc_created":                 "",
		"email_bounce":                "",
		"signature_notification_sent": "",
	} {
		if got := MapEventStatus(event); got != want {
			t.Errorf("MapEventStatus(%q) = %q, want %q", event, got, want)
		}
	}
}
