package observability

import (
	"log/slog"
	"strings"
	"testing"
)

// A customer's BYOK key was written to a retained log in plaintext, by a WARN
// nobody would think of as a credential site:
//
//	{"level":"WARN","msg":"product pdf import: extraction call failed",
//	 "error":"Post \"https://generativelanguage.googleapis.com/v1beta/models/
//	  gemini-3.7-flash:generateContent?key=AQ.Fake0000...\": context deadline exceeded"}
//
// Key-based masking cannot catch it: the attribute is called "error" and the
// secret is inside the value. Hence the value-level scrub.

func TestRedactPII_ScrubsCredentialInErrorValue(t *testing.T) {
	raw := `Post "https://generativelanguage.googleapis.com/v1beta/models/` +
		`gemini-3.7-flash:generateContent?%24alt=json&key=AQ.Fake0000ExampleKeyNotReal0000000000000000000000": ` +
		`context deadline exceeded`

	got := RedactPII(nil, slog.String("error", raw)).Value.String()

	if strings.Contains(got, "AQ.Fake0000ExampleKeyNotReal0000000000000000000000") {
		t.Fatalf("the API key survived into the log line: %s", got)
	}
	if !strings.Contains(got, "key="+redactedPlaceholder) {
		t.Fatalf("expected the key parameter masked in place, got: %s", got)
	}
	// An error whose URL is entirely redacted is an error nobody can debug.
	for _, keep := range []string{"generativelanguage.googleapis.com", "gemini-3.7-flash", "context deadline exceeded"} {
		if !strings.Contains(got, keep) {
			t.Fatalf("scrub ate the debuggable part (%q missing): %s", keep, got)
		}
	}
}

func TestScrubSecrets(t *testing.T) {
	cases := map[string]struct{ in, wantMasked string }{
		"api_key":      {"https://x/y?api_key=abc123&z=1", "abc123"},
		"access_token": {"GET https://x?access_token=tok_live_9", "tok_live_9"},
		"token":        {"https://x?a=1&token=zzz", "zzz"},
		"secret":       {"https://x?secret=shh", "shh"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ScrubSecrets(tc.in)
			if strings.Contains(got, tc.wantMasked) {
				t.Fatalf("secret survived: %s", got)
			}
			if !strings.Contains(got, redactedPlaceholder) {
				t.Fatalf("nothing was masked: %s", got)
			}
		})
	}
}

func TestScrubSecrets_LeavesOrdinaryTextAlone(t *testing.T) {
	for _, s := range []string{
		"connection refused",
		"produto=Ventilador Infantil",      // a "=" is not a credential
		"https://api.example.com/v1/plans", // no query at all
	} {
		if got := ScrubSecrets(s); got != s {
			t.Fatalf("rewrote innocent text %q into %q", s, got)
		}
	}
}
