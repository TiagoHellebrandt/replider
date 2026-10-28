// Package observability wraps error tracking so the rest of the code
// doesn't depend directly on Sentry. When SENTRY_DSN is empty (local
// dev, tests) every function is a no-op — no nil panics, no surprise
// HTTP calls, no special wiring at call sites.
package observability

import (
	"log/slog"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
)

// InitSentry configures the global Sentry hub. Call once at startup. The
// returned flusher should be invoked in a defer at the top of main so
// in-flight events ship before the process exits.
func InitSentry(dsn, environment, release string) (flush func()) {
	if dsn == "" {
		slog.Info("observability: Sentry DSN not set, error tracking disabled")
		return func() {}
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      environment,
		Release:          release,
		TracesSampleRate: 0.0, // performance tracing off by default — costs add up fast
		AttachStacktrace: true,
		// Strip PII before any event leaves the process (LGPD, US-074).
		BeforeSend: scrubEvent,
		// Silence noisy framework errors that aren't actionable.
		IgnoreErrors: []string{"context canceled"},
	})
	if err != nil {
		slog.Error("observability: Sentry init failed", "error", err)
		return func() {}
	}
	slog.Info("observability: Sentry enabled", "environment", environment)
	return func() { sentry.Flush(2 * time.Second) }
}

// sensitiveHeaders are request headers stripped from every Sentry event:
// they carry credentials or per-user tokens, never useful for debugging and
// dangerous to ship to a third-party dashboard. Matched case-insensitively.
var sensitiveHeaders = map[string]struct{}{
	"authorization":       {},
	"cookie":              {},
	"set-cookie":          {},
	"x-captcha-token":     {},
	"x-api-key":           {},
	"proxy-authorization": {},
}

// scrubEvent is the Sentry BeforeSend hook. It removes credentials and
// personal data from the captured request/context before the event ships,
// reusing the same PII key taxonomy as log redaction (US-050). Returning the
// (mutated) event keeps it; returning nil would drop it entirely.
func scrubEvent(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	if event == nil {
		return nil
	}
	// Request headers: drop credential/token headers wholesale.
	if event.Request != nil {
		if event.Request.Headers != nil {
			for k := range event.Request.Headers {
				if _, bad := sensitiveHeaders[strings.ToLower(k)]; bad {
					event.Request.Headers[k] = redactedPlaceholder
				}
			}
		}
		// Cookies and raw body can both carry session tokens / passwords / PII.
		event.Request.Cookies = ""
		event.Request.Data = ""
		// The query string may embed ?token=... (WS handshake) or ?captcha_token=.
		event.Request.QueryString = scrubQueryString(event.Request.QueryString)
	}
	// Tags / extras: mask any value whose key denotes personal data.
	scrubStringMap(event.Tags)
	scrubExtraMap(event.Extra)
	// User: keep the opaque account/user IDs but never the email/IP.
	event.User.Email = ""
	event.User.IPAddress = ""
	return event
}

// scrubQueryString masks the values of sensitive query parameters while
// preserving the key names, so a leaked token in the URL never reaches Sentry.
func scrubQueryString(qs string) string {
	if qs == "" {
		return qs
	}
	parts := strings.Split(qs, "&")
	for i, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		key := strings.ToLower(kv[0])
		if isPIIKey(key) || strings.Contains(key, "token") || strings.Contains(key, "captcha") {
			parts[i] = kv[0] + "=" + redactedPlaceholder
		}
	}
	return strings.Join(parts, "&")
}

func scrubStringMap(m map[string]string) {
	for k := range m {
		if isPIIKey(k) {
			m[k] = redactedPlaceholder
		}
	}
}

func scrubExtraMap(m map[string]interface{}) {
	for k := range m {
		if isPIIKey(k) {
			m[k] = redactedPlaceholder
		}
	}
}

// GinMiddleware returns a gin middleware that captures panics and
// 5xx responses. Call BEFORE gin.Recovery() so Sentry sees the panic
// first and the recovery handler still returns a clean 500 to the client.
func GinMiddleware() gin.HandlerFunc {
	return sentrygin.New(sentrygin.Options{Repanic: true})
}

// CaptureError reports a non-fatal error with structured tags. Safe to
// call when Sentry is disabled — becomes a no-op.
//
//	observability.CaptureError(err, map[string]string{
//	    "component": "whatsapp_webhook",
//	    "wamid":     parsed.MessageID,
//	})
func CaptureError(err error, tags map[string]string) {
	if err == nil {
		return
	}
	sentry.WithScope(func(s *sentry.Scope) {
		for k, v := range tags {
			s.SetTag(k, v)
		}
		sentry.CaptureException(err)
	})
}

// CaptureMessage reports an informational/warning event without an
// associated error object. Useful for "AI silently produced empty
// response" type situations that aren't exceptions per se.
func CaptureMessage(msg, level string, tags map[string]string) {
	var lvl sentry.Level
	switch level {
	case "warning":
		lvl = sentry.LevelWarning
	case "error":
		lvl = sentry.LevelError
	default:
		lvl = sentry.LevelInfo
	}
	sentry.WithScope(func(s *sentry.Scope) {
		s.SetLevel(lvl)
		for k, v := range tags {
			s.SetTag(k, v)
		}
		sentry.CaptureMessage(msg)
	})
}
