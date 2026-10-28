package email

import (
	"encoding/base64"
	"path/filepath"
	"regexp"
	"strings"
)

// Attachments and authentication results on inbound mail.
//
// Both were missing, and both are the kind of gap that is invisible until it
// costs something. A clinic replying "segue em anexo o termo de referência"
// produced a message with no attachment and no warning that one had been
// dropped. And with nobody checking SPF/DKIM/DMARC, anyone could forge a
// sender and have it land in the funnel as a lead's reply.

// InboundAttachment is one file that came with the message.
type InboundAttachment struct {
	Filename    string
	ContentType string
	// Content is the decoded bytes. Nil when the provider sent only a URL
	// (Mailgun's default) — the caller then has to fetch it, and MUST do so
	// through the SSRF-guarded client.
	Content []byte
	// URL is set instead of Content by providers that host the file.
	URL string
	// Size is the declared size in bytes; 0 when the provider omitted it.
	Size int64
	// ContentID is the cid: reference for an INLINE image (a signature logo,
	// a tracking pixel). Kept so the caller can tell a real document from
	// message furniture — showing a 1x1 pixel as "1 anexo" trains people to
	// ignore the attachment indicator, which is exactly when the real
	// contract slips past.
	ContentID string
	// Inline is decided by the DISPOSITION (plus cid:-referenced images):
	// Gmail stamps a Content-ID on regular attachments too, so ContentID
	// alone flagged a contract PDF as message furniture and hid it from the
	// attachment counter.
	Inline bool
}

// IsInline reports whether this is embedded message furniture rather than a
// document someone meant to send.
func (a InboundAttachment) IsInline() bool { return a.Inline }

// SafeFilename strips any directory component and control characters from
// the provider-supplied name.
//
// The name comes from the SENDER, so it is hostile input: "../../etc/passwd"
// and a name full of newlines are both things a real attacker sends. This
// never returns empty — an unnamed file still has to be storable and
// clickable.
func (a InboundAttachment) SafeFilename() string {
	name := filepath.Base(strings.TrimSpace(a.Filename))
	name = unsafeNameRe.ReplaceAllString(name, "_")
	name = strings.TrimLeft(name, ".")
	if len(name) > 180 {
		// Keep the extension: it is what tells the reader (and the browser)
		// what the file is.
		ext := filepath.Ext(name)
		if len(ext) > 12 {
			ext = ""
		}
		name = name[:180-len(ext)] + ext
	}
	if strings.TrimSpace(name) == "" {
		return "anexo"
	}
	return name
}

var unsafeNameRe = regexp.MustCompile(`[^\p{L}\p{N} ._()\-\[\]+&@]`)

// rawAttachment is the union of the shapes providers use. Resend and
// SendGrid send snake_case, Postmark PascalCase, Mailgun hyphenated — same
// union strategy as rawInbound, so a new vendor is a new key rather than a
// new package.
type rawAttachment struct {
	Filename  string `json:"filename"`
	Name      string `json:"Name"`
	FileName2 string `json:"file_name"`

	ContentType  string `json:"content_type"`
	ContentType2 string `json:"ContentType"`
	ContentType3 string `json:"type"`

	// Base64 payload. Providers disagree on whether it is "content" or
	// "Content"; Go's encoding/json is case-insensitive on match but an
	// explicit tag documents which vendor sends what.
	Content  string `json:"content"`
	Content2 string `json:"Content"`

	URL  string `json:"url"`
	URL2 string `json:"Url"`

	Size  int64 `json:"size"`
	Size2 int64 `json:"ContentLength"`

	ContentID  string `json:"content_id"`
	ContentID2 string `json:"ContentID"`
	ContentID3 string `json:"cid"`
}

func (r rawAttachment) normalize() (InboundAttachment, bool) {
	out := InboundAttachment{
		Filename:    firstNonEmpty(r.Filename, r.Name, r.FileName2),
		ContentType: strings.ToLower(firstNonEmpty(r.ContentType, r.ContentType2, r.ContentType3)),
		URL:         firstNonEmpty(r.URL, r.URL2),
		ContentID:   strings.Trim(firstNonEmpty(r.ContentID, r.ContentID2, r.ContentID3), "<>"),
	}
	// Same rule as the MIME walk: only a cid:-referenced IMAGE is furniture.
	out.Inline = out.ContentID != "" &&
		strings.HasPrefix(out.ContentType, "image/")
	if r.Size > 0 {
		out.Size = r.Size
	} else if r.Size2 > 0 {
		out.Size = r.Size2
	}

	if payload := firstNonEmpty(r.Content, r.Content2); payload != "" {
		// Providers vary between standard and URL-safe base64, and some wrap
		// at 76 columns. Decode leniently rather than dropping a real
		// document over an encoding detail.
		if decoded, err := decodeLenientBase64(payload); err == nil {
			out.Content = decoded
			if out.Size == 0 {
				out.Size = int64(len(decoded))
			}
		}
	}

	// Nothing to point at: no bytes, no URL. Dropping it silently is what the
	// old parser did to EVERY attachment, so the caller is told instead by
	// simply not receiving an entry it cannot act on.
	if len(out.Content) == 0 && out.URL == "" {
		return InboundAttachment{}, false
	}
	return out, true
}

func decodeLenientBase64(s string) ([]byte, error) {
	cleaned := strings.NewReplacer("\n", "", "\r", "", " ", "").Replace(s)
	if b, err := base64.StdEncoding.DecodeString(cleaned); err == nil {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(cleaned); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(cleaned); err == nil {
		return b, nil
	}
	return base64.RawURLEncoding.DecodeString(cleaned)
}

// AuthResult is what the receiving mail server concluded about the sender.
//
// This is NOT proof of identity on its own — it is the provider's verdict,
// trusted because the webhook itself is authenticated. What it buys is the
// difference between "this really came from the clinic's domain" and "someone
// typed that address into the From line", which is otherwise invisible.
type AuthResult string

const (
	// AuthUnknown means nobody told us — an older provider, or a payload
	// without the header. It must NOT be shown as "verified": the whole
	// point is to stop treating an unchecked message as a checked one.
	AuthUnknown AuthResult = "unknown"
	AuthPass    AuthResult = "pass"
	AuthFail    AuthResult = "fail"
	// AuthNeutral covers none/neutral/softfail/policy — the domain did not
	// publish a usable policy, which is weaker than a pass and not a forgery
	// signal on its own.
	AuthNeutral AuthResult = "neutral"
)

// InboundAuth is the three-way verdict plus the raw header it came from.
type InboundAuth struct {
	SPF   AuthResult
	DKIM  AuthResult
	DMARC AuthResult
	// Raw is the Authentication-Results header verbatim, kept so a support
	// conversation about "why was this flagged" has evidence rather than a
	// summary.
	Raw string
}

// Trustworthy reports whether the message can be treated as genuinely from
// the domain it claims.
//
// DMARC passing is sufficient on its own: DMARC pass REQUIRES an aligned SPF
// or DKIM pass, so it already subsumes both. Without DMARC, a DKIM pass is
// the stronger remaining signal because it survives forwarding, which SPF
// does not.
//
// Deliberately conservative: unknown is not trustworthy. A verdict we never
// computed must never render as a checkmark.
func (a InboundAuth) Trustworthy() bool {
	if a.DMARC == AuthPass {
		return true
	}
	if a.DMARC == AuthFail {
		return false
	}
	return a.DKIM == AuthPass || a.SPF == AuthPass
}

// Suspicious reports an ACTIVE failure — something asserted and disproved,
// as opposed to merely unchecked. Worth surfacing louder than a missing
// verdict, because it is the shape a forged sender takes.
func (a InboundAuth) Suspicious() bool {
	return a.DMARC == AuthFail || a.DKIM == AuthFail || a.SPF == AuthFail
}

// authMethodRe pulls "spf=pass", "dkim = fail", "dmarc=none" out of an
// RFC 8601 Authentication-Results header.
var authMethodRe = regexp.MustCompile(`(?i)\b(spf|dkim|dmarc)\s*=\s*([a-z]+)`)

func parseAuthResult(v string) AuthResult {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "pass":
		return AuthPass
	case "fail", "hardfail", "permerror":
		return AuthFail
	case "none", "neutral", "softfail", "policy", "temperror":
		return AuthNeutral
	default:
		return AuthUnknown
	}
}

// ParseAuthResults reads the standard header first and falls back to the
// per-vendor fields.
//
// The standard header is preferred because it is the one thing every
// receiving MTA writes the same way; the vendor fields exist because not
// every provider forwards it.
func ParseAuthResults(header string, vendor map[string]string) InboundAuth {
	out := InboundAuth{SPF: AuthUnknown, DKIM: AuthUnknown, DMARC: AuthUnknown, Raw: strings.TrimSpace(header)}

	for _, m := range authMethodRe.FindAllStringSubmatch(header, -1) {
		result := parseAuthResult(m[2])
		switch strings.ToLower(m[1]) {
		case "spf":
			out.SPF = result
		case "dkim":
			out.DKIM = result
		case "dmarc":
			out.DMARC = result
		}
	}

	// Vendor fields fill only what the header did not answer, so a real
	// Authentication-Results always wins over a vendor's summary.
	for key, value := range vendor {
		result := parseAuthResult(value)
		if result == AuthUnknown {
			continue
		}
		k := strings.ToLower(key)
		switch {
		case out.SPF == AuthUnknown && strings.Contains(k, "spf"):
			out.SPF = result
		case out.DKIM == AuthUnknown && strings.Contains(k, "dkim"):
			out.DKIM = result
		case out.DMARC == AuthUnknown && strings.Contains(k, "dmarc"):
			out.DMARC = result
		}
	}
	return out
}
