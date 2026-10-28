package whatsapp

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGetPhoneNumberStatus_CloudAPI(t *testing.T) {
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/pn-1") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if !strings.Contains(r.URL.RawQuery, "platform_type") ||
			!strings.Contains(r.URL.RawQuery, "webhook_configuration") {
			t.Errorf("expected platform_type + webhook_configuration in fields: %s", r.URL.RawQuery)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("missing auth header: %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{
		  "id": "pn-1",
		  "display_phone_number": "+55 11 99999-9999",
		  "verified_name": "Acme",
		  "code_verification_status": "VERIFIED",
		  "platform_type": "CLOUD_API",
		  "webhook_configuration": {"application": "https://api.replider.com/v1/webhooks/whatsapp"}
		}`))
	}))

	st, err := GetPhoneNumberStatus(context.Background(), "pn-1", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.PlatformType != "CLOUD_API" {
		t.Errorf("PlatformType=%q", st.PlatformType)
	}
	if st.WebhookApplication != "https://api.replider.com/v1/webhooks/whatsapp" {
		t.Errorf("WebhookApplication=%q", st.WebhookApplication)
	}
	if st.CodeVerificationStatus != "VERIFIED" {
		t.Errorf("CodeVerificationStatus=%q", st.CodeVerificationStatus)
	}
}

func TestGetPhoneNumberStatus_NotApplicable(t *testing.T) {
	// Mirrors the production symptom: number exists but is still
	// owned by another Cloud API app, so platform_type stays
	// NOT_APPLICABLE and the diagnostics endpoint must surface
	// needs_register = true.
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
		  "id":"pn-x",
		  "display_phone_number":"+55 11 9000-0002",
		  "verified_name":"Catholize",
		  "code_verification_status":"VERIFIED",
		  "platform_type":"NOT_APPLICABLE",
		  "webhook_configuration":{"application":"https://api.replider.com/v1/webhooks/whatsapp"}
		}`))
	}))
	st, err := GetPhoneNumberStatus(context.Background(), "pn-x", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.PlatformType != "NOT_APPLICABLE" {
		t.Fatalf("expected NOT_APPLICABLE, got %q", st.PlatformType)
	}
}

func TestGetPhoneNumberStatus_MetaError(t *testing.T) {
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid token","type":"OAuthException","code":190}}`))
	}))
	_, err := GetPhoneNumberStatus(context.Background(), "pn-x", "bad")
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if !strings.Contains(err.Error(), "Invalid token") {
		t.Errorf("expected Graph error message in err, got %v", err)
	}
}

func TestRegisterPhoneNumber_Success(t *testing.T) {
	srv := withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/pn-1/register") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("missing auth: %q", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"pin":"654321"`) {
			t.Errorf("expected pin in body, got %s", body)
		}
		if !strings.Contains(string(body), `"messaging_product":"whatsapp"`) {
			t.Errorf("expected messaging_product=whatsapp, got %s", body)
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	_ = srv
	if err := RegisterPhoneNumber(context.Background(), "pn-1", "tok", "654321"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestRegisterPhoneNumber_InvalidPIN(t *testing.T) {
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Two-step verification pin mismatch","code":133005}}`))
	}))
	err := RegisterPhoneNumber(context.Background(), "pn-1", "tok", "000000")
	if err == nil {
		t.Fatal("expected error on wrong PIN")
	}
	if !strings.Contains(err.Error(), "133005") && !strings.Contains(err.Error(), "pin mismatch") {
		t.Errorf("expected PIN-mismatch signal in error, got %v", err)
	}
}

func TestDeregisterPhoneNumber_Success(t *testing.T) {
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/pn-old/deregister") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	if err := DeregisterPhoneNumber(context.Background(), "pn-old", "old-token"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestDeregisterPhoneNumber_Error(t *testing.T) {
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"Not permitted","code":200}}`))
	}))
	err := DeregisterPhoneNumber(context.Background(), "pn-x", "tok")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRequestVerificationCode_SMS(t *testing.T) {
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/pn-1/request_code") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		s := string(body)
		if !strings.Contains(s, "code_method=SMS") {
			t.Errorf("expected code_method=SMS, got %s", s)
		}
		if !strings.Contains(s, "language=pt_BR") {
			t.Errorf("expected default language pt_BR, got %s", s)
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	if err := RequestVerificationCode(context.Background(), "pn-1", "tok", "SMS", ""); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestRequestVerificationCode_MetaError(t *testing.T) {
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Number already verified","code":136025}}`))
	}))
	err := RequestVerificationCode(context.Background(), "pn-1", "tok", "VOICE", "en_US")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVerifyCode_Success(t *testing.T) {
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "code=123456") {
			t.Errorf("expected code in body, got %s", body)
		}
		if !strings.HasSuffix(r.URL.Path, "/verify_code") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	if err := VerifyCode(context.Background(), "pn-1", "tok", "123456"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestVerifyCode_WrongCode(t *testing.T) {
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid code","code":136020}}`))
	}))
	err := VerifyCode(context.Background(), "pn-1", "tok", "000000")
	if err == nil {
		t.Fatal("expected error on wrong code")
	}
}
