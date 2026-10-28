package observability

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"strings"
	"testing"
)

// newRedactingLogger mirrors the production wiring in cmd/api/main.go so the
// test exercises the real ReplaceAttr path, writing JSON to a buffer.
func newRedactingLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{
		ReplaceAttr: RedactPII,
	}))
}

func TestRedactPII_MasksSensitiveFields(t *testing.T) {
	var buf bytes.Buffer
	logger := newRedactingLogger(&buf)

	logger.Info("inbound message",
		"phone", "+5511999998888",
		"email", "user@example.com",
		"cpf", "123.456.789-09",
		"phone_number", "+5511777776666",
		"user_email", "admin@replider.com",
		"contact_cpf", "987.654.321-00",
		"conversation_id", "conv_123",
		"account_id", "acc_456",
	)

	out := buf.String()

	// PII values must be gone.
	for _, leaked := range []string{
		"+5511999998888", "user@example.com", "123.456.789-09",
		"+5511777776666", "admin@replider.com", "987.654.321-00",
	} {
		if strings.Contains(out, leaked) {
			t.Errorf("log output leaked PII value %q: %s", leaked, out)
		}
	}

	// Non-PII values must survive.
	for _, kept := range []string{"conv_123", "acc_456"} {
		if !strings.Contains(out, kept) {
			t.Errorf("log output dropped non-PII value %q: %s", kept, out)
		}
	}

	// Redacted keys carry the placeholder.
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("log line is not valid JSON: %v", err)
	}
	for _, k := range []string{"phone", "email", "cpf", "phone_number", "user_email", "contact_cpf"} {
		if rec[k] != redactedPlaceholder {
			t.Errorf("key %q = %v, want %q", k, rec[k], redactedPlaceholder)
		}
	}
}

func TestRedactPII_PreservesGroupsAndOtherFields(t *testing.T) {
	var buf bytes.Buffer
	logger := newRedactingLogger(&buf)

	logger.Info("nested",
		slog.Group("contact", "phone", "+5511999998888", "name", "Maria"),
	)

	out := buf.String()
	if strings.Contains(out, "+5511999998888") {
		t.Errorf("PII inside group not redacted: %s", out)
	}
	if !strings.Contains(out, "Maria") {
		t.Errorf("non-PII inside group dropped: %s", out)
	}
}

func TestIsPIIKey(t *testing.T) {
	cases := map[string]bool{
		"phone":           true,
		"Email":           true,
		"CPF":             true,
		"phone_number":    true,
		"from_phone":      true,
		"user_email":      true,
		"account_id":      false,
		"conversation_id": false,
		"name":            false,
	}
	for key, want := range cases {
		if got := isPIIKey(key); got != want {
			t.Errorf("isPIIKey(%q) = %v, want %v", key, got, want)
		}
	}
}

// The 2026-09-09 audit found 144 ERROR rows in production whose `error`
// attribute was the literal `{}`: an error type is a struct with no exported
// fields, so the JSON encoder had nothing to write. A log that records a
// failure without recording what failed is a log that costs an SSH session.
func TestRedactPII_FlattensErrorValuesToTheirMessage(t *testing.T) {
	got := RedactPII(nil, slog.Any("error", errors.New("pncp: status 503")))
	if got.Value.Kind() != slog.KindString {
		t.Fatalf("kind = %v, want a string", got.Value.Kind())
	}
	if got.Value.String() != "pncp: status 503" {
		t.Fatalf("value = %q, want the error message", got.Value.String())
	}
}

// The flattened message still goes through the credential scrub — an SDK
// error quotes the URL it failed on, key and all.
func TestRedactPII_ScrubsCredentialsInsideAnErrorValue(t *testing.T) {
	err := errors.New(`Post "https://x.googleapis.com/v1?key=AQ.Fake00secret": deadline`)
	got := RedactPII(nil, slog.Any("error", err))
	if strings.Contains(got.Value.String(), "AQ.Fake00secret") {
		t.Fatalf("credential survived: %q", got.Value.String())
	}
	if !strings.Contains(got.Value.String(), "deadline") {
		t.Fatalf("message was over-redacted: %q", got.Value.String())
	}
}

// Neither shape of nil error may panic or render as an empty object. A
// typed nil goes through Error() (net.OpError guards its own nil); a bare
// nil interface never satisfies the assertion and keeps slog's "<nil>".
func TestRedactPII_NilErrorValuesDoNotPanic(t *testing.T) {
	var typed *net.OpError
	if s := RedactPII(nil, slog.Any("error", error(typed))).Value.String(); s == "{}" {
		t.Fatalf("typed nil rendered as an empty object")
	}

	var bare error
	if s := RedactPII(nil, slog.Any("error", bare)).Value.String(); s == "{}" {
		t.Fatalf("bare nil rendered as an empty object")
	}
}
