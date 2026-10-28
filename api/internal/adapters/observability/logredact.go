package observability

import (
	"log/slog"
	"regexp"
	"strings"
)

// redactedPlaceholder is what every PII value is replaced with in logs.
const redactedPlaceholder = "[REDACTED]"

// piiKeys is the set of slog attribute keys whose values are personal data
// under LGPD and must never be written to logs (US-050). Matching is
// case-insensitive and also covers common suffixed/prefixed variants
// (e.g. "phone_number", "user_email", "contact_cpf") via substring match.
var piiKeys = map[string]struct{}{
	"phone": {},
	"email": {},
	"cpf":   {},
}

// isPIIKey reports whether a normalized attribute key denotes personal data.
// It matches an exact key or any key that contains a PII token as a
// word-ish fragment (handling snake_case/camelCase via lowercasing), so
// "phone_number", "from_phone", "user_email" and "contact_cpf" are all caught.
func isPIIKey(key string) bool {
	k := strings.ToLower(key)
	if _, ok := piiKeys[k]; ok {
		return true
	}
	for token := range piiKeys {
		if strings.Contains(k, token) {
			return true
		}
	}
	return false
}

// RedactPII is an slog HandlerOptions.ReplaceAttr function that masks the
// values of personal-data attributes (phone, email, cpf and variants) before
// they reach the log output. Wire it into the JSON/Text handler used by the
// application logger so every slog call site is covered without per-call-site
// changes. Group attributes are passed through untouched (their children are
// visited individually by slog).
func RedactPII(_ []string, a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindGroup {
		return a
	}
	if isPIIKey(a.Key) {
		a.Value = slog.StringValue(redactedPlaceholder)
		return a
	}
	// Key-based masking cannot see a secret carried INSIDE a value. An SDK
	// error quotes the URL it failed on, credentials and all:
	//
	//   Post "https://generativelanguage.googleapis.com/...?key=AQ.Fake0000..."
	//     : context deadline exceeded
	//
	// That is a customer's BYOK key, in plaintext, in a retained log, written
	// by a WARN nobody would think of as a credential site. So the scrub runs
	// on the VALUE, at the same single seam.
	if a.Value.Kind() == slog.KindString {
		if scrubbed := ScrubSecrets(a.Value.String()); scrubbed != a.Value.String() {
			a.Value = slog.StringValue(scrubbed)
		}
		return a
	}
	// An error carried as a value is the most common attribute in this
	// codebase and the least useful one once stored: almost every error type
	// is a struct with no exported fields, so `json.Marshal` writes `{}` and
	// the retained log records that something failed without recording what.
	// Production had 144 ERROR rows and not one of them said why.
	//
	// Flattening to Error() here — the same seam the key scrub uses — fixes
	// both sinks at once, and the message goes through the scrub because an
	// SDK error quotes the URL it failed on, credentials and all.
	if a.Value.Kind() == slog.KindAny {
		if err, ok := a.Value.Any().(error); ok {
			if err == nil {
				return slog.String(a.Key, "")
			}
			return slog.String(a.Key, ScrubSecrets(err.Error()))
		}
	}
	return a
}

// credentialInQuery matches a credential passed as a URL query parameter.
// Deliberately narrow: it masks the VALUE of a parameter whose name says
// credential, and leaves the rest of the message readable — an error whose
// URL is entirely redacted is an error nobody can debug.
var credentialInQuery = regexp.MustCompile(
	`(?i)([?&](?:key|api[_-]?key|access[_-]?token|auth[_-]?token|token|secret|password|signature|sig)=)[^&"'\s]+`)

// ScrubSecrets masks credentials embedded in a free-text string (typically an
// error quoting a URL). Exported because anything that persists or forwards a
// provider error — an alert body, a stored job failure — needs the same
// treatment as a log line.
func ScrubSecrets(s string) string {
	// Cheap reject: no query separator, no credential to find.
	if !strings.Contains(s, "=") {
		return s
	}
	return credentialInQuery.ReplaceAllString(s, "${1}"+redactedPlaceholder)
}
