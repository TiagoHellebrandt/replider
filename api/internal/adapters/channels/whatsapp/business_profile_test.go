package whatsapp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// withMetaServer swaps metaGraphBaseURL for the duration of the test.
// Restores the original value even if the test panics.
func withMetaServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	orig := metaGraphBaseURL
	metaGraphBaseURL = srv.URL
	t.Cleanup(func() {
		metaGraphBaseURL = orig
		srv.Close()
	})
	return srv
}

func TestGetBusinessProfile_Success(t *testing.T) {
	srv := withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/pn-123/whatsapp_business_profile") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer token-abc" {
			t.Errorf("missing or wrong Authorization header: %q", r.Header.Get("Authorization"))
		}
		// Must request the expected fields
		if !strings.Contains(r.URL.RawQuery, "fields=about") {
			t.Errorf("missing fields query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"about":"Nossa loja","email":"e@e.com","vertical":"RETAIL","websites":["https://x"]}]}`))
	}))
	_ = srv

	p, err := GetBusinessProfile(context.Background(), "pn-123", "token-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.About != "Nossa loja" {
		t.Errorf("About=%q", p.About)
	}
	if p.Email != "e@e.com" || p.Vertical != "RETAIL" {
		t.Errorf("profile not parsed correctly: %+v", p)
	}
}

func TestGetBusinessProfile_EmptyData(t *testing.T) {
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	p, err := GetBusinessProfile(context.Background(), "pn", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil profile")
	}
}

func TestGetBusinessProfile_HTTPError(t *testing.T) {
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid token"}}`))
	}))
	_, err := GetBusinessProfile(context.Background(), "pn", "bad-token")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HTTP 401") {
		t.Errorf("error should mention status: %v", err)
	}
}

func TestUpdateBusinessProfile_SendsAllFields(t *testing.T) {
	var received map[string]interface{}
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected JSON content type, got %s", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))

	profile := BusinessProfile{
		About:       "about",
		Address:     "addr",
		Description: "desc",
		Email:       "e@e.com",
		Websites:    []string{"https://x"},
		Vertical:    "EDU",
	}
	err := UpdateBusinessProfile(context.Background(), "pn", "tok", profile, "handle-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received["messaging_product"] != "whatsapp" {
		t.Error("should include messaging_product=whatsapp")
	}
	if received["about"] != "about" || received["vertical"] != "EDU" {
		t.Errorf("fields not sent: %v", received)
	}
	if received["profile_picture_handle"] != "handle-xyz" {
		t.Errorf("picture handle not sent: %v", received["profile_picture_handle"])
	}
}

func TestUpdateBusinessProfile_OmitsEmptyFields(t *testing.T) {
	var received map[string]interface{}
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		_, _ = w.Write([]byte(`{}`))
	}))

	err := UpdateBusinessProfile(context.Background(), "pn", "tok", BusinessProfile{About: "only-this"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// messaging_product + about only (picture handle must NOT be sent when empty)
	if _, has := received["profile_picture_handle"]; has {
		t.Error("profile_picture_handle should be omitted when empty")
	}
	if _, has := received["email"]; has {
		t.Error("empty email should be omitted")
	}
	if received["about"] != "only-this" {
		t.Error("about should be sent")
	}
}

func TestUpdateBusinessProfile_HTTPError(t *testing.T) {
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad"}}`))
	}))
	err := UpdateBusinessProfile(context.Background(), "pn", "tok", BusinessProfile{About: "x"}, "")
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") {
		t.Errorf("expected HTTP 400 error, got: %v", err)
	}
}

func TestUploadProfilePictureToMeta_TwoStepFlow(t *testing.T) {
	step := 0
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step++
		switch step {
		case 1:
			// Step 1: create upload session
			if !strings.Contains(r.URL.Path, "/app-123/uploads") {
				t.Errorf("step1 path should contain app_id/uploads, got %s", r.URL.Path)
			}
			if !strings.Contains(r.URL.RawQuery, "file_length=3") {
				t.Errorf("step1 should have file_length=3, got %s", r.URL.RawQuery)
			}
			if !strings.Contains(r.URL.RawQuery, "access_token=tok") {
				t.Errorf("step1 should have access_token query")
			}
			_, _ = w.Write([]byte(`{"id":"upload:99"}`))
		case 2:
			// Step 2: upload bytes
			if r.Header.Get("Authorization") != "OAuth tok" {
				t.Errorf("step2 should use OAuth header, got %q", r.Header.Get("Authorization"))
			}
			body, _ := io.ReadAll(r.Body)
			if string(body) != "abc" {
				t.Errorf("step2 body should be raw bytes, got %q", body)
			}
			_, _ = w.Write([]byte(`{"h":"HANDLE_XYZ"}`))
		}
	}))

	handle, err := UploadProfilePictureToMeta(context.Background(), "app-123", "tok", []byte("abc"), "image/jpeg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle != "HANDLE_XYZ" {
		t.Errorf("handle=%q", handle)
	}
	if step != 2 {
		t.Errorf("expected both steps called, step=%d", step)
	}
}

func TestUploadProfilePictureToMeta_Step1Fails(t *testing.T) {
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad"}`))
	}))
	_, err := UploadProfilePictureToMeta(context.Background(), "app", "tok", []byte("x"), "image/jpeg")
	if err == nil || !strings.Contains(err.Error(), "upload init failed") {
		t.Errorf("expected init failure, got: %v", err)
	}
}

func TestUploadProfilePictureToMeta_EmptyUploadID(t *testing.T) {
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`)) // valid JSON but no id
	}))
	_, err := UploadProfilePictureToMeta(context.Background(), "app", "tok", []byte("x"), "image/jpeg")
	if err == nil || !strings.Contains(err.Error(), "empty upload id") {
		t.Errorf("expected empty id error, got: %v", err)
	}
}

func TestUploadProfilePictureToMeta_Step2Fails(t *testing.T) {
	step := 0
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step++
		if step == 1 {
			_, _ = w.Write([]byte(`{"id":"upload:1"}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`boom`))
	}))
	_, err := UploadProfilePictureToMeta(context.Background(), "app", "tok", []byte("x"), "image/jpeg")
	if err == nil || !strings.Contains(err.Error(), "upload bytes failed") {
		t.Errorf("expected step2 failure, got: %v", err)
	}
}

func TestUploadProfilePictureToMeta_EmptyHandle(t *testing.T) {
	step := 0
	withMetaServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step++
		if step == 1 {
			_, _ = w.Write([]byte(`{"id":"upload:1"}`))
		} else {
			_, _ = w.Write([]byte(`{}`)) // no "h" field
		}
	}))
	_, err := UploadProfilePictureToMeta(context.Background(), "app", "tok", []byte("x"), "image/jpeg")
	if err == nil || !strings.Contains(err.Error(), "empty handle") {
		t.Errorf("expected empty handle error, got: %v", err)
	}
}
