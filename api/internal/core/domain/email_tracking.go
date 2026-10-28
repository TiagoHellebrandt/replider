package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// Open and click tracking on outbound email (§6.8.3).
//
// The hierarchy of what these signals are worth is not a detail — it is the
// whole design:
//
//	resposta > clique > abertura
//
// An OPEN is weak. Apple Mail Privacy Protection pre-fetches images for every
// message a user receives, whether or not they read it, which inflates opens
// by roughly half. It is recorded because the trend still moves, and it is
// labelled weak everywhere it is shown so nobody plans a cadence around it.
//
// A CLICK is strong: somebody chose to act. A click on a pricing link is a
// lead getting warm, and that is worth surfacing.
//
// The security problem this file exists to solve is the redirect. A tracked
// link is by definition "our URL that sends you somewhere else", which is an
// open redirect unless the destination is signed — otherwise anyone can mint
// a link on our domain pointing at their phishing page, and it will carry
// whatever trust our domain has.

// TrackKind distinguishes the two endpoints.
type TrackKind string

const (
	TrackOpen  TrackKind = "o"
	TrackClick TrackKind = "c"
)

// trackPurpose binds a signature to this feature, so a token minted here can
// never be replayed against another that happens to share the secret.
const trackPurpose = "email-track-v1"

// ErrInvalidTrackToken is returned for every failure. One error on purpose:
// a caller learning WHICH check failed learns whether a message id exists.
var ErrInvalidTrackToken = errors.New("invalid tracking token")

// TrackToken is what a tracked URL carries.
type TrackToken struct {
	Kind       TrackKind
	MessageID  string
	TemplateID string
	Version    int
	// URL is the destination, for a click. Empty for an open. Signed with
	// everything else, which is what stops this being an open redirect.
	URL string
}

// MintTrackToken encodes and signs.
func MintTrackToken(secret string, t TrackToken) string {
	payload := strings.Join([]string{
		string(t.Kind), t.MessageID, t.TemplateID,
		strconv.Itoa(t.Version), t.URL,
	}, "\x1f")
	return base64.RawURLEncoding.EncodeToString(
		[]byte(payload + "\x1f" + signTrack(secret, payload)))
}

// ParseTrackToken validates and decodes.
func ParseTrackToken(secret, raw string) (TrackToken, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return TrackToken{}, ErrInvalidTrackToken
	}
	parts := strings.Split(string(decoded), "\x1f")
	if len(parts) != 6 {
		return TrackToken{}, ErrInvalidTrackToken
	}
	payload := strings.Join(parts[:5], "\x1f")
	// Constant time: a plain == leaks how much of a forged signature was
	// right, one byte at a time.
	if !hmac.Equal([]byte(parts[5]), []byte(signTrack(secret, payload))) {
		return TrackToken{}, ErrInvalidTrackToken
	}
	version, err := strconv.Atoi(parts[3])
	if err != nil {
		return TrackToken{}, ErrInvalidTrackToken
	}
	out := TrackToken{
		Kind:       TrackKind(parts[0]),
		MessageID:  parts[1],
		TemplateID: parts[2],
		Version:    version,
		URL:        parts[4],
	}
	if out.Kind != TrackOpen && out.Kind != TrackClick {
		return TrackToken{}, ErrInvalidTrackToken
	}
	// A click with no destination cannot redirect anywhere, and an open with
	// one would be a redirect wearing a pixel's clothes.
	if (out.Kind == TrackClick) == (out.URL == "") {
		return TrackToken{}, ErrInvalidTrackToken
	}
	if out.Kind == TrackClick && !IsSafeRedirectTarget(out.URL) {
		return TrackToken{}, ErrInvalidTrackToken
	}
	return out, nil
}

func signTrack(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(trackPurpose + "\x00" + payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// IsSafeRedirectTarget is checked at MINT time and again at redirect time.
//
// Twice on purpose. The signature already guarantees nobody tampered with the
// destination, but it guarantees nothing about what we ourselves signed: a
// bug that wrapped a `javascript:` href would produce a perfectly valid token
// pointing at a script. The second check is what stops our own mistake from
// becoming a customer's.
func IsSafeRedirectTarget(raw string) bool {
	v := strings.ToLower(strings.TrimSpace(raw))
	return strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "http://")
}

// hrefRe finds link targets in an HTML body.
var hrefRe = regexp.MustCompile(`(?i)(href\s*=\s*)(["'])(https?://[^"']+)(["'])`)

// WrapLinks rewrites every http(s) link so a click goes through the tracker.
//
// Only http(s): a mailto: or tel: link is not a click worth measuring, and
// routing one through a web redirect would break it. Unsubscribe links are
// left alone by the caller, because a tracker sitting between someone and
// their opt-out is the one place a broken redirect becomes a compliance
// problem rather than a lost metric.
func WrapLinks(html, secret, baseURL string, t TrackToken) string {
	if strings.TrimSpace(baseURL) == "" {
		return html
	}
	return hrefRe.ReplaceAllStringFunc(html, func(match string) string {
		g := hrefRe.FindStringSubmatch(match)
		target := g[3]
		if !IsSafeRedirectTarget(target) || IsUnsubscribeLink(target) {
			return match
		}
		tok := MintTrackToken(secret, TrackToken{
			Kind:       TrackClick,
			MessageID:  t.MessageID,
			TemplateID: t.TemplateID,
			Version:    t.Version,
			URL:        target,
		})
		return g[1] + g[2] + strings.TrimRight(baseURL, "/") + "/t/c/" + tok + g[4]
	})
}

var unsubscribeRe = regexp.MustCompile(`(?i)(unsubscribe|descadastr|opt[-_]?out|cancelar[-_]?inscri)`)

// IsUnsubscribeLink reports a link that must never be wrapped.
//
// If the tracker is down, a wrapped unsubscribe link stops working — and a
// broken opt-out is not a lost metric, it is a complaint to the mailbox
// provider and a hit to the sender's reputation.
func IsUnsubscribeLink(url string) bool { return unsubscribeRe.MatchString(url) }

// OpenPixelTag is the img tag appended to a tracked HTML body.
//
// Explicit width/height/alt and a border reset: without them some clients
// render a broken-image placeholder, and a mystery broken image in a cold
// email is exactly the kind of thing that makes a stranger delete it.
func OpenPixelTag(secret, baseURL string, t TrackToken) string {
	if strings.TrimSpace(baseURL) == "" {
		return ""
	}
	tok := MintTrackToken(secret, TrackToken{
		Kind:       TrackOpen,
		MessageID:  t.MessageID,
		TemplateID: t.TemplateID,
		Version:    t.Version,
	})
	return `<img src="` + strings.TrimRight(baseURL, "/") + "/t/o/" + tok +
		`" width="1" height="1" alt="" style="border:0;width:1px;height:1px" />`
}
