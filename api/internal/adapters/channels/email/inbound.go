package email

import (
	"encoding/json"
	"html"
	"net/mail"
	"regexp"
	"strings"
)

// Inbound email parsing (US-M005).
//
// Inbound providers (Resend, SendGrid, Mailgun, Postmark) all POST a JSON
// document with the same handful of fields under different names. Rather
// than one adapter per vendor — each a new deploy to support a customer's
// choice — the parser accepts the union of the common spellings and
// normalizes. Adding a vendor becomes adding a key, not a package.

// InboundEmail is the normalized shape the webhook handler consumes.
type InboundEmail struct {
	From       string // bare address, lowercased — the sender identity
	FromName   string
	To         string
	Subject    string
	Body       string // ALWAYS plain text (see sanitizeToText)
	MessageID  string
	InReplyTo  string
	References string
	// ReplyTo is where an answer would GO, which is not always where the
	// message came from. A mismatch is the mechanic of a redirected reply —
	// distinct from InReplyTo, which is a threading header.
	ReplyTo string
	// HadHTMLOnly records that the original carried no text/plain part, so
	// the body above was derived from HTML. Useful when debugging a reply
	// that came out oddly formatted.
	HadHTMLOnly bool

	// HTML is the ORIGINAL markup, untouched. It is never rendered directly:
	// the dashboard shows Body (plain text) and offers "view original", which
	// opens this in a sandboxed frame with remote content blocked. Kept
	// separate from Body precisely so nothing can render it by accident.
	HTML string

	// Attachments are the files that came with the message. Before this they
	// were dropped silently — a reply saying "segue em anexo" arrived with
	// nothing attached and no indication anything was missing.
	Attachments []InboundAttachment

	// Auth is what the receiving server concluded about the sender. Without
	// it a forged From line is indistinguishable from the real clinic.
	Auth InboundAuth

	// ListUnsubscribe reports the header every legitimate bulk sender sets.
	// It is the honest signal that something is a newsletter — matching the
	// word "descadastrar" in prose would file a customer's own request to
	// cancel as bulk mail.
	ListUnsubscribe bool

	// The markers a machine leaves on mail nobody wrote (RFC 3834 and the
	// de-facto vendor headers). Read by IsAutoSubmitted: a vacation notice
	// or a MAILER-DAEMON report must never become a contact, a lead in
	// "responded", or the reason a cadence stops.
	//
	// AutoSubmitted is the raw `Auto-Submitted` value ("auto-replied",
	// "auto-generated", "no", ...). Precedence is the raw `Precedence`
	// value ("bulk", "list", "junk", "auto_reply"). HasAutoReplyHeader
	// records that one of `X-Autoreply` / `X-Autorespond` /
	// `X-Auto-Response-Suppress` was present at all — their values vary by
	// vendor and mean nothing; their presence is the signal.
	AutoSubmitted      string
	Precedence         string
	HasAutoReplyHeader bool
}

// RealAttachments excludes inline message furniture (signature logos,
// tracking pixels), which is what the reader means by "anexo".
func (e InboundEmail) RealAttachments() []InboundAttachment {
	out := make([]InboundAttachment, 0, len(e.Attachments))
	for _, a := range e.Attachments {
		if !a.IsInline() {
			out = append(out, a)
		}
	}
	return out
}

// rawInbound is the union of field spellings across providers. Unknown
// fields are ignored, so a provider that adds one does not break parsing.
type rawInbound struct {
	From    string `json:"from"`
	Sender  string `json:"sender"`
	To      string `json:"to"`
	Subject string `json:"subject"`

	Text     string `json:"text"`
	TextBody string `json:"TextBody"`
	Plain    string `json:"body-plain"`

	HTML     string `json:"html"`
	HTMLBody string `json:"HtmlBody"`
	BodyHTML string `json:"body-html"`

	MessageID  string `json:"message_id"`
	MessageID2 string `json:"MessageID"`
	MessageID3 string `json:"Message-Id"`

	InReplyTo  string `json:"in_reply_to"`
	InReplyTo2 string `json:"In-Reply-To"`

	ReplyTo    string `json:"reply_to"`
	ReplyTo2   string `json:"ReplyTo"`
	References string `json:"references"`
	Reference2 string `json:"References"`

	// Some providers nest the raw headers instead of promoting them.
	Headers map[string]string `json:"headers"`

	Attachments  []rawAttachment `json:"attachments"`
	Attachments2 []rawAttachment `json:"Attachments"`

	ListUnsubscribe string `json:"list_unsubscribe"`

	// Vendor-specific authentication verdicts, used only when the standard
	// Authentication-Results header is absent.
	SPF        string `json:"SPF"`
	SPF2       string `json:"spf"`
	DKIM       string `json:"dkim"`
	DKIM2      string `json:"DKIM"`
	DMARC      string `json:"dmarc"`
	AuthResult string `json:"authentication_results"`
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// ParseInboundWebhook normalizes a provider payload. Returns ok=false when
// the document carries no usable sender — an unattributable message cannot
// become a conversation, and acking it is better than 500-ing at a provider
// that will then retry forever.
func ParseInboundWebhook(body []byte) (InboundEmail, bool) {
	var raw rawInbound
	if err := json.Unmarshal(body, &raw); err != nil {
		return InboundEmail{}, false
	}

	fromRaw := firstNonEmpty(raw.From, raw.Sender, headerLookup(raw.Headers, "from"))
	addr, name := splitAddress(fromRaw)
	if addr == "" {
		return InboundEmail{}, false
	}

	text := firstNonEmpty(raw.Text, raw.TextBody, raw.Plain)
	htmlBody := firstNonEmpty(raw.HTML, raw.HTMLBody, raw.BodyHTML)

	out := InboundEmail{
		From:     addr,
		FromName: name,
		To:       strings.ToLower(strings.TrimSpace(firstNonEmpty(raw.To, headerLookup(raw.Headers, "to")))),
		Subject:  firstNonEmpty(raw.Subject, headerLookup(raw.Headers, "subject")),
		MessageID: firstNonEmpty(raw.MessageID, raw.MessageID2, raw.MessageID3,
			headerLookup(raw.Headers, "message-id")),
		InReplyTo:  firstNonEmpty(raw.InReplyTo, raw.InReplyTo2, headerLookup(raw.Headers, "in-reply-to")),
		References: firstNonEmpty(raw.References, raw.Reference2, headerLookup(raw.Headers, "references")),
	}
	if rt := firstNonEmpty(raw.ReplyTo, raw.ReplyTo2, headerLookup(raw.Headers, "reply-to")); rt != "" {
		out.ReplyTo, _ = splitAddress(rt)
	}

	if text != "" {
		out.Body = normalizeText(text)
	} else if htmlBody != "" {
		out.Body = sanitizeToText(htmlBody)
		out.HadHTMLOnly = true
	}
	out.HTML = htmlBody

	rawAtts := raw.Attachments
	if len(rawAtts) == 0 {
		rawAtts = raw.Attachments2
	}
	for _, ra := range rawAtts {
		if att, ok := ra.normalize(); ok {
			out.Attachments = append(out.Attachments, att)
		}
	}

	out.ListUnsubscribe = firstNonEmpty(
		raw.ListUnsubscribe,
		headerLookup(raw.Headers, "list-unsubscribe"),
		headerLookup(raw.Headers, "list-id"),
	) != ""
	// Providers rarely forward these, and a missing one is fine — empty
	// means "not marked", which is what IsAutoSubmitted needs.
	out.AutoSubmitted = headerLookup(raw.Headers, "auto-submitted")
	out.Precedence = headerLookup(raw.Headers, "precedence")
	out.HasAutoReplyHeader = hasAnyHeader(raw.Headers,
		"x-autoreply", "x-autorespond", "x-auto-response-suppress")

	out.Auth = ParseAuthResults(
		firstNonEmpty(raw.AuthResult, headerLookup(raw.Headers, "authentication-results")),
		map[string]string{
			"spf":   firstNonEmpty(raw.SPF, raw.SPF2),
			"dkim":  firstNonEmpty(raw.DKIM, raw.DKIM2),
			"dmarc": raw.DMARC,
		},
	)
	return out, true
}

func headerLookup(headers map[string]string, key string) string {
	if headers == nil {
		return ""
	}
	for k, v := range headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

// splitAddress accepts both `Nome <a@b.com>` and a bare address.
func splitAddress(raw string) (address, name string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", ""
	}
	if parsed, err := mail.ParseAddress(s); err == nil {
		return strings.ToLower(strings.TrimSpace(parsed.Address)), strings.TrimSpace(parsed.Name)
	}
	// Not RFC-parseable (providers do send malformed values) — fall back to
	// the angle-bracket contents, then to the whole string.
	if i, j := strings.Index(s, "<"), strings.Index(s, ">"); i >= 0 && j > i {
		inner := strings.ToLower(strings.TrimSpace(s[i+1 : j]))
		return inner, strings.TrimSpace(strings.Trim(s[:i], ` "`))
	}
	return strings.ToLower(s), ""
}

// Go's regexp is RE2: no backreferences, so each element gets its own
// pattern instead of one `<(script|style)…</\1>`.
var elementRes = []*regexp.Regexp{
	regexp.MustCompile(`(?is)<script[^>]*>.*?</\s*script\s*>`),
	regexp.MustCompile(`(?is)<style[^>]*>.*?</\s*style\s*>`),
	regexp.MustCompile(`(?is)<head[^>]*>.*?</\s*head\s*>`),
	regexp.MustCompile(`(?is)<title[^>]*>.*?</\s*title\s*>`),
	// An unclosed <script ...> would otherwise leave its body as text.
	regexp.MustCompile(`(?is)<script[^>]*>.*`),
}

var (
	brRe         = regexp.MustCompile(`(?i)<\s*br\s*/?\s*>`)
	blockCloseRe = regexp.MustCompile(`(?i)</\s*(p|div|tr|li|h[1-6]|table|blockquote)\s*>`)
	tagRe        = regexp.MustCompile(`(?s)<[^>]*>`)
	manyBlankRe  = regexp.MustCompile(`\n{3,}`)
	trailingWSRe = regexp.MustCompile(`[ \t]+\n`)
)

// sanitizeToText converts an HTML body to plain text (V4: XSS via inbound
// HTML rendered in the dashboard).
//
// The deliberate choice is CONVERSION, not sanitization. An allowlist-based
// HTML sanitizer has to be right about every tag, attribute, protocol and
// encoding trick forever; stripping to text is right by construction. The
// cost is losing formatting on the minority of replies that are HTML-only —
// worth it, since what the pipeline actually needs is the words: the AI
// reads text, and the dashboard renders text bubbles.
func sanitizeToText(input string) string {
	s := input
	// Drop entire elements whose CONTENT would otherwise survive tag
	// stripping as visible garbage (or as script source).
	for _, re := range elementRes {
		s = re.ReplaceAllString(s, " ")
	}
	// Preserve the line structure a reader depends on before tags go.
	s = brRe.ReplaceAllString(s, "\n")
	s = blockCloseRe.ReplaceAllString(s, "\n")
	s = tagRe.ReplaceAllString(s, "")

	// Decode entities and strip AGAIN, until the text stops changing.
	//
	// Order matters twice over. Decoding BEFORE the first strip would let
	// `&lt;script&gt;` become a real tag after the stripper had already
	// run. Decoding without a SECOND strip leaves the literal text
	// "<script>alert(1)</script>" in the body — harmless in a text widget,
	// but this function promises plain text, and the next renderer to touch
	// it may not be a text widget. Double-encoded payloads
	// (`&amp;lt;script&amp;gt;`) need the loop.
	for i := 0; i < 3; i++ {
		decoded := html.UnescapeString(s)
		if decoded == s {
			break
		}
		s = decoded
		for _, re := range elementRes {
			s = re.ReplaceAllString(s, " ")
		}
		s = tagRe.ReplaceAllString(s, "")
	}
	return normalizeText(s)
}

func normalizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, " ", " ") // nbsp reads as a stray glyph
	s = trailingWSRe.ReplaceAllString(s, "\n")
	s = manyBlankRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// quotedReplyRe matches the common "On <date>, X wrote:" separators (pt-BR
// and en) and the ">"-quoted block that follows.
var quotedReplyRe = regexp.MustCompile(`(?ims)^\s*(-{2,}\s*mensagem original\s*-{2,}|em .{0,80}escreveu:|on .{0,80}wrote:|de:\s.*\nenviada?( em)?:\s)`)

// StripQuotedReply removes the quoted history from a reply.
//
// Without it every reply grows: the AI re-reads the whole thread it already
// wrote, tokens are paid for repeatedly, and the dashboard bubble shows a
// wall of quoted text instead of the one sentence the person actually sent.
func StripQuotedReply(body string) string {
	if loc := quotedReplyRe.FindStringIndex(body); loc != nil {
		body = body[:loc[0]]
	}
	// Trailing ">"-quoted lines (some clients quote with no separator).
	lines := strings.Split(body, "\n")
	end := len(lines)
	for end > 0 {
		trimmed := strings.TrimSpace(lines[end-1])
		if trimmed == "" || strings.HasPrefix(trimmed, ">") {
			end--
			continue
		}
		break
	}
	return normalizeText(strings.Join(lines[:end], "\n"))
}

// MetaReplyTo is the metadata key under which the inbound handler stores the
// customer's Reply-To address (already reduced to the bare address). The
// reply path sends there instead of to From when present and valid.
const MetaReplyTo = "reply_to"

// ThreadMetadata is the JSON stored on the inbound message so a later reply
// can thread onto it (the outbound adapter reads these keys).
func (e InboundEmail) ThreadMetadata() []byte {
	m := map[string]any{}
	if e.Subject != "" {
		m[MetaSubject] = e.Subject
	}
	if e.MessageID != "" {
		m[MetaMessageID] = e.MessageID
	}
	if e.InReplyTo != "" {
		m[MetaInReplyTo] = e.InReplyTo
	}
	if e.References != "" {
		m[MetaReferences] = e.References
	}
	if len(m) == 0 {
		return nil
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return raw
}

// ReplyMetadata builds the metadata an OUTBOUND reply to this email needs so
// it lands in the same thread: In-Reply-To points at this message, and the
// subject gets a single "Re: " prefix (clients dislike "Re: Re: Re:").
func (e InboundEmail) ReplyMetadata() map[string]any {
	m := map[string]any{}
	subject := strings.TrimSpace(e.Subject)
	if subject != "" {
		if !strings.HasPrefix(strings.ToLower(subject), "re:") {
			subject = "Re: " + subject
		}
		m[MetaSubject] = subject
	}
	if e.MessageID != "" {
		m[MetaInReplyTo] = e.MessageID
		refs := strings.TrimSpace(e.References)
		if refs == "" {
			refs = e.MessageID
		} else if !strings.Contains(refs, e.MessageID) {
			refs += " " + e.MessageID
		}
		m[MetaReferences] = refs
	}
	return m
}
