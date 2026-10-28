package storage

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	gcs "cloud.google.com/go/storage"
)

// validTestCredentialsBase64 builds a service-account JSON with a REAL (throwaway)
// RSA key, because gcs.NewClient parses the private key at construction — a
// placeholder string fails before the memoization under test is reached. No
// network is touched: the token source is created lazily.
func validTestCredentialsBase64() string {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		panic(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	sa := map[string]string{
		"type":         "service_account",
		"project_id":   "test",
		"private_key":  string(pemKey),
		"client_email": "test@test.iam.gserviceaccount.com",
		"token_uri":    "https://oauth2.googleapis.com/token",
	}
	raw, err := json.Marshal(sa)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// TestWithDecodedCredentials_ZeroesAfterUse locks in US-078: the decoded
// service-account key must be wiped from memory once the GCS client has parsed
// it, so a heap dump after init cannot recover GCS credentials. We capture the
// backing slice during fn and assert every byte is zero after the call returns.
func TestWithDecodedCredentials_ZeroesAfterUse(t *testing.T) {
	const fakeKey = `{"type":"service_account","private_key":"-----BEGIN PRIVATE KEY-----secret-----END PRIVATE KEY-----"}`
	a := NewGCSAdapter("bucket", base64.StdEncoding.EncodeToString([]byte(fakeKey)))

	var captured []byte
	_, _ = a.withDecodedCredentials(func(decoded []byte) (*gcs.Client, error) {
		if string(decoded) != fakeKey {
			t.Fatalf("decoded credentials mismatch: got %q", string(decoded))
		}
		captured = decoded // same backing array zeroBytes will wipe
		return nil, nil
	})

	if len(captured) == 0 {
		t.Fatal("expected non-empty decoded credentials buffer")
	}
	for i, b := range captured {
		if b != 0 {
			t.Fatalf("credential byte %d not zeroed post-init: %v", i, b)
		}
	}
}

// TestWithDecodedCredentials_EmptyConfig keeps the not-configured guard honest:
// no buffer is decoded (so nothing to zero) and the caller gets a clear error.
func TestWithDecodedCredentials_EmptyConfig(t *testing.T) {
	a := NewGCSAdapter("bucket", "")
	_, err := a.withDecodedCredentials(func(decoded []byte) (*gcs.Client, error) {
		t.Fatal("fn must not run when credentials are unconfigured")
		return nil, nil
	})
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected not-configured error, got %v", err)
	}
}

// TestValidateSignedURLTTL_Bounds locks in US-028: signed-URL expiry must
// sit within [5m, 24h]. Anything shorter breaks legitimate client flows;
// anything longer is a security exposure for a leaked URL.
func TestValidateSignedURLTTL_Bounds(t *testing.T) {
	cases := []struct {
		name    string
		ttl     time.Duration
		wantErr bool
	}{
		{"zero rejected", 0, true},
		{"below min rejected", 5*time.Minute - time.Second, true},
		{"exact min ok", 5 * time.Minute, false},
		{"mid-range ok", time.Hour, false},
		{"exact max ok", 24 * time.Hour, false},
		{"above max rejected", 24*time.Hour + time.Second, true},
		{"negative rejected", -time.Minute, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSignedURLTTL(tc.ttl)
			if tc.wantErr && err == nil {
				t.Fatalf("ttl %s: expected error, got nil", tc.ttl)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ttl %s: expected no error, got %v", tc.ttl, err)
			}
		})
	}
}

// TestGenerateReadSignedURL_RejectsOutOfRangeTTL proves the bound is wired
// into the public mint path — an out-of-range TTL fails fast with the range
// error before any GCS client work.
func TestGenerateReadSignedURL_RejectsOutOfRangeTTL(t *testing.T) {
	a := NewGCSAdapter("bucket", "")
	_, err := a.GenerateReadSignedURL(context.Background(), "some/object", 48*time.Hour)
	if err == nil || !strings.Contains(err.Error(), "out of allowed range") {
		t.Fatalf("expected out-of-range error, got %v", err)
	}
}

// TestGenerateSignedURL_RejectsOutOfRangeTTL covers the upload path too.
func TestGenerateSignedURL_RejectsOutOfRangeTTL(t *testing.T) {
	a := NewGCSAdapter("bucket", "")
	_, err := a.GenerateSignedURL(context.Background(), "obj", "image/png", time.Minute)
	if err == nil || !strings.Contains(err.Error(), "out of allowed range") {
		t.Fatalf("expected out-of-range error, got %v", err)
	}
}

// The health probe used to log `gcs bucket "..." unreachable: context
// deadline exceeded` and flip the endpoint to 503 while uploads were working
// fine. Every method built its own client, so every call — probe included —
// paid for a JWT signature and an OAuth token exchange inside a 4s budget.
func TestGCSAdapter_ClientIsBuiltOnceAndShared(t *testing.T) {
	a := NewGCSAdapter("bucket", validTestCredentialsBase64())

	first, err := a.newClient(context.Background())
	if err != nil {
		t.Fatalf("first build: %v", err)
	}
	second, err := a.newClient(context.Background())
	if err != nil {
		t.Fatalf("second build: %v", err)
	}
	if first != second {
		t.Fatal("a second call built a new client — every media call would pay for a token exchange")
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// A token exchange that failed once must not leave the adapter permanently
// broken: the next caller has to try again.
func TestGCSAdapter_AFailedBuildIsNotCached(t *testing.T) {
	a := NewGCSAdapter("bucket", "not-base64!!")

	if _, err := a.newClient(context.Background()); err == nil {
		t.Fatal("want an error for undecodable credentials")
	}
	a.clientMu.Lock()
	cached := a.client
	a.clientMu.Unlock()
	if cached != nil {
		t.Fatal("a failed build must leave nothing cached")
	}
	if _, err := a.newClient(context.Background()); err == nil {
		t.Fatal("the second attempt must still report the error, not a stale success")
	}
}

// Close on an adapter that never built a client is a no-op, so shutdown does
// not depend on whether any media call happened.
func TestGCSAdapter_CloseWithoutAClientIsANoOp(t *testing.T) {
	if err := NewGCSAdapter("bucket", "").Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
