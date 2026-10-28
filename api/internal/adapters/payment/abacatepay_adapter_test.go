package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func domain_PixInput(amount int64) domain.PixChargeInput {
	return domain.PixChargeInput{AmountCents: amount, Description: "test", Metadata: map[string]string{"account_id": "acc_1", "plan_id": "plan_1"}}
}

func domain_CardInput(amount int64) domain.CardCheckoutInput {
	return domain.CardCheckoutInput{PlanName: "Pro", AmountCents: amount, ExternalID: "acc_1_plan_1", SuccessURL: "https://app.replider.com/ok", CancelURL: "https://app.replider.com/no"}
}

func TestAbacate_Disabled(t *testing.T) {
	a := NewAbacatePayAdapter("", "")
	assert.False(t, a.Enabled())
	_, err := a.CreatePixCharge(context.Background(), domain_PixInput(1000))
	assert.ErrorIs(t, err, ErrAbacatePayDisabled)
}

func TestAbacate_CreatePixCharge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/transparents/create", r.URL.Path)
		assert.Equal(t, "Bearer key_123", r.Header.Get("Authorization"))
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"id":"pix_1","amount":9900,"status":"PENDING","brCode":"000201","brCodeBase64":"data:image/png;base64,AAA","expiresAt":"2026-07-14T12:00:00Z"},"error":null,"success":{"message":"ok"}}`)
	}))
	defer srv.Close()

	a := NewAbacatePayAdapter("key_123", "wh").WithBaseURL(srv.URL)
	charge, err := a.CreatePixCharge(context.Background(), domain_PixInput(9900))
	require.NoError(t, err)
	assert.Equal(t, "pix_1", charge.ID)
	assert.Equal(t, int64(9900), charge.AmountCents)
	assert.Equal(t, "000201", charge.BRCode)
	assert.Contains(t, charge.BRCodeBase64, "base64")
}

func TestAbacate_CreateCardCheckout(t *testing.T) {
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/products/create":
			fmt.Fprint(w, `{"data":{"id":"prod_1"},"error":null,"success":{"message":"ok"}}`)
		case "/checkouts/create":
			fmt.Fprint(w, `{"data":{"id":"bill_1","url":"https://app.abacatepay.com/pay/bill_1","status":"PENDING"},"error":null,"success":{"message":"ok"}}`)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	a := NewAbacatePayAdapter("key_123", "wh").WithBaseURL(srv.URL)
	url, err := a.CreateCardCheckout(context.Background(), domain_CardInput(9900))
	require.NoError(t, err)
	assert.Equal(t, "https://app.abacatepay.com/pay/bill_1", url)
	assert.Equal(t, []string{"/products/create", "/checkouts/create"}, hits)
}

func TestAbacate_GetChargeStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/transparents/check", r.URL.Path)
		assert.Equal(t, "pix_1", r.URL.Query().Get("id"))
		fmt.Fprint(w, `{"data":{"id":"pix_1","status":"PAID"},"error":null,"success":{"message":"ok"}}`)
	}))
	defer srv.Close()

	a := NewAbacatePayAdapter("key_123", "wh").WithBaseURL(srv.URL)
	status, err := a.GetChargeStatus(context.Background(), "pix_1")
	require.NoError(t, err)
	assert.Equal(t, "PAID", status)
}

func TestAbacate_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":null,"error":"amount too low","success":null}`)
	}))
	defer srv.Close()

	a := NewAbacatePayAdapter("key_123", "wh").WithBaseURL(srv.URL)
	_, err := a.CreatePixCharge(context.Background(), domain_PixInput(1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "amount too low")
}

// US-M014. The published scheme, not the Stripe-shaped guess this
// replaced: base64 HMAC-SHA256 over the RAW body, and the key AbacatePay
// prints in its own docs. That key being public is the whole point of the
// second gate — see VerifyWebhookSecret.
func TestAbacate_VerifySignature(t *testing.T) {
	const key = "test_public_key"
	body := []byte(`{"id":"log_1","event":"billing.paid","data":{"id":"pix_1","status":"PAID"}}`)

	sign := func(payload []byte) string {
		mac := hmac.New(sha256.New, []byte(key))
		mac.Write(payload)
		return base64.StdEncoding.EncodeToString(mac.Sum(nil))
	}
	newAdapter := func() *AbacatePayAdapter {
		a := NewAbacatePayAdapter("key_123", "whsec_test")
		a.hmacKey = key
		return a
	}

	t.Run("valid", func(t *testing.T) {
		assert.True(t, newAdapter().VerifySignature(body, sign(body)))
	})

	t.Run("tampered body", func(t *testing.T) {
		tampered := []byte(`{"id":"log_1","event":"billing.paid","data":{"metadata":{"account_id":"attacker"}}}`)
		assert.False(t, newAdapter().VerifySignature(tampered, sign(body)))
	})

	t.Run("missing header", func(t *testing.T) {
		assert.False(t, newAdapter().VerifySignature(body, ""))
	})

	// The documented key is the default, so a deployment that configures
	// nothing still validates real deliveries.
	t.Run("falls back to the documented public key", func(t *testing.T) {
		a := NewAbacatePayAdapter("key_123", "whsec_test")
		mac := hmac.New(sha256.New, []byte(abacatePublicHMACKey))
		mac.Write(body)
		assert.True(t, a.VerifySignature(body, base64.StdEncoding.EncodeToString(mac.Sum(nil))))
	})
}

// The query secret is what AUTHENTICATES a delivery — the signature key is
// public, so without this anyone could activate a paid plan.
func TestAbacate_VerifyWebhookSecret(t *testing.T) {
	a := NewAbacatePayAdapter("key_123", "whsec_test")

	assert.True(t, a.VerifyWebhookSecret("whsec_test"))
	assert.False(t, a.VerifyWebhookSecret("whsec_wrong"))
	assert.False(t, a.VerifyWebhookSecret(""), "an empty secret must never pass")

	// Unconfigured means CLOSED: this endpoint activates paid plans, so a
	// missing secret cannot mean "allow everything".
	unconfigured := NewAbacatePayAdapter("key_123", "")
	assert.False(t, unconfigured.VerifyWebhookSecret(""))
	assert.False(t, unconfigured.VerifyWebhookSecret("anything"))
}
