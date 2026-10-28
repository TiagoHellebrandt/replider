// Package email implements domain.MessageProvider for the email channel
// (US-M003/M004).
//
// Not to be confused with internal/adapters/email, which sends OUR
// transactional mail (verification, receipts) from OUR domain. This one
// sends the CUSTOMER's outbound from the CUSTOMER's domain, with the
// customer's own SMTP credentials — the reputational blast radius of a bad
// send lands on them, which is why the config is per channel and never
// shared.
//
// Two things make email different from every other channel here:
//
//   - It has a SUBJECT and a THREAD. WhatsApp/Telegram messages are a flat
//     stream; an email reply must carry In-Reply-To/References or the
//     recipient's client files it as a new conversation. Those live in
//     Message.Metadata, so no schema change was needed.
//   - It is NOT bubble-split. The other adapters break a long reply into
//     several messages because that reads naturally in a chat app; sending
//     three emails in a row reads as a malfunction.
package email

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/oklog/ulid/v2"
	"log/slog"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/replider/api/internal/adapters/httpsafe"
	"github.com/replider/api/internal/core/domain"
)

const (
	smtpTimeout = 20 * time.Second
	// Subject lines longer than this are truncated: mail clients cut them
	// anyway, and an unbounded subject is an easy way to blow the header.
	maxSubjectLen = 200
)

// Metadata keys the pipeline uses to carry email-specific fields. Kept as
// constants because the inbound parser (US-M005) and the approach sender
// both read them.
const (
	MetaSubject = "subject"
	// MetaTemplateID and MetaTemplateVersion say which reusable text this
	// message came from, so the send can be counted against the exact words
	// that earned the reply.
	MetaTemplateID      = "template_id"
	MetaTemplateVersion = "template_version"
	MetaInReplyTo       = "in_reply_to"
	MetaReferences      = "references"
	MetaMessageID       = "message_id"
)

// defaultSubject is used when the caller supplies none. An empty subject is
// a strong spam signal, so we never send one.
const defaultSubject = "Mensagem de %s"

// Adapter sends mail through the channel's own SMTP credentials.
type Adapter struct {
	// newTransport is the seam tests replace. Production dials real SMTP;
	// tests capture the message without a server.
	newTransport func(cfg smtpConfig) transport

	// Tracking. All optional: an empty base URL means links go out untouched
	// and no pixel is inserted, which is the correct behaviour for an account
	// that has not set up a tracking domain.
	trackingBaseURL string
	trackingSecret  string
	// templateStats counts sends per template version. Nil means the numbers
	// are simply not kept — never a reason to fail a send.
	templateStats domain.EmailTemplateRepository
	// tokens mints the short-lived access token for a channel connected by
	// consent instead of by password. Nil means the platform has no Google
	// client configured, which is an operator problem — an OAuth channel then
	// refuses to send rather than falling back to an empty password, which
	// would read to the customer as a rejected mailbox.
	tokens domain.EmailOAuthTokenSource
}

// SetOAuthTokens wires the token source. Optional setter so the constructor
// and its test call sites stay untouched.
func (a *Adapter) SetOAuthTokens(t domain.EmailOAuthTokenSource) { a.tokens = t }

// tokenForgetter is the optional half of a token source that caches:
// *GoogleMailOAuth has it, a test fake need not.
type tokenForgetter interface {
	Forget(channelID string)
}

// forgetCachedToken drops a channel's cached access token after the provider
// refused it, so the next attempt mints a fresh one. A source that does not
// cache has nothing to forget.
func forgetCachedToken(src domain.EmailOAuthTokenSource, channelID string) {
	if f, ok := src.(tokenForgetter); ok && f != nil {
		f.Forget(channelID)
	}
}

// SetTracking wires the tracking domain and the signing secret.
func (a *Adapter) SetTracking(baseURL, secret string) {
	a.trackingBaseURL, a.trackingSecret = baseURL, secret
}

// SetTemplateStats wires the per-version counters.
func (a *Adapter) SetTemplateStats(r domain.EmailTemplateRepository) { a.templateStats = r }

var _ domain.MessageProvider = (*Adapter)(nil)

func NewAdapter() *Adapter {
	return &Adapter{newTransport: dialSMTP}
}

// WithTransport overrides the transport factory (tests).
func (a *Adapter) WithTransport(f func(cfg smtpConfig) transport) *Adapter {
	a.newTransport = f
	return a
}

// transport is the minimal SMTP surface the adapter needs.
type transport interface {
	Send(ctx context.Context, from string, to []string, raw []byte) error
}

type smtpConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	// AccessToken, when set, replaces Password and switches the mechanism to
	// XOAUTH2. The two are mutually exclusive: a channel connected by consent
	// has no password to store, which is the point.
	AccessToken string
	FromAddress string
	FromName    string
	ReplyTo     string
}

var (
	ErrChannelNotEmail    = &domain.AppError{Code: "CHANNEL_NOT_EMAIL", Message: "channel is not an email channel"}
	ErrEmailNotConfigured = &domain.AppError{Code: "CHANNEL_MISSING_CONFIG", Message: "email channel is missing SMTP configuration"}
	ErrEmailNoWebhook     = &domain.AppError{Code: "CHANNEL_NO_WEBHOOK", Message: "email inbound arrives through the provider webhook route, not this adapter"}
)

func configFrom(channel *domain.Channel) (smtpConfig, error) {
	if channel == nil || channel.Type != "email" {
		return smtpConfig{}, ErrChannelNotEmail
	}
	raw, err := json.Marshal(channel.Config)
	if err != nil {
		return smtpConfig{}, ErrEmailNotConfigured
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return smtpConfig{}, ErrEmailNotConfigured
	}
	get := func(k string) string {
		s, _ := m[k].(string)
		return strings.TrimSpace(s)
	}
	cfg := smtpConfig{
		Host:        get("smtp_host"),
		Port:        get("smtp_port"),
		Username:    get("smtp_username"),
		Password:    get("smtp_password"),
		FromAddress: get("from_address"),
		FromName:    get("from_name"),
		ReplyTo:     get("reply_to"),
	}
	if domain.IsOAuthEmail(channel) {
		// Neither host nor username is asked of the customer on this path —
		// the address IS the account, and the endpoint is Google's. A typo in
		// either is indistinguishable from a rejected grant, so there is
		// nothing to type.
		cfg.Host = domain.GoogleSMTPHost
		cfg.Port = domain.GoogleSMTPPort
		cfg.Username = cfg.FromAddress
		cfg.Password = ""
	}
	if cfg.Port == "" {
		// 587 (submission + STARTTLS) is what every managed provider
		// documents; 25 is blocked outbound by most hosts.
		cfg.Port = "587"
	}
	if cfg.Host == "" || cfg.FromAddress == "" {
		return smtpConfig{}, ErrEmailNotConfigured
	}
	return cfg, nil
}

// SendMessage delivers one email. `to` is the recipient address; threading
// headers ride on msg.Metadata so a reply lands inside the same thread in
// the recipient's client.
func (a *Adapter) SendMessage(ctx context.Context, channel *domain.Channel, to string, msg *domain.Message) error {
	cfg, err := configFrom(channel)
	if err != nil {
		return err
	}
	// Resolved per send, not per channel: an access token lives about an hour
	// and a channel outlives thousands of them.
	if domain.IsOAuthEmail(channel) {
		if a.tokens == nil {
			return domain.ErrEmailOAuthUnavailable
		}
		token, tokErr := a.tokens.AccessToken(ctx, channel)
		if tokErr != nil {
			return tokErr
		}
		cfg.AccessToken = token
	}
	if !domain.IsEmailAddress(to) {
		return &domain.AppError{Code: "VALIDATION_ERROR", Message: "recipient is not a valid email address"}
	}
	body := ""
	if msg != nil && msg.Content != nil {
		body = *msg.Content
	}
	if strings.TrimSpace(body) == "" {
		// An empty email is worse than no email: it burns a send and looks
		// broken to the recipient.
		return &domain.AppError{Code: "EMPTY_MESSAGE", Message: "email body is empty"}
	}

	meta := metadataMap(msg)
	subject := strings.TrimSpace(stringFrom(meta, MetaSubject))
	if subject == "" {
		name := cfg.FromName
		if name == "" {
			name = channel.Name
		}
		subject = fmt.Sprintf(defaultSubject, name)
	}
	if len(subject) > maxSubjectLen {
		subject = strings.TrimSpace(subject[:maxSubjectLen])
	}

	messageID := newMessageID(domain.EmailDomain(cfg.FromAddress))

	// Tracking, when the body is HTML and a tracking domain is configured.
	//
	// Cold outbound is plain text ON PURPOSE — see buildMIME — and it stays
	// untracked, because both halves of tracking would damage it. An open
	// pixel needs HTML at all, and a wrapped link in plain text shows the
	// reader "track.x.com/t/c/eyJ..." instead of the real destination, which
	// in a message from a stranger reads as phishing. Trading reply rate for
	// an open rate that Apple already inflates is a bad trade.
	tpl := domain.TrackToken{
		MessageID:  messageID,
		TemplateID: stringFrom(meta, MetaTemplateID),
		Version:    intFrom(meta, MetaTemplateVersion),
	}
	isHTML := looksLikeHTML(body)
	if a.trackingBaseURL != "" && a.trackingSecret != "" && isHTML {
		body = domain.WrapLinks(body, a.trackingSecret, a.trackingBaseURL, tpl)
		body += domain.OpenPixelTag(a.trackingSecret, a.trackingBaseURL, tpl)
	}

	// The picture somebody asked for. Only on a REPLY — a message threaded
	// onto one already in the mailbox — because that is where "manda uma foto
	// do equipamento" gets asked. A first-touch message to a stranger carrying
	// a file is a spam filter's easiest decision, and the deliverability of
	// the first message is the whole funnel.
	var files []attachment
	inReplyTo := stringFrom(meta, MetaInReplyTo)
	if msg != nil && msg.MediaURL != nil && inReplyTo != "" {
		if f := fetchAttachment(ctx, *msg.MediaURL); f != nil {
			files = append(files, *f)
		} else {
			// The message still goes: a reply that never arrives is worse than
			// one missing its attachment. But say so — a picture the sender
			// believes they sent is the kind of gap nobody discovers.
			slog.WarnContext(ctx, "email: attachment could not be fetched — sending without it",
				"channel_id", channel.ID)
		}
	}

	raw := buildMIME(cfg, to, subject, body, mimeThreading{
		messageID:  messageID,
		inReplyTo:  inReplyTo,
		references: stringFrom(meta, MetaReferences),
	}, files...)

	if err := a.newTransport(cfg).Send(ctx, cfg.FromAddress, []string{to}, raw); err != nil {
		// A refused XOAUTH2 login means the access token we hold is dead —
		// revoked, or minted from a grant that has since been replaced.
		// Dropping it from the cache makes the NEXT send refresh from the
		// stored grant instead of failing for the rest of the hour on a
		// token we already know is bad.
		if errors.Is(err, domain.ErrEmailOAuthNotConnected) {
			forgetCachedToken(a.tokens, channel.ID)
		}
		return fmt.Errorf("email send: %w", err)
	}

	// Counted AFTER the send succeeded: a send that failed is not a send, and
	// counting it would make every template's reply rate look worse than it
	// is. Best effort — the message is already out, and a failed counter must
	// not surface as a send error.
	if a.templateStats != nil && tpl.TemplateID != "" {
		if err := a.templateStats.RecordSend(ctx, tpl.TemplateID, tpl.Version); err != nil {
			slog.WarnContext(ctx, "email: could not count the send",
				"template_id", tpl.TemplateID, "error", err)
		}
	}

	// Record the id we generated so the inbound side can stitch the reply
	// back onto this conversation (US-M005). Metadata is an `interface{}` on
	// the entity (it round-trips as JSONB), so write through a map copy
	// rather than assuming the caller already gave us one.
	out := metadataMap(msg)
	out[MetaMessageID] = messageID
	out[MetaSubject] = subject
	msg.Metadata = out
	return nil
}

// SendMessageAndGetID satisfies the outbound-approach sender port: it sends
// and returns the provider-side id. For email that id is the Message-ID we
// mint — which is also what a later reply threads onto, so the two uses are
// the same value by construction.
func (a *Adapter) SendMessageAndGetID(ctx context.Context, channel *domain.Channel, to string, msg *domain.Message) (string, error) {
	if err := a.SendMessage(ctx, channel, to, msg); err != nil {
		return "", err
	}
	return stringFrom(metadataMap(msg), MetaMessageID), nil
}

// HandleWebhook always rejects: inbound email arrives on the per-channel
// webhook route parsed by the inbound handler (US-M005), not here. Mirrors
// the internal-channel adapter's contract.
func (a *Adapter) HandleWebhook(_ context.Context, _ *domain.Channel, _ []byte) error {
	return ErrEmailNoWebhook
}

// metadataMap normalizes Message.Metadata (an `interface{}` that arrives as
// a map, as raw JSON from the DB, or nil) into something readable.
func metadataMap(msg *domain.Message) map[string]any {
	if msg == nil || msg.Metadata == nil {
		return map[string]any{}
	}
	switch v := msg.Metadata.(type) {
	case map[string]any:
		return v
	case json.RawMessage:
		m := map[string]any{}
		_ = json.Unmarshal(v, &m)
		return m
	case []byte:
		m := map[string]any{}
		_ = json.Unmarshal(v, &m)
		return m
	case string:
		m := map[string]any{}
		_ = json.Unmarshal([]byte(v), &m)
		return m
	default:
		// Anything else round-trips through JSON so a struct payload still
		// yields its fields instead of silently becoming empty.
		raw, err := json.Marshal(v)
		if err != nil {
			return map[string]any{}
		}
		m := map[string]any{}
		_ = json.Unmarshal(raw, &m)
		return m
	}
}

func stringFrom(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

type mimeThreading struct {
	messageID  string
	inReplyTo  string
	references string
}

// newMessageID mints an RFC 5322 Message-ID rooted at the sender's own
// domain. Using the sender domain (not ours) matters: mail systems compare
// it against the envelope for consistency, and a mismatch is a spam signal.
func newMessageID(domainPart string) string {
	if domainPart == "" {
		domainPart = "replider.local"
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// Time-based fallback: uniqueness matters, unpredictability does not.
		return fmt.Sprintf("<%d@%s>", time.Now().UnixNano(), domainPart)
	}
	return fmt.Sprintf("<%s@%s>", base64.RawURLEncoding.EncodeToString(buf), domainPart)
}

// buildMIME assembles a plain-text email.
//
// Plain text on purpose: cold outbound that looks hand-written outperforms
// HTML (the deliverability research in docs/gaps-to-top3-sdr.md puts a
// 25-100 word plain body at the top), and an HTML body is one more thing a
// spam filter can object to.
func buildMIME(cfg smtpConfig, to, subject, body string, th mimeThreading, files ...attachment) []byte {
	var b strings.Builder

	from := cfg.FromAddress
	if cfg.FromName != "" {
		from = (&mail.Address{Name: cfg.FromName, Address: cfg.FromAddress}).String()
	}

	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	if cfg.ReplyTo != "" {
		b.WriteString("Reply-To: " + cfg.ReplyTo + "\r\n")
	}
	// Encoded-word so accented Portuguese subjects survive transport.
	b.WriteString("Subject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\n")
	b.WriteString("Message-ID: " + th.messageID + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	if th.inReplyTo != "" {
		b.WriteString("In-Reply-To: " + th.inReplyTo + "\r\n")
		// References accumulates the whole chain; without it long threads
		// fragment in Gmail.
		refs := strings.TrimSpace(th.references)
		if refs == "" {
			refs = th.inReplyTo
		} else if !strings.Contains(refs, th.inReplyTo) {
			refs = refs + " " + th.inReplyTo
		}
		b.WriteString("References: " + refs + "\r\n")
	}
	b.WriteString("MIME-Version: 1.0\r\n")

	// The content type follows the BODY, not a preference. Declaring
	// text/plain for an HTML body delivers the markup as visible source —
	// tags, tracking pixel and all — which is worse than either choice made
	// on purpose. Plain text stays the default because that is what an
	// empty/hand-written body is.
	bodyType := "text/plain; charset=UTF-8"
	if looksLikeHTML(body) {
		bodyType = "text/html; charset=UTF-8"
	}
	// Normalize to CRLF: a bare LF in the body breaks strict SMTP servers.
	crlfBody := strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n")

	if len(files) == 0 {
		b.WriteString("Content-Type: " + bodyType + "\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		b.WriteString("\r\n")
		b.WriteString(crlfBody)
		b.WriteString("\r\n")
		return []byte(b.String())
	}

	// multipart/mixed, and ONLY when something is attached: wrapping every
	// message in a multipart envelope would change the shape of the cold
	// outbound that currently delivers, to buy nothing.
	boundary := newMIMEBoundary()
	b.WriteString("Content-Type: multipart/mixed; boundary=\"" + boundary + "\"\r\n")
	b.WriteString("\r\n")
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: " + bodyType + "\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	b.WriteString(crlfBody)
	b.WriteString("\r\n")
	for _, f := range files {
		writeAttachment(&b, boundary, f)
	}
	b.WriteString("\r\n--" + boundary + "--\r\n")

	return []byte(b.String())
}

// newMIMEBoundary is a delimiter that cannot appear inside base64 or a mail
// body. Random rather than fixed: a boundary a body could contain truncates
// the message at that line.
func newMIMEBoundary() string {
	return "replider-" + ulid.Make().String()
}

// smtpTransport is the production transport. Structurally a clone of the
// transactional adapter's dial path (implicit TLS on 465, STARTTLS
// elsewhere, refuse to hand over the password without TLS on a submission
// port) — the security posture must not diverge between the two.
type smtpTransport struct{ cfg smtpConfig }

func dialSMTP(cfg smtpConfig) transport { return &smtpTransport{cfg: cfg} }

func (t *smtpTransport) Send(ctx context.Context, from string, to []string, raw []byte) error {
	cfg := t.cfg
	addr := net.JoinHostPort(cfg.Host, cfg.Port)

	// Customer-named host: the guarded dialer refuses internal addresses
	// at connect time (same rule as the IMAP fetcher and the webhook client).
	dialer := httpsafe.Dialer(smtpTimeout)
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(smtpTimeout))

	if cfg.Port == "465" {
		conn = tls.Client(conn, &tls.Config{ServerName: cfg.Host})
	}

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer func() { _ = client.Close() }()

	if cfg.Port != "465" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		} else if cfg.Port != "1025" && cfg.Port != "25" {
			// Never send the customer's password in the clear.
			return fmt.Errorf("smtp server does not advertise STARTTLS on port %s", cfg.Port)
		}
	}

	switch {
	case cfg.AccessToken != "":
		if err := client.Auth(
			newSMTPXOAuth2(cfg.Username, cfg.AccessToken, cfg.Host)); err != nil {
			// A grant that was revoked or expired past refresh reads as an auth
			// failure here. Naming it is what turns a five-minute-forever
			// silent failure into "connect the account again".
			return fmt.Errorf("%w: %w", domain.ErrEmailOAuthNotConnected, err)
		}
	case cfg.Username != "":
		if err := client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL: %w", err)
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp RCPT: %w", err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		_ = w.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}
	return client.Quit()
}

// looksLikeHTML is deliberately crude: the only question is whether an open
// pixel would render, and a body with a tag in it is one where it would.
func looksLikeHTML(body string) bool {
	lowered := strings.ToLower(body)
	return strings.Contains(lowered, "<html") ||
		strings.Contains(lowered, "<p>") ||
		strings.Contains(lowered, "<br") ||
		strings.Contains(lowered, "<div")
}

// intFrom reads a numeric metadata value, whatever shape JSON handed it back
// in — a round trip through JSONB turns an int into a float64.
func intFrom(meta map[string]any, key string) int {
	switch v := meta[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}
