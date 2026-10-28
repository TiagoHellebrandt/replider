package email

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// Bounce detection on the inbound route (§6.8.4).
//
// A delivery failure reaches us in one of two shapes, and BOTH used to be
// mishandled:
//
//   - A PROVIDER EVENT ("type": "email.bounced"). It has no From, so the
//     normal parser rejected it and the handler acked "ignored" — the bounce
//     vanished.
//   - A DSN, a real email from MAILER-DAEMON. Worse: it parsed fine, so it
//     became a conversation with a robot as the contact, and the agent
//     replied to it. Tokens spent answering a mail server.
//
// So this runs BEFORE the ordinary inbound parse, and both shapes have to be
// recognised here or the failure is invisible.

// bounceEvent is the union of provider webhook event spellings.
type bounceEvent struct {
	// The event name. Resend: "email.bounced"/"email.complained".
	// SendGrid/Mailgun/Postmark: "bounce"/"failed"/"spamcomplaint".
	Type       string `json:"type"`
	Event      string `json:"event"`
	EventType  string `json:"event_type"`
	RecordType string `json:"RecordType"`

	// Postmark promotes these to the top level.
	Email       string `json:"Email"`
	Recipient   string `json:"recipient"`
	Description string `json:"Description"`
	Details     string `json:"Details"`
	// Postmark's own hard/soft word.
	PostmarkType string `json:"Type"`

	// Mailgun nests everything one level down.
	EventData struct {
		Event     string `json:"event"`
		Severity  string `json:"severity"`
		Reason    string `json:"reason"`
		Recipient string `json:"recipient"`
		Delivery  struct {
			Message     string `json:"message"`
			Code        int    `json:"code"`
			Description string `json:"description"`
		} `json:"delivery-status"`
		Message struct {
			Headers map[string]string `json:"headers"`
		} `json:"message"`
	} `json:"event-data"`

	// Resend nests under data.
	Data struct {
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Bounce  struct {
			Type       string `json:"type"`
			SubType    string `json:"subType"`
			Message    string `json:"message"`
			Diagnostic string `json:"diagnosticCode"`
		} `json:"bounce"`
		Headers []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
	} `json:"data"`

	// SendGrid posts an ARRAY of these; see ParseBounce.
	SGEmail  string `json:"email"`
	SGReason string `json:"reason"`
	SGStatus string `json:"status"`
	SGType   string `json:"type_"`
}

// bounceEventNames are the event words that mean "this did not arrive".
// Matched on the whole value, not a substring — "email.delivered" must not
// match on "deliver".
var bounceEventNames = map[string]string{
	"email.bounced":    "",
	"email.complained": "complaint",
	"email.failed":     "",
	"bounce":           "",
	"bounced":          "",
	"dropped":          "hard",
	"failed":           "",
	"spamcomplaint":    "complaint",
	"spamreport":       "complaint",
	"complained":       "complaint",
	"complaint":        "complaint",
	// Deliberately NOT here: Postmark's SubscriptionChange. It covers plain
	// unsubscribes as well as complaints, and recording an unsubscribe as
	// "reported us as spam" is a lie the customer would read in the panel.
	// An unsubscribe belongs to the opt-out path.
	"permanent_fail": "hard",
	"temporary_fail": "soft",
}

// ParseBounce recognises a delivery failure in a provider webhook payload.
//
// Returns ok=false for anything else, including a delivery SUCCESS event —
// which is why the event name is matched exactly.
func ParseBounce(body []byte) (domain.EmailBounce, bool) {
	trimmed := strings.TrimSpace(string(body))

	// SendGrid posts a batch. Take the first failure in it; the rest are
	// separate events the provider will also deliver.
	if strings.HasPrefix(trimmed, "[") {
		var batch []json.RawMessage
		if err := json.Unmarshal(body, &batch); err != nil {
			return domain.EmailBounce{}, false
		}
		for _, item := range batch {
			if b, ok := ParseBounce(item); ok {
				return b, true
			}
		}
		return domain.EmailBounce{}, false
	}

	var e bounceEvent
	if err := json.Unmarshal(body, &e); err != nil {
		return domain.EmailBounce{}, false
	}

	name := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		e.Type, e.Event, e.EventType, e.RecordType, e.EventData.Event, e.SGType)))
	hint, isBounce := bounceEventNames[name]
	if !isBounce {
		return domain.EmailBounce{}, false
	}

	recipient := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		e.Email, e.Recipient, e.SGEmail, e.EventData.Recipient, firstOf(e.Data.To))))
	if recipient == "" {
		// A bounce with no recipient cannot suppress anything and cannot be
		// attributed. Reporting it as parsed would only produce a counter
		// nobody can act on.
		return domain.EmailBounce{}, false
	}

	diagnostic := firstNonEmpty(
		e.Data.Bounce.Diagnostic, e.Data.Bounce.Message,
		e.EventData.Delivery.Description, e.EventData.Delivery.Message,
		e.EventData.Reason, e.SGReason, e.SGStatus, e.Details, e.Description)

	// The provider's own hard/soft word, when it has one. Resend puts it on
	// bounce.type, Mailgun on severity, Postmark on Type.
	if hint == "" {
		hint = firstNonEmpty(e.Data.Bounce.Type, e.Data.Bounce.SubType,
			e.EventData.Severity, e.PostmarkType)
	}

	return domain.EmailBounce{
		Kind:              domain.ClassifyBounce(hint, diagnostic),
		Recipient:         recipient,
		Diagnostic:        strings.TrimSpace(diagnostic),
		OriginalMessageID: bounceOriginalID(e),
	}, true
}

func firstOf(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

// bounceOriginalID digs out the Message-ID of the message that failed.
//
// Best effort by nature: only some providers echo the original headers. When
// it comes back empty the use case falls back to the recipient, which is why
// nothing here returns an error.
func bounceOriginalID(e bounceEvent) string {
	for _, h := range e.Data.Headers {
		if strings.EqualFold(h.Name, "message-id") {
			return strings.TrimSpace(h.Value)
		}
	}
	if id := headerLookup(e.EventData.Message.Headers, "message-id"); id != "" {
		return normalizeMessageID(id)
	}
	return ""
}

// normalizeMessageID restores the angle brackets some providers strip. Our
// own sent ids carry them, so a stripped one would never match.
func normalizeMessageID(id string) string {
	s := strings.TrimSpace(id)
	if s == "" || strings.HasPrefix(s, "<") {
		return s
	}
	return "<" + s + ">"
}

// ── DSN (a bounce that arrives as an ordinary email) ──────────────────────

// mailerDaemonRe matches the senders a mail system uses to report failures.
var mailerDaemonRe = regexp.MustCompile(`(?i)^(mailer-daemon|postmaster|no-?reply)@`)

// dsnSubjectRe matches the subjects those reports carry.
var dsnSubjectRe = regexp.MustCompile(`(?i)(undelivered mail|delivery status notification|returned mail|mail delivery failed|undeliverable|falha na entrega|não foi entregue|delivery has failed)`)

var (
	dsnFinalRecipientRe = regexp.MustCompile(`(?i)final-recipient:\s*[a-z0-9-]+;\s*(\S+)`)
	dsnOrigRecipientRe  = regexp.MustCompile(`(?i)original-recipient:\s*[a-z0-9-]+;\s*(\S+)`)
	dsnActionRe         = regexp.MustCompile(`(?i)action:\s*(\w+)`)
	dsnStatusRe         = regexp.MustCompile(`(?i)status:\s*(\d\.\d{1,3}\.\d{1,3})`)
	dsnDiagnosticRe     = regexp.MustCompile(`(?i)diagnostic-code:\s*[a-z0-9-]+;\s*([^\r\n]+)`)
	dsnOriginalIDRe     = regexp.MustCompile(`(?i)(?:original-)?message-id:\s*(<[^>]+>)`)
)

// LooksLikeDSN reports a parsed inbound that is really a delivery report.
//
// Checked on the SENDER and the SUBJECT together, because either alone is
// wrong: plenty of legitimate senders use noreply@, and a person can write
// "delivery failed" in an ordinary reply. A report body with a
// Final-Recipient line settles it either way.
func LooksLikeDSN(e InboundEmail) bool {
	fromDaemon := mailerDaemonRe.MatchString(e.From)
	subjectSays := dsnSubjectRe.MatchString(e.Subject)
	hasReport := dsnFinalRecipientRe.MatchString(e.Body) ||
		dsnOrigRecipientRe.MatchString(e.Body)

	if fromDaemon && (subjectSays || hasReport) {
		return true
	}
	// A machine-readable report is proof regardless of who sent it: no
	// human writes "Final-Recipient: rfc822; x@y.com" by hand.
	return hasReport && dsnStatusRe.MatchString(e.Body)
}

// ── Auto-replies (mail a machine sent on somebody's behalf) ───────────────

// autoPrecedenceValues are the Precedence words that mean "not a person
// writing to you": list traffic, bulk mail, and the two spellings vacation
// responders use.
var autoPrecedenceValues = map[string]bool{
	"bulk": true, "auto_reply": true, "junk": true, "list": true,
}

// IsAutoSubmitted reports mail that was generated, not written: an
// out-of-office, a "we received your message" acknowledgement, a list
// digest.
//
// It is NOT a bounce — the address works, and nothing should be suppressed
// — but it is not a reply either. Stored, it becomes a conversation with the
// autoresponder, a lead in "responded", and the reason a cadence stops:
// three wrong facts from one message nobody wrote. So it is dropped.
//
// The rules follow RFC 3834 plus what the big vendors actually stamp:
//
//   - `Auto-Submitted` present with any value but "no" (Gmail, Outlook,
//     Zimbra and everything RFC-compliant).
//   - Any of `X-Autoreply` / `X-Autorespond` / `X-Auto-Response-Suppress`
//     present at all (older Exchange, cPanel, assorted PHP responders).
//   - `Precedence` in {bulk, auto_reply, junk, list}.
//
// Case-insensitive throughout; "Auto-Replied" and "AUTO-REPLIED" both occur
// in the wild.
func IsAutoSubmitted(e InboundEmail) bool {
	if v := strings.ToLower(strings.TrimSpace(e.AutoSubmitted)); v != "" && v != "no" {
		// "auto-replied; owner-token=..." carries parameters; the first
		// token is the verdict.
		return true
	}
	if e.HasAutoReplyHeader {
		return true
	}
	return autoPrecedenceValues[strings.ToLower(strings.TrimSpace(e.Precedence))]
}

// hasAnyHeader reports whether any of the named headers is present in a
// provider's forwarded header map, regardless of key case.
func hasAnyHeader(headers map[string]string, names ...string) bool {
	for _, name := range names {
		if headerLookup(headers, name) != "" {
			return true
		}
	}
	return false
}

// ParseDSN extracts the failure from a delivery report email.
func ParseDSN(e InboundEmail) (domain.EmailBounce, bool) {
	body := e.Body
	if body == "" {
		body = e.HTML
	}

	recipient := ""
	if m := dsnFinalRecipientRe.FindStringSubmatch(body); m != nil {
		recipient = m[1]
	} else if m := dsnOrigRecipientRe.FindStringSubmatch(body); m != nil {
		recipient = m[1]
	}
	recipient = strings.ToLower(strings.Trim(strings.TrimSpace(recipient), "<>;"))
	if !domain.IsEmailAddress(recipient) {
		// Without the failed address there is nothing to suppress. Better to
		// let it through as an ordinary (weird) message than to suppress the
		// mail server itself.
		return domain.EmailBounce{}, false
	}

	diagnostic := ""
	if m := dsnDiagnosticRe.FindStringSubmatch(body); m != nil {
		diagnostic = strings.TrimSpace(m[1])
	}
	// The enhanced status code is the authoritative verdict; prepend it so
	// ClassifyBounce sees it even when the diagnostic line is missing.
	if m := dsnStatusRe.FindStringSubmatch(body); m != nil {
		diagnostic = strings.TrimSpace(m[1] + " " + diagnostic)
	}

	hint := ""
	if m := dsnActionRe.FindStringSubmatch(body); m != nil {
		switch strings.ToLower(m[1]) {
		case "failed":
			// "failed" alone does not say permanent — the status code does.
			hint = ""
		case "delayed":
			hint = "soft"
		}
	}

	original := ""
	if m := dsnOriginalIDRe.FindStringSubmatch(body); m != nil {
		original = strings.TrimSpace(m[1])
	}
	if original == "" {
		// The report is a reply to the message that failed, so the threading
		// header points at it when the body does not.
		original = strings.TrimSpace(e.InReplyTo)
	}

	return domain.EmailBounce{
		Kind:              domain.ClassifyBounce(hint, diagnostic),
		Recipient:         recipient,
		Diagnostic:        strings.TrimSpace(diagnostic),
		OriginalMessageID: original,
	}, true
}
