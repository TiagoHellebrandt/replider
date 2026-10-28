package email

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

// Signature check for the inbound-mail webhook.
//
// Every other channel verified its sender and this one did not: WhatsApp and
// Instagram check a Meta HMAC, SMS checks X-Twilio-Signature, Telegram checks a
// secret token in constant time. E-mail checked nothing, so the only thing
// standing between a stranger and the seller's inbox was the channel token in
// the URL — and a secret in a URL leaks through proxy logs, Referer headers,
// browser history and the copy button in our own interface.
//
// That gap mattered more here than it would elsewhere. The webhook path fills
// InboundEmail.Auth from the POST body (ParseAuthResults), while the IMAP path
// verifies DKIM against the raw message (VerifyDKIM). So a forged POST could
// declare `spf=pass; dkim=pass` and AssessEmailRisk would grade it clean —
// which is exactly the supplier-impersonation fraud that risk layer exists to
// catch, walking in through the one door with no lock. Once the signature is
// required and fails closed, the body can only have come from the provider we
// chose, and trusting its verdict is a decision about the provider rather than
// about a stranger.
//
// The scheme is Standard Webhooks (what Resend, and Svix under it, send). The
// header names ship in two spellings and both are accepted.

// webhookToleranceSeconds bounds replay. Standard Webhooks recommends five
// minutes; a captured request older than that is refused even with a perfect
// signature, so a POST lifted from a log cannot be replayed tomorrow.
const webhookToleranceSeconds = 300

// VerifyWebhookSignature reports whether body was signed by the holder of
// secret, using the Standard Webhooks scheme.
//
// secret is the `whsec_`-prefixed value the provider shows once; the prefix is
// optional here so an operator who pasted it either way is not silently
// unprotected. now is injected for tests.
func VerifyWebhookSignature(secret string, headers func(string) string, body []byte, now time.Time) bool {
	key := decodeWebhookSecret(secret)
	if len(key) == 0 || headers == nil {
		return false
	}
	id := firstHeader(headers, "svix-id", "webhook-id")
	ts := firstHeader(headers, "svix-timestamp", "webhook-timestamp")
	sigs := firstHeader(headers, "svix-signature", "webhook-signature")
	if id == "" || ts == "" || sigs == "" {
		return false
	}
	if !timestampFresh(ts, now) {
		return false
	}

	mac := hmac.New(sha256.New, key)
	// The signed content is id.timestamp.body — the id and the timestamp are
	// inside the MAC, which is what makes the replay window enforceable
	// instead of advisory.
	mac.Write([]byte(id))
	mac.Write([]byte("."))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(body)
	want := mac.Sum(nil)

	// The header carries a space-separated list so a secret can be rotated
	// with both keys live. Every candidate is compared in constant time, and
	// the loop does not break early on a match: bailing out on the first hit
	// would make the work depend on which entry matched.
	ok := false
	for _, entry := range strings.Fields(sigs) {
		version, encoded, found := strings.Cut(entry, ",")
		if !found || version != "v1" {
			continue
		}
		got, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			continue
		}
		if hmac.Equal(got, want) {
			ok = true
		}
	}
	return ok
}

// decodeWebhookSecret turns the provider's `whsec_<base64>` into raw key
// bytes. A secret that does not decode yields nil, which VerifyWebhookSignature
// treats as "no secret" — refusing, never passing.
func decodeWebhookSecret(secret string) []byte {
	s := strings.TrimSpace(secret)
	if s == "" {
		return nil
	}
	s = strings.TrimPrefix(s, "whsec_")
	key, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	return key
}

func timestampFresh(ts string, now time.Time) bool {
	secs, err := strconv.ParseInt(strings.TrimSpace(ts), 10, 64)
	if err != nil {
		return false
	}
	// Both directions: a timestamp far in the FUTURE is as much a sign of a
	// forged request as a stale one, and accepting it would hand an attacker
	// an unbounded replay window.
	drift := now.Unix() - secs
	if drift < 0 {
		drift = -drift
	}
	return drift <= webhookToleranceSeconds
}

func firstHeader(headers func(string) string, names ...string) string {
	for _, n := range names {
		if v := strings.TrimSpace(headers(n)); v != "" {
			return v
		}
	}
	return ""
}
