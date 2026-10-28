package twilio

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	twclient "github.com/twilio/twilio-go/client"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"

	"github.com/replider/api/internal/core/domain"
)

// fakeTransport implements twclient.BaseClient, re-pointing the SDK's
// pinned https://api.twilio.com base at an httptest server so adapter
// tests exercise the REAL generated request encoding over real HTTP.
type fakeTransport struct {
	serverURL  string
	accountSID string
	authToken  string
}

func (f *fakeTransport) AccountSid() string         { return f.accountSID }
func (f *fakeTransport) SetTimeout(_ time.Duration) {}
func (f *fakeTransport) SetOauth(_ twclient.OAuth)  {}
func (f *fakeTransport) OAuth() twclient.OAuth      { return nil }
func (f *fakeTransport) SendRequest(method, rawURL string, data url.Values,
	headers map[string]interface{}, body ...byte) (*http.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, f.serverURL+u.Path, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(f.accountSID, f.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	// Mirror the real twclient.Client contract: non-2xx decodes into a
	// TwilioRestError and comes back as the error, never as a response.
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		restErr := &twclient.TwilioRestError{}
		if decodeErr := json.NewDecoder(resp.Body).Decode(restErr); decodeErr != nil {
			return nil, decodeErr
		}
		return nil, restErr
	}
	return resp, nil
}

// testAdapter wires a TwilioAdapter at server, returning it.
func testAdapter(server *httptest.Server) *TwilioAdapter {
	a := NewTwilioAdapter()
	a.newMessageAPI = func(accountSID, authToken string) messageAPI {
		return openapi.NewApiServiceWithClient(&fakeTransport{
			serverURL:  server.URL,
			accountSID: accountSID,
			authToken:  authToken,
		})
	}
	return a
}

func smsChannel(t *testing.T) *domain.Channel {
	t.Helper()
	cfg, err := json.Marshal(map[string]string{
		"account_sid": "ACtest00000000000000000000000000",
		"auth_token":  "secret-token",
		"from_number": "+18005551234",
	})
	if err != nil {
		t.Fatal(err)
	}
	return &domain.Channel{
		ID:        "ch_sms_1",
		AccountID: "acc_1",
		Type:      "sms",
		Config:    json.RawMessage(cfg),
	}
}

func textMsg(content string) *domain.Message {
	return &domain.Message{Type: "text", Content: &content}
}

func okResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprint(w, `{"sid":"SMxxxx","status":"queued","error_code":null}`)
}

func TestSendMessage_Text(t *testing.T) {
	var gotPath, gotUser, gotPass string
	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotUser, gotPass, _ = r.BasicAuth()
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		gotForm = r.PostForm
		okResponse(w)
	}))
	defer server.Close()

	err := testAdapter(server).SendMessage(context.Background(), smsChannel(t), "+15559876543", textMsg("hello there"))
	if err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}

	wantPath := "/2010-04-01/Accounts/ACtest00000000000000000000000000/Messages.json"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
	if gotUser != "ACtest00000000000000000000000000" || gotPass != "secret-token" {
		t.Errorf("basic auth = %q/%q, want channel credentials", gotUser, gotPass)
	}
	if got := gotForm.Get("To"); got != "+15559876543" {
		t.Errorf("To = %q", got)
	}
	if got := gotForm.Get("From"); got != "+18005551234" {
		t.Errorf("From = %q", got)
	}
	if got := gotForm.Get("Body"); got != "hello there" {
		t.Errorf("Body = %q", got)
	}
	if got := gotForm.Get("MediaUrl"); got != "" {
		t.Errorf("MediaUrl should be absent for text, got %q", got)
	}
}

func TestSendMessage_MediaBecomesMMS(t *testing.T) {
	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.PostForm
		okResponse(w)
	}))
	defer server.Close()

	caption := "product photo"
	mediaURL := "https://cdn.example.com/img.jpg"
	msg := &domain.Message{Type: "image", Content: &caption, MediaURL: &mediaURL}
	if err := testAdapter(server).SendMessage(context.Background(), smsChannel(t), "+15559876543", msg); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if got := gotForm.Get("MediaUrl"); got != mediaURL {
		t.Errorf("MediaUrl = %q, want %q", got, mediaURL)
	}
	if got := gotForm.Get("Body"); got != caption {
		t.Errorf("Body = %q, want caption %q", got, caption)
	}
}

func TestSendMessage_LocationBecomesMapsLink(t *testing.T) {
	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.PostForm
		okResponse(w)
	}))
	defer server.Close()

	latlon := "-23.550520, -46.633308"
	msg := &domain.Message{Type: "location", Content: &latlon}
	if err := testAdapter(server).SendMessage(context.Background(), smsChannel(t), "+15559876543", msg); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	want := "https://maps.google.com/?q=-23.550520,-46.633308"
	if got := gotForm.Get("Body"); got != want {
		t.Errorf("Body = %q, want %q", got, want)
	}
}

func TestSendMessage_BodyOver1600CharsTruncated(t *testing.T) {
	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.PostForm
		okResponse(w)
	}))
	defer server.Close()

	long := strings.Repeat("é", 2500) // multibyte to prove rune-boundary truncation
	if err := testAdapter(server).SendMessage(context.Background(), smsChannel(t), "+15559876543", textMsg(long)); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	body := gotForm.Get("Body")
	runes := []rune(body)
	if len(runes) != smsMaxBodyRunes {
		t.Errorf("truncated body = %d runes, want %d", len(runes), smsMaxBodyRunes)
	}
	if runes[len(runes)-1] != '…' {
		t.Errorf("truncated body should end with ellipsis, got %q", string(runes[len(runes)-10:]))
	}
	if !strings.HasPrefix(body, "éé") {
		t.Errorf("truncated body lost its prefix")
	}
}

func TestSendMessage_BodyExactly1600CharsUntouched(t *testing.T) {
	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.PostForm
		okResponse(w)
	}))
	defer server.Close()

	exact := strings.Repeat("a", smsMaxBodyRunes)
	if err := testAdapter(server).SendMessage(context.Background(), smsChannel(t), "+15559876543", textMsg(exact)); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if got := gotForm.Get("Body"); got != exact {
		t.Errorf("exactly-at-cap body was modified (len %d)", len(got))
	}
}

func TestSendMessage_TwilioAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"code":21211,"message":"The 'To' number is not a valid phone number.","more_info":"https://www.twilio.com/docs/errors/21211","status":400}`)
	}))
	defer server.Close()

	err := testAdapter(server).SendMessage(context.Background(), smsChannel(t), "not-a-phone", textMsg("hi"))
	if err == nil {
		t.Fatal("expected error from Twilio 400, got nil")
	}
	if !strings.Contains(err.Error(), "21211") {
		t.Errorf("error should surface Twilio code 21211, got: %v", err)
	}
}

func TestSendMessage_ErrorCodeInSuccessBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"sid":"SMxxxx","status":"failed","error_code":30007}`)
	}))
	defer server.Close()

	err := testAdapter(server).SendMessage(context.Background(), smsChannel(t), "+15559876543", textMsg("hi"))
	if err == nil {
		t.Fatal("expected error when response carries error_code, got nil")
	}
	if !strings.Contains(err.Error(), "30007") {
		t.Errorf("error should surface error code 30007, got: %v", err)
	}
}

func TestSendMessage_ConfigValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no HTTP call expected for invalid config")
	}))
	defer server.Close()
	adapter := testAdapter(server)

	cases := []struct {
		name   string
		config interface{}
	}{
		{"nil config", nil},
		{"missing account_sid", json.RawMessage(`{"auth_token":"t","from_number":"+1800"}`)},
		{"missing auth_token", json.RawMessage(`{"account_sid":"AC1","from_number":"+1800"}`)},
		{"missing from_number", json.RawMessage(`{"account_sid":"AC1","auth_token":"t"}`)},
		{"malformed json", json.RawMessage(`{not json`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ch := &domain.Channel{ID: "ch_bad", Type: "sms", Config: tc.config}
			if err := adapter.SendMessage(context.Background(), ch, "+15559876543", textMsg("hi")); err == nil {
				t.Error("expected config error, got nil")
			}
		})
	}
}

func TestSendMessage_EmptyRecipientAndEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no HTTP call expected")
	}))
	defer server.Close()
	adapter := testAdapter(server)

	if err := adapter.SendMessage(context.Background(), smsChannel(t), "", textMsg("hi")); err == nil {
		t.Error("expected error for empty recipient")
	}
	if err := adapter.SendMessage(context.Background(), smsChannel(t), "+15559876543", textMsg("")); err == nil {
		t.Error("expected error for message with no body and no media")
	}
}

func TestSendMessage_CancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no HTTP call expected after ctx cancel")
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := testAdapter(server).SendMessage(ctx, smsChannel(t), "+15559876543", textMsg("hi")); err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestHandleWebhook_NoOp(t *testing.T) {
	adapter := NewTwilioAdapter()
	if err := adapter.HandleWebhook(context.Background(), smsChannel(t), []byte("From=%2B1555&Body=hi")); err != nil {
		t.Errorf("HandleWebhook should be a no-op, got: %v", err)
	}
}

func TestTruncateBody(t *testing.T) {
	short, truncated := truncateBody("hello")
	if truncated || short != "hello" {
		t.Errorf("short body modified: %q %v", short, truncated)
	}
	long, truncated := truncateBody(strings.Repeat("x", 1601))
	if !truncated || len([]rune(long)) != smsMaxBodyRunes {
		t.Errorf("long body not capped: %d runes, truncated=%v", len([]rune(long)), truncated)
	}
}
