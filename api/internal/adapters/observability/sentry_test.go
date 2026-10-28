package observability

import (
	"testing"

	"github.com/getsentry/sentry-go"
)

// TestScrubEvent_StripsCredentialHeaders proves credential/token headers are
// masked before an event would ship to Sentry.
func TestScrubEvent_StripsCredentialHeaders(t *testing.T) {
	ev := &sentry.Event{
		Request: &sentry.Request{
			Headers: map[string]string{
				"Authorization":   "Bearer secret-jwt",
				"Cookie":          "session=abc",
				"X-Captcha-Token": "tok",
				"User-Agent":      "curl/8",
			},
		},
	}
	out := scrubEvent(ev, nil)
	if out == nil {
		t.Fatal("event was dropped")
	}
	for _, k := range []string{"Authorization", "Cookie", "X-Captcha-Token"} {
		if got := out.Request.Headers[k]; got != redactedPlaceholder {
			t.Errorf("header %q = %q, want redacted", k, got)
		}
	}
	// Non-sensitive header preserved.
	if out.Request.Headers["User-Agent"] != "curl/8" {
		t.Errorf("User-Agent should be preserved, got %q", out.Request.Headers["User-Agent"])
	}
}

// TestScrubEvent_ClearsBodyCookiesAndUserPII ensures the request body,
// cookies and user email/IP never reach the dashboard.
func TestScrubEvent_ClearsBodyCookiesAndUserPII(t *testing.T) {
	ev := &sentry.Event{
		Request: &sentry.Request{
			Cookies: "session=abc",
			Data:    `{"password":"hunter2","phone":"+5511999"}`,
		},
	}
	ev.User.ID = "acc_123"
	ev.User.Email = "a@b.com"
	ev.User.IPAddress = "1.2.3.4"

	out := scrubEvent(ev, nil)
	if out.Request.Cookies != "" {
		t.Errorf("cookies not cleared: %q", out.Request.Cookies)
	}
	if out.Request.Data != "" {
		t.Errorf("body not cleared: %q", out.Request.Data)
	}
	if out.User.Email != "" || out.User.IPAddress != "" {
		t.Errorf("user PII not cleared: email=%q ip=%q", out.User.Email, out.User.IPAddress)
	}
	// Opaque account ID is useful and kept.
	if out.User.ID != "acc_123" {
		t.Errorf("user ID should be kept, got %q", out.User.ID)
	}
}

// TestScrubEvent_MasksQueryTokensAndPIITags covers query-string tokens and
// PII-keyed tags/extras.
func TestScrubEvent_MasksQueryTokensAndPIITags(t *testing.T) {
	ev := &sentry.Event{
		Request: &sentry.Request{
			QueryString: "token=jwt123&captcha_token=ct&limit=20",
		},
		Tags:  map[string]string{"phone": "+5511999", "component": "webhook"},
		Extra: map[string]interface{}{"user_email": "x@y.com", "count": 3},
	}
	out := scrubEvent(ev, nil)
	if want := "token=" + redactedPlaceholder + "&captcha_token=" + redactedPlaceholder + "&limit=20"; out.Request.QueryString != want {
		t.Errorf("query string = %q, want %q", out.Request.QueryString, want)
	}
	if out.Tags["phone"] != redactedPlaceholder {
		t.Errorf("PII tag not masked: %q", out.Tags["phone"])
	}
	if out.Tags["component"] != "webhook" {
		t.Errorf("non-PII tag should be kept: %q", out.Tags["component"])
	}
	if out.Extra["user_email"] != redactedPlaceholder {
		t.Errorf("PII extra not masked: %v", out.Extra["user_email"])
	}
	if out.Extra["count"] != 3 {
		t.Errorf("non-PII extra should be kept: %v", out.Extra["count"])
	}
}
