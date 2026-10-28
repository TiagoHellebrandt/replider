package whatsapp

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestIsNonIdempotentPath(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   bool
	}{
		{"POST", "/v25.0/pn/messages", true},
		{"POST", "/v25.0/pn/register", true},
		{"POST", "/v25.0/pn/deregister", true},
		{"POST", "/v25.0/pn/verify_code", true},
		{"POST", "/v25.0/pn/request_code", true},
		{"GET", "/v25.0/pn/messages", false},
		{"POST", "/v25.0/waba/subscribed_apps", false},
		{"POST", "/v25.0/pn/media", false},
		{"GET", "/v25.0/pn/register", false},
	}
	for _, c := range cases {
		req, _ := http.NewRequest(c.method, "https://graph.facebook.com"+c.path, nil)
		got := isNonIdempotentPath(req)
		if got != c.want {
			t.Errorf("%s %s: got %v, want %v", c.method, c.path, got, c.want)
		}
	}
}

func TestIsNonIdempotentPath_NilRequest(t *testing.T) {
	if isNonIdempotentPath(nil) {
		t.Error("expected false for nil request")
	}
}

func TestRedactGraphURL(t *testing.T) {
	cases := []struct {
		raw     string
		want    string
		notWant string
	}{
		{
			raw:     "https://graph.facebook.com/v25.0/me?access_token=SECRETTOKEN&fields=id",
			want:    "access_token=%5Bredacted%5D",
			notWant: "SECRETTOKEN",
		},
		{
			raw:     "https://graph.facebook.com/v25.0/oauth/access_token?client_id=1&client_secret=SHHH&code=abc",
			want:    "client_secret=%5Bredacted%5D",
			notWant: "SHHH",
		},
		{
			raw:     "https://graph.facebook.com/v25.0/me?fields=id",
			want:    "fields=id",
			notWant: "redacted",
		},
	}
	for _, c := range cases {
		u, _ := url.Parse(c.raw)
		got := redactGraphURL(u)
		if c.want != "" && !strings.Contains(got, c.want) {
			t.Errorf("%s: expected %q in result, got %q", c.raw, c.want, got)
		}
		if c.notWant != "" && strings.Contains(got, c.notWant) {
			t.Errorf("%s: did NOT expect %q in result, got %q", c.raw, c.notWant, got)
		}
	}
}

func TestRedactGraphURL_NilURL(t *testing.T) {
	if got := redactGraphURL(nil); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}
