package email

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"html/template"
	"log/slog"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/replider/api/internal/adapters/httpsafe"
	"github.com/replider/api/internal/core/domain"
)

// EmailService defines the interface for sending emails. Every method
// is short-lived (returns inside the handler's request context) so the
// adapter MUST honor a timeout — see the dial deadline in send().
//
// Every customer-facing composer takes a trailing `locale` resolved
// from the recipient's account (domain.EmailLocaleForCurrency — USD
// accounts get English copy, US-018 review fixes). Empty string falls
// back to pt-BR via resolveLocale.
type EmailService interface {
	SendPasswordResetEmail(to string, resetToken string, appBaseURL, locale string) error
	SendVerificationEmail(to string, verificationToken string, appBaseURL, locale string) error
	SendWelcomeEmail(to, userName, appURL, locale string) error
	SendUsageWarningEmail(to, userName string, usagePercent int, planName string, upgradeURL, locale string) error
	SendUsageLimitEmail(to, userName string, planName string, upgradeURL, locale string) error
	SendPaymentFailedEmail(to, userName, planName, retryURL, locale string) error
	// SendPrebillingEmail warns the account admin ~72h before the next
	// subscription charge (US-124) to reduce involuntary churn / surprise
	// renewals. daysUntil drives the "renews in X" phrasing; locale picks
	// pt-BR vs en copy (empty → pt-BR). manageURL deep-links to the
	// in-app "Manage subscription" flow.
	SendPrebillingEmail(to, userName, planName string, daysUntil int, manageURL, locale string) error
	SendTeamInviteEmail(to string, inviterName string, resetToken string, appBaseURL, locale string) error
	// SendReplyReminderEmail is fired by the reply-reminder worker
	// after the configured threshold elapses without a human reply.
	// `minutesWaiting` is the elapsed wait surfaced in the body so the
	// attendant gets the urgency in a glance.
	SendReplyReminderEmail(to, attendantName, contactName, conversationURL string, minutesWaiting int) error
	// SendTestEmail is the self-service SMTP health check exposed on
	// Settings → Conformidade. It bypasses the stub fallback (returns
	// ErrSMTPNotConfigured when host/port empty) so admins see a real
	// failure instead of a silent log line.
	SendTestEmail(to, userName string) error
}

// ErrSMTPNotConfigured is returned by SendTestEmail when the SMTP adapter
// has no host/port set. The stubbed-log fallback used by regular sends is
// deliberately disabled for the test path — the whole point is to know
// whether emails actually leave the server.
var ErrSMTPNotConfigured = fmt.Errorf("smtp is not configured")

// smtpTimeout caps every step of the SMTP dialog (dial, TLS handshake,
// AUTH, DATA) so a hung remote host can't block the request goroutine
// past Cloudflare's edge timeout.
//
// Why 8s: the Flutter Dio client uses a 10s receiveTimeout. Anything
// north of that surfaces as a "request cancelled" on the dashboard,
// which is exactly the bug Tiago saw on /admin/smtp/test before this
// timeout existed — the previous implementation called net/smtp.SendMail
// directly, which has no deadline at all.
const smtpTimeout = 8 * time.Second

// SMTPAdapter sends emails using SMTP. Plain-text fields are kept (not
// derived from the URL) so the embedded slog stub can mirror what
// real SMTP would have shipped without re-running the template.
type SMTPAdapter struct {
	host       string
	port       string
	username   string
	password   string
	from       string
	appBaseURL string // optional — used to embed the logo in branded emails
}

func NewSMTPAdapter(host, port, username, password, from string) *SMTPAdapter {
	return &SMTPAdapter{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

// SetAppBaseURL injects the public app URL so email templates can pull
// the logo asset off the same origin. Optional — when empty the templates
// degrade gracefully and just hide the logo.
func (a *SMTPAdapter) SetAppBaseURL(url string) { a.appBaseURL = url }

func (a *SMTPAdapter) configured() bool {
	return a.host != "" && a.port != ""
}

// brand returns the shared template data with branding fields filled
// in for the requested locale. Per-call composers extend this with the
// email-specific intro / CTA.
func (a *SMTPAdapter) brand(loc Locale) emailData {
	d := brandDefaults(a.appBaseURL)
	d.Locale = loc
	d.FooterTagline = tr(loc, "footer_tagline")
	return d
}

// firstName returns the first whitespace-delimited token of name, or
// "Admin" / "Olá" defaults when name is empty.
func firstName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if i := strings.IndexAny(name, " \t"); i > 0 {
		return name[:i]
	}
	return name
}

// greet returns "Olá, João!" / "Hi, João!" — the empty-name fallback
// drops the comma so the line stays grammatical.
func greet(loc Locale, name string) string {
	first := firstName(name)
	if first == "" {
		if loc == LocaleEn {
			return "Hi!"
		}
		return "Olá!"
	}
	if loc == LocaleEn {
		return "Hi, " + escapeHTML(first) + "!"
	}
	return "Olá, " + escapeHTML(first) + "!"
}

// send is the single SMTP send path used by every helper above. It:
//  1. Builds a multipart/alternative MIME message (text + html)
//  2. Dials with a hard timeout
//  3. Negotiates STARTTLS on submission ports (587/2525), implicit
//     TLS on 465
//  4. Authenticates and ships
//
// Returns ErrSMTPNotConfigured up the stack when the adapter is
// stubbed; logs and silently returns nil for non-test sends so a
// missing dev SMTP config doesn't break sign-up.
func (a *SMTPAdapter) send(ctx context.Context, to, subject, htmlBody, textBody string, isTest bool) error {
	if !a.configured() {
		if isTest {
			return ErrSMTPNotConfigured
		}
		slog.Info("email stub: would send", "to", to, "subject", subject)
		return nil
	}

	msg, err := buildMultipart(a.from, to, subject, htmlBody, textBody)
	if err != nil {
		return fmt.Errorf("build mime: %w", err)
	}

	addr := net.JoinHostPort(a.host, a.port)

	// Same guarded dialer as the customer-facing mail paths: the policy
	// (private targets allowed outside production only) is process-wide,
	// so a dev MailHog on localhost keeps working.
	dialer := httpsafe.Dialer(smtpTimeout)
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	// Hard ceiling — guards against a peer that accepts the TCP
	// handshake but stalls during the SMTP banner / STARTTLS upgrade.
	deadline := time.Now().Add(smtpTimeout)
	_ = conn.SetDeadline(deadline)

	// Implicit TLS on 465; STARTTLS on 587/2525/25 after the banner.
	if a.port == "465" {
		conn = tls.Client(conn, &tls.Config{ServerName: a.host})
	}

	client, err := smtp.NewClient(conn, a.host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer func() { _ = client.Close() }()

	if a.port != "465" {
		// STARTTLS — required by Resend (587). Ignore unsupported
		// errors only for non-prod localhost ports where TLS may not
		// be configured.
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: a.host}); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		} else if a.port != "1025" && a.port != "25" {
			// Submission ports must offer STARTTLS or we refuse to
			// hand over the password.
			return fmt.Errorf("smtp server does not advertise STARTTLS on port %s", a.port)
		}
	}

	if a.username != "" {
		auth := smtp.PlainAuth("", a.username, a.password, a.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(a.from); err != nil {
		return fmt.Errorf("smtp MAIL: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp RCPT: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		_ = w.Close()
		return fmt.Errorf("smtp body write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp DATA close: %w", err)
	}
	if err := client.Quit(); err != nil {
		// QUIT errors are cosmetic — the server already accepted the
		// message at this point. Log at DEBUG and move on instead of
		// reporting a misleading send failure to the caller.
		slog.Debug("smtp QUIT non-fatal", "error", err)
	}
	return nil
}

// buildMultipart builds an RFC 2822 message with the structure:
//
//	multipart/related
//	  ├── multipart/alternative
//	  │     ├── text/plain
//	  │     └── text/html  (references cid:<logoCID> for the brand logo)
//	  └── image/png         (inline, Content-ID: <logoCID>)
//
// The outer multipart/related ties the HTML body to the embedded logo
// via Content-ID, so the email renders self-contained — no cross-
// origin fetch, no broken-image fallback when an email client proxies
// images and the originating host is unreachable. multipart/alternative
// stays nested inside so HTML-stripping clients still see the plain-
// text version. Subject is RFC 2047 encoded so accented characters
// (à, ã, é) survive Gmail / Apple Mail.
func buildMultipart(from, to, subject, htmlBody, textBody string) (string, error) {
	if textBody == "" {
		// Cheap fallback: strip tags from HTML so we never ship an
		// empty text/plain part. Not perfect but better than nothing
		// for clients that always show the first non-empty part.
		textBody = stripTags(htmlBody)
	}
	relatedBoundary := "replider-related-" + randHex(12)
	altBoundary := "replider-alt-" + randHex(12)
	encodedSubject := mime.QEncoding.Encode("UTF-8", subject)

	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + encodedSubject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/related; boundary=\"" + relatedBoundary + "\"; type=\"multipart/alternative\"\r\n")
	b.WriteString("\r\n")

	// ── Inner multipart/alternative ───────────────────────────────────
	b.WriteString("--" + relatedBoundary + "\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + altBoundary + "\"\r\n")
	b.WriteString("\r\n")

	// Plain text part
	b.WriteString("--" + altBoundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(textBody)
	b.WriteString("\r\n")

	// HTML part
	b.WriteString("--" + altBoundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(htmlBody)
	b.WriteString("\r\n")

	// Close the alternative part
	b.WriteString("--" + altBoundary + "--\r\n")

	// ── Inline logo attachment ───────────────────────────────────────
	// Content-ID matches the cid: URL the template renders. Apple Mail,
	// Gmail (web + native iOS app + Android app), Outlook, Resend's
	// preview — all match on the bare token before the angle brackets
	// regardless of whether the cid: URL has them.
	if len(logoPNG) > 0 {
		b.WriteString("--" + relatedBoundary + "\r\n")
		b.WriteString("Content-Type: image/png; name=\"logo.png\"\r\n")
		b.WriteString("Content-Transfer-Encoding: base64\r\n")
		b.WriteString("Content-ID: <" + logoCID + ">\r\n")
		b.WriteString("Content-Disposition: inline; filename=\"logo.png\"\r\n")
		b.WriteString("\r\n")
		// Wrap base64 at 76 chars per RFC 2045 — some legacy
		// gateways drop overly-long lines.
		encoded := base64.StdEncoding.EncodeToString(logoPNG)
		for i := 0; i < len(encoded); i += 76 {
			end := i + 76
			if end > len(encoded) {
				end = len(encoded)
			}
			b.WriteString(encoded[i:end])
			b.WriteString("\r\n")
		}
	}

	b.WriteString("--" + relatedBoundary + "--\r\n")
	return b.String(), nil
}

// stripTags is a deliberately-naïve HTML-to-text fallback. We control
// every template's HTML so we don't need a full parser.
func stripTags(s string) string {
	var out strings.Builder
	inside := false
	for _, r := range s {
		switch {
		case r == '<':
			inside = true
		case r == '>':
			inside = false
		case !inside:
			out.WriteRune(r)
		}
	}
	collapsed := strings.Join(strings.Fields(out.String()), " ")
	return collapsed
}

// randHex returns 2*n hex chars sourced from time + counter — good
// enough for MIME boundaries (uniqueness within one process is the
// only requirement).
var boundaryCounter int64

func randHex(n int) string {
	boundaryCounter++
	return fmt.Sprintf("%x", time.Now().UnixNano()+boundaryCounter)[:n]
}

// ── Public API ──────────────────────────────────────────────────────

func (a *SMTPAdapter) SendPasswordResetEmail(to, resetToken, appBaseURL, locale string) error {
	loc := resolveLocale(locale)
	d := a.brand(loc)
	resetLink := strings.TrimRight(appBaseURL, "/") + "/reset-password?token=" + resetToken

	d.Subject = tr(loc, "reset_subject")
	d.HeaderTitle = tr(loc, "reset_title")
	d.Greeting = greet(loc, "")
	d.IntroHTML = asHTML(`<p style="margin:0 0 12px 0;">` + escapeHTML(tr(loc, "reset_intro")) + `</p>`)
	d.CTALabel = tr(loc, "reset_cta")
	d.CTAURL = resetLink
	d.FootnoteHTML = asHTML(escapeHTML(tr(loc, "link_expires_one_hour")) + " " + escapeHTML(tr(loc, "if_not_requested")))
	d.PlainText = tr(loc, "reset_intro") + "\n\n" + resetLink + "\n\n" +
		tr(loc, "link_expires_one_hour") + "\n" + tr(loc, "if_not_requested") + plaintextFooter(loc)

	subject, htmlBody, textBody, err := renderEmail(d)
	if err != nil {
		return err
	}
	return a.send(context.Background(), to, subject, htmlBody, textBody, false)
}

func (a *SMTPAdapter) SendVerificationEmail(to, verificationToken, appBaseURL, locale string) error {
	loc := resolveLocale(locale)
	d := a.brand(loc)
	// Point at the Flutter app route, NOT the bare API endpoint. The link
	// lands on app.replider.com (appBaseURL is the dashboard origin); the
	// /verify-email screen then calls GET /v1/auth/verify-email itself.
	// Using "/v1/auth/verify-email" here pointed the browser at the app
	// host's API path, which has no Flutter route → "Page Not Found".
	// Mirrors the password-reset / team-invite links (also app routes).
	verifyLink := strings.TrimRight(appBaseURL, "/") + "/verify-email?token=" + verificationToken

	d.Subject = tr(loc, "verify_subject")
	d.HeaderTitle = tr(loc, "verify_title")
	d.Greeting = greet(loc, "")
	d.IntroHTML = asHTML(`<p style="margin:0 0 12px 0;">` + escapeHTML(tr(loc, "verify_intro")) + `</p>`)
	d.CTALabel = tr(loc, "verify_cta")
	d.CTAURL = verifyLink
	d.FootnoteHTML = asHTML(escapeHTML(tr(loc, "link_expires_24_hours")))
	d.PlainText = tr(loc, "verify_intro") + "\n\n" + verifyLink + "\n\n" +
		tr(loc, "link_expires_24_hours") + plaintextFooter(loc)

	subject, htmlBody, textBody, err := renderEmail(d)
	if err != nil {
		return err
	}
	return a.send(context.Background(), to, subject, htmlBody, textBody, false)
}

func (a *SMTPAdapter) SendWelcomeEmail(to, userName, appURL, locale string) error {
	loc := resolveLocale(locale)
	d := a.brand(loc)

	d.Subject = tr(loc, "welcome_subject")
	d.HeaderTitle = tr(loc, "welcome_title")
	d.Greeting = greet(loc, userName)
	d.IntroHTML = asHTML(`<p style="margin:0 0 12px 0;">` + escapeHTML(tr(loc, "welcome_intro")) + `</p>`)
	d.BodyHTML = asHTML(tr(loc, "welcome_steps_html"))
	d.CTALabel = tr(loc, "welcome_cta")
	d.CTAURL = appURL
	d.PlainText = tr(loc, "welcome_intro") + "\n\n" + appURL + plaintextFooter(loc)

	subject, htmlBody, textBody, err := renderEmail(d)
	if err != nil {
		return err
	}
	return a.send(context.Background(), to, subject, htmlBody, textBody, false)
}

func (a *SMTPAdapter) SendUsageWarningEmail(to, userName string, usagePercent int, planName, upgradeURL, locale string) error {
	loc := resolveLocale(locale)
	d := a.brand(loc)
	d.BrandColor = "#F59E0B" // amber accent for the warning tier

	d.Subject = tr(loc, "usage_warning_subject_template", usagePercent)
	d.HeaderTitle = tr(loc, "usage_warning_title")
	d.Greeting = greet(loc, userName)
	d.IntroHTML = asHTML(`<p style="margin:0 0 12px 0;">` +
		fmt.Sprintf(localeFor(loc)["usage_warning_intro_template"], usagePercent, escapeHTML(planName)) +
		`</p>`)
	d.CTALabel = tr(loc, "usage_warning_cta")
	d.CTAURL = upgradeURL
	d.PlainText = fmt.Sprintf("%d%% — %s\n%s\n", usagePercent, planName, upgradeURL) + plaintextFooter(loc)

	subject, htmlBody, textBody, err := renderEmail(d)
	if err != nil {
		return err
	}
	return a.send(context.Background(), to, subject, htmlBody, textBody, false)
}

func (a *SMTPAdapter) SendUsageLimitEmail(to, userName, planName, upgradeURL, locale string) error {
	loc := resolveLocale(locale)
	d := a.brand(loc)
	d.BrandColor = "#EF4444" // red accent — limit reached

	d.Subject = tr(loc, "usage_limit_subject")
	d.HeaderTitle = tr(loc, "usage_limit_title")
	d.Greeting = greet(loc, userName)
	d.IntroHTML = asHTML(`<p style="margin:0 0 12px 0;">` +
		fmt.Sprintf(localeFor(loc)["usage_limit_intro_template"], escapeHTML(planName)) +
		`</p>`)
	d.CTALabel = tr(loc, "usage_limit_cta")
	d.CTAURL = upgradeURL
	d.PlainText = planName + "\n" + upgradeURL + plaintextFooter(loc)

	subject, htmlBody, textBody, err := renderEmail(d)
	if err != nil {
		return err
	}
	return a.send(context.Background(), to, subject, htmlBody, textBody, false)
}

func (a *SMTPAdapter) SendReplyReminderEmail(to, attendantName, contactName, conversationURL string, minutesWaiting int) error {
	loc := LocalePtBR
	d := a.brand(loc)
	d.BrandColor = "#EF4444" // red — urgency

	d.Subject = fmt.Sprintf(localeFor(loc)["reminder_subject_template"], contactName, minutesWaiting)
	d.HeaderTitle = tr(loc, "reminder_title")
	d.Greeting = greet(loc, attendantName)
	d.IntroHTML = asHTML(`<p style="margin:0 0 12px 0;">` +
		fmt.Sprintf(localeFor(loc)["reminder_intro_template"], escapeHTML(contactName), minutesWaiting) +
		`</p>`)
	d.CTALabel = tr(loc, "reminder_cta")
	d.CTAURL = conversationURL
	d.PlainText = fmt.Sprintf("%s — %d min\n%s\n", contactName, minutesWaiting, conversationURL) + plaintextFooter(loc)

	subject, htmlBody, textBody, err := renderEmail(d)
	if err != nil {
		return err
	}
	return a.send(context.Background(), to, subject, htmlBody, textBody, false)
}

func (a *SMTPAdapter) SendPaymentFailedEmail(to, userName, planName, retryURL, locale string) error {
	loc := resolveLocale(locale)
	d := a.brand(loc)
	d.BrandColor = "#EF4444"

	d.Subject = tr(loc, "payment_failed_subject")
	d.HeaderTitle = tr(loc, "payment_failed_title")
	d.Greeting = greet(loc, userName)
	d.IntroHTML = asHTML(`<p style="margin:0 0 12px 0;">` +
		fmt.Sprintf(localeFor(loc)["payment_failed_intro_template"], escapeHTML(planName)) +
		`</p>`)
	d.CTALabel = tr(loc, "payment_failed_cta")
	d.CTAURL = retryURL
	d.PlainText = planName + "\n" + retryURL + plaintextFooter(loc)

	subject, htmlBody, textBody, err := renderEmail(d)
	if err != nil {
		return err
	}
	return a.send(context.Background(), to, subject, htmlBody, textBody, false)
}

// prebillingWhen renders the human "renews in …" phrase for the given
// number of days, localized. 0 → today, 1 → tomorrow, else "N days".
// Negative values are clamped to "today" so a clock skew never produces
// "renews in -1 days".
func prebillingWhen(loc Locale, daysUntil int) string {
	switch {
	case daysUntil <= 0:
		return tr(loc, "prebilling_when_today")
	case daysUntil == 1:
		return tr(loc, "prebilling_when_tomorrow")
	default:
		return tr(loc, "prebilling_when_days_template", daysUntil)
	}
}

func (a *SMTPAdapter) SendPrebillingEmail(to, userName, planName string, daysUntil int, manageURL, locale string) error {
	loc := resolveLocale(locale)
	d := a.brand(loc)
	d.BrandColor = "#3B82F6" // blue accent — informational reminder

	when := prebillingWhen(loc, daysUntil)
	d.Subject = tr(loc, "prebilling_subject")
	d.HeaderTitle = tr(loc, "prebilling_title")
	d.Greeting = greet(loc, userName)
	d.IntroHTML = asHTML(`<p style="margin:0 0 12px 0;">` +
		fmt.Sprintf(localeFor(loc)["prebilling_intro_template"], escapeHTML(planName), escapeHTML(when)) +
		`</p>`)
	d.CTALabel = tr(loc, "prebilling_cta")
	d.CTAURL = manageURL
	d.PlainText = planName + " — " + when + "\n" + manageURL + plaintextFooter(loc)

	subject, htmlBody, textBody, err := renderEmail(d)
	if err != nil {
		return err
	}
	return a.send(context.Background(), to, subject, htmlBody, textBody, false)
}

func (a *SMTPAdapter) SendTeamInviteEmail(to, inviterName, resetToken, appBaseURL, locale string) error {
	loc := resolveLocale(locale)
	d := a.brand(loc)
	setPasswordLink := strings.TrimRight(appBaseURL, "/") + "/reset-password?token=" + resetToken

	intro := fmt.Sprintf(localeFor(loc)["invite_intro_template"], escapeHTML(inviterName))
	d.Subject = tr(loc, "invite_subject")
	d.HeaderTitle = tr(loc, "invite_title")
	d.Greeting = greet(loc, "")
	d.IntroHTML = asHTML(`<p style="margin:0 0 12px 0;">` + intro + `</p>`)
	d.CTALabel = tr(loc, "invite_cta")
	d.CTAURL = setPasswordLink
	d.FootnoteHTML = asHTML(escapeHTML(tr(loc, "link_expires_24_hours")) + " " + escapeHTML(tr(loc, "if_not_invited")))
	d.PlainText = inviterName + "\n" + setPasswordLink + "\n\n" + tr(loc, "link_expires_24_hours") + plaintextFooter(loc)

	subject, htmlBody, textBody, err := renderEmail(d)
	if err != nil {
		return err
	}
	return a.send(context.Background(), to, subject, htmlBody, textBody, false)
}

// SendSalesLeadEmail routes an Enterprise-tier qualification lead to the sales
// inbox (US-157). Unlike the customer-facing composers this is an INTERNAL
// notification, so the copy is fixed pt-BR and `to` is the sales address (not
// the lead's). Reply-To would ideally be the lead's email; the current send()
// path only sets From, so the lead email is surfaced prominently in the body
// instead.
func (a *SMTPAdapter) SendSalesLeadEmail(to string, lead domain.SalesLead) error {
	loc := LocalePtBR
	d := a.brand(loc)
	d.BrandColor = "#3B82F6" // blue accent — Enterprise

	row := func(label, value string) string {
		return `<p style="margin:0 0 8px 0;"><strong>` + escapeHTML(label) + `:</strong> ` + escapeHTML(value) + `</p>`
	}

	d.Subject = "Novo lead Enterprise — " + lead.Company
	d.HeaderTitle = "Novo lead Enterprise"
	d.Greeting = "Olá!"
	d.IntroHTML = asHTML(`<p style="margin:0 0 12px 0;">Um novo formulário de qualificação Enterprise foi enviado pelo site:</p>`)
	body := row("Empresa", lead.Company) +
		row("E-mail", lead.Email) +
		row("Porte", lead.CompanySize) +
		row("Canais", lead.Channels) +
		row("Volume mensal", lead.MonthlyVolume)
	if strings.TrimSpace(lead.Message) != "" {
		body += row("Mensagem", lead.Message)
	}
	d.BodyHTML = asHTML(body)
	d.CTALabel = "Responder por e-mail"
	d.CTAURL = "mailto:" + lead.Email

	plain := strings.Join([]string{
		"Novo lead Enterprise",
		"Empresa: " + lead.Company,
		"E-mail: " + lead.Email,
		"Porte: " + lead.CompanySize,
		"Canais: " + lead.Channels,
		"Volume mensal: " + lead.MonthlyVolume,
	}, "\n")
	if strings.TrimSpace(lead.Message) != "" {
		plain += "\nMensagem: " + lead.Message
	}
	d.PlainText = plain + plaintextFooter(loc)

	subject, htmlBody, textBody, err := renderEmail(d)
	if err != nil {
		return err
	}
	return a.send(context.Background(), to, subject, htmlBody, textBody, false)
}

// SendSDRDigestEmail renders the daily actionable digest (US-M010).
//
// A FALLBACK for accounts with no push subscription — never a second copy
// of a push that already landed. Each line is a link, because the digest's
// only job is getting the reader one click from the work.
func (a *SMTPAdapter) SendSDRDigestEmail(to, userName string, lines []domain.DigestLine, appBaseURL, locale string) error {
	if len(lines) == 0 {
		// Nothing actionable: the caller should not have asked, and
		// sending an empty digest is how a daily email gets filtered.
		return nil
	}
	loc := resolveLocale(locale)
	d := a.brand(loc)
	d.BrandColor = "#3B82F6"

	base := strings.TrimRight(appBaseURL, "/")
	d.Subject = tr(loc, "sdr_digest_subject")
	d.HeaderTitle = tr(loc, "sdr_digest_title")
	d.Greeting = greet(loc, userName)
	d.IntroHTML = asHTML(`<p style="margin:0 0 12px 0;">` + escapeHTML(tr(loc, "sdr_digest_intro")) + `</p>`)

	var body strings.Builder
	body.WriteString(`<ul style="margin:0 0 16px 0;padding-left:18px;">`)
	for _, line := range lines {
		fmt.Fprintf(&body,
			`<li style="margin:0 0 8px 0;"><a href="%s%s" style="color:#3B82F6;text-decoration:none;">%s</a></li>`,
			escapeHTML(base), escapeHTML(line.ClickURL), escapeHTML(line.Text))
	}
	body.WriteString(`</ul>`)
	d.BodyHTML = asHTML(body.String())

	d.CTALabel = tr(loc, "sdr_digest_cta")
	d.CTAURL = base + lines[0].ClickURL

	var plain strings.Builder
	plain.WriteString(tr(loc, "sdr_digest_intro"))
	for _, line := range lines {
		fmt.Fprintf(&plain, "\n- %s: %s%s", line.Text, base, line.ClickURL)
	}
	d.PlainText = plain.String() + plaintextFooter(loc)

	subject, htmlBody, textBody, err := renderEmail(d)
	if err != nil {
		return err
	}
	return a.send(context.Background(), to, subject, htmlBody, textBody, false)
}

// SendWeeklyDigestEmail renders the retention digest (US-176): the
// trailing-week activity summary for one account's admin. It is NOT part
// of the EmailService interface on purpose — it's driven by a background
// worker via a narrow notifier port (mirrors SendSalesLeadEmail), so the
// large EmailService mocks don't need a new method. locale picks pt-BR
// (default) vs en; manageURL deep-links to the dashboard; unsubscribeURL
// (optional) surfaces the per-account opt-out.
func (a *SMTPAdapter) SendWeeklyDigestEmail(to, userName string, digest domain.WeeklyDigest, manageURL, unsubscribeURL, locale string) error {
	loc := resolveLocale(locale)
	d := a.brand(loc)
	d.BrandColor = "#3B82F6" // blue accent — informational recap

	stats := digest.Stats
	d.Subject = tr(loc, "digest_subject")
	d.HeaderTitle = tr(loc, "digest_title")
	d.Greeting = greet(loc, userName)
	d.IntroHTML = asHTML(`<p style="margin:0 0 12px 0;">` + escapeHTML(tr(loc, "digest_intro")) + `</p>`)

	d.BodyHTML = asHTML(digestBodyHTML(loc, stats))
	d.CTALabel = tr(loc, "digest_cta")
	d.CTAURL = manageURL
	if unsubscribeURL != "" {
		d.UnsubscribeURL = unsubscribeURL
		d.FootnoteHTML = asHTML(escapeHTML(tr(loc, "digest_unsubscribe")))
	}
	d.PlainText = digestPlainText(loc, stats, manageURL)

	subject, htmlBody, textBody, err := renderEmail(d)
	if err != nil {
		return err
	}
	return a.send(context.Background(), to, subject, htmlBody, textBody, false)
}

// digestBodyHTML renders the metric rows + top-FAQ list for the digest
// email body. Falls back to a friendly "no activity yet" note when the
// account had a silent week so the email never looks broken/empty.
func digestBodyHTML(loc Locale, stats domain.WeeklyDigestStats) string {
	if stats.MessagesHandled == 0 && len(stats.TopFAQs) == 0 {
		return `<p style="margin:0 0 12px 0;color:#D4D4D4;">` + escapeHTML(tr(loc, "digest_no_activity")) + `</p>`
	}

	metric := func(label, value string) string {
		return `<tr>` +
			`<td style="padding:8px 0;color:#D4D4D4;">` + escapeHTML(label) + `</td>` +
			`<td style="padding:8px 0;text-align:right;font-weight:700;color:#FFFFFF;">` + escapeHTML(value) + `</td>` +
			`</tr>`
	}

	var b strings.Builder
	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 16px 0;border-collapse:collapse;">`)
	b.WriteString(metric(tr(loc, "digest_metric_messages"), fmt.Sprintf("%d", stats.MessagesHandled)))
	b.WriteString(metric(tr(loc, "digest_metric_time_saved"), tr(loc, "digest_minutes_template", stats.TimeSavedMinutes)))
	b.WriteString(metric(tr(loc, "digest_metric_conversion"), fmt.Sprintf("%d%%", stats.ConversionPct)))
	b.WriteString(`</table>`)

	if len(stats.TopFAQs) > 0 {
		b.WriteString(`<p style="margin:0 0 8px 0;font-weight:600;">` + escapeHTML(tr(loc, "digest_faqs_title")) + `</p>`)
		b.WriteString(`<ol style="margin:0 0 12px 18px;padding:0;color:#D4D4D4;">`)
		for _, faq := range stats.TopFAQs {
			b.WriteString(`<li style="margin-bottom:6px;">` + escapeHTML(faq.Question) +
				` <span style="color:#9CA3AF;">(` + fmt.Sprintf("%d", faq.Count) + `)</span></li>`)
		}
		b.WriteString(`</ol>`)
	}
	return b.String()
}

// digestPlainText is the text/plain fallback mirroring digestBodyHTML.
func digestPlainText(loc Locale, stats domain.WeeklyDigestStats, manageURL string) string {
	var b strings.Builder
	b.WriteString(tr(loc, "digest_intro"))
	b.WriteString("\n\n")
	if stats.MessagesHandled == 0 && len(stats.TopFAQs) == 0 {
		b.WriteString(tr(loc, "digest_no_activity"))
	} else {
		b.WriteString(fmt.Sprintf("%s: %d\n", tr(loc, "digest_metric_messages"), stats.MessagesHandled))
		b.WriteString(fmt.Sprintf("%s: %s\n", tr(loc, "digest_metric_time_saved"), tr(loc, "digest_minutes_template", stats.TimeSavedMinutes)))
		b.WriteString(fmt.Sprintf("%s: %d%%\n", tr(loc, "digest_metric_conversion"), stats.ConversionPct))
		if len(stats.TopFAQs) > 0 {
			b.WriteString("\n" + tr(loc, "digest_faqs_title") + ":\n")
			for _, faq := range stats.TopFAQs {
				b.WriteString(fmt.Sprintf("- %s (%d)\n", faq.Question, faq.Count))
			}
		}
	}
	b.WriteString("\n" + manageURL)
	return b.String() + plaintextFooter(loc)
}

// SendReactivationEmail nudges a lapsed-trial admin to come back (US-181).
// daysSinceTrial is the number of days since the trial expired (the 7/14/30
// milestone); reactivateURL is the (UTM-tagged, click-tracked) link back
// into the app.
func (a *SMTPAdapter) SendReactivationEmail(to, userName string, daysSinceTrial int, reactivateURL, locale string) error {
	loc := resolveLocale(locale)
	d := a.brand(loc)
	d.BrandColor = "#3B82F6" // blue accent — lifecycle nudge

	d.Subject = tr(loc, "reactivation_subject")
	d.HeaderTitle = tr(loc, "reactivation_title")
	d.Greeting = greet(loc, userName)
	d.IntroHTML = asHTML(`<p style="margin:0 0 12px 0;">` + escapeHTML(tr(loc, "reactivation_intro", daysSinceTrial)) + `</p>` +
		`<p style="margin:0 0 12px 0;">` + escapeHTML(tr(loc, "reactivation_body")) + `</p>`)
	d.CTALabel = tr(loc, "reactivation_cta")
	d.CTAURL = reactivateURL
	d.FootnoteHTML = asHTML(escapeHTML(tr(loc, "reactivation_footnote")))
	d.PlainText = tr(loc, "reactivation_intro", daysSinceTrial) + "\n\n" +
		tr(loc, "reactivation_body") + "\n\n" + reactivateURL +
		"\n\n" + tr(loc, "reactivation_footnote") + plaintextFooter(loc)

	subject, htmlBody, textBody, err := renderEmail(d)
	if err != nil {
		return err
	}
	return a.send(context.Background(), to, subject, htmlBody, textBody, false)
}

// SendWinBackEmail sends the win-back recovery offer to a churned customer
// (US-179): a 50%-off-3-months coupon, sent immediately on cancellation
// (daysSince 0) and re-sent as follow-up reminders at 7/30 days. One
// composer serves all three sends — only the timing (decided by the
// worker) differs, so the copy is timing-agnostic; daysSince is accepted
// for parity/logging.
func (a *SMTPAdapter) SendWinBackEmail(to, userName string, daysSince, discountPercent, durationMonths int, couponCode, reactivateURL, locale string) error {
	_ = daysSince
	loc := resolveLocale(locale)
	d := a.brand(loc)
	d.BrandColor = "#3B82F6" // blue accent — lifecycle nudge

	d.Subject = tr(loc, "winback_subject", discountPercent, durationMonths)
	d.HeaderTitle = tr(loc, "winback_title")
	d.Greeting = greet(loc, userName)
	d.IntroHTML = asHTML(`<p style="margin:0 0 12px 0;">` + escapeHTML(tr(loc, "winback_intro", discountPercent, durationMonths)) + `</p>` +
		`<p style="margin:0 0 12px 0;">` + escapeHTML(tr(loc, "winback_body", couponCode)) + `</p>`)
	d.CTALabel = tr(loc, "winback_cta")
	d.CTAURL = reactivateURL
	d.FootnoteHTML = asHTML(escapeHTML(tr(loc, "winback_footnote")))
	d.PlainText = tr(loc, "winback_intro", discountPercent, durationMonths) + "\n\n" +
		tr(loc, "winback_body", couponCode) + "\n\n" + reactivateURL +
		"\n\n" + tr(loc, "winback_footnote") + plaintextFooter(loc)

	subject, htmlBody, textBody, err := renderEmail(d)
	if err != nil {
		return err
	}
	return a.send(context.Background(), to, subject, htmlBody, textBody, false)
}

// onboardingNudgeKeyPrefix maps a nudge stage to its copy key namespace in
// strings.go. Defaults to the share stage for any unexpected value so the
// email never renders with empty copy.
func onboardingNudgeKeyPrefix(milestone string) string {
	switch milestone {
	case "channel", "products", "customers", "proposal", "first_inbound", "team_invited":
		return "onboarding_nudge_" + milestone
	default:
		// An unknown milestone renders the first step rather than a blank
		// email: the link still lands somewhere useful.
		return "onboarding_nudge_channel"
	}
}

// SendOnboardingNudgeEmail drips one activation nudge to a new account's
// admin (US-182). `milestone` selects the copy — one of the six sales
// milestones (channel, products, customers, proposal, first_inbound,
// team_invited) — and ctaURL is the (UTM-tagged, click-tracked) deep link
// into the screen where it gets done. The footnote copy is shared.
func (a *SMTPAdapter) SendOnboardingNudgeEmail(to, userName string, milestone string, ctaURL, locale string) error {
	loc := resolveLocale(locale)
	d := a.brand(loc)
	d.BrandColor = "#3B82F6" // blue accent — lifecycle nudge
	key := onboardingNudgeKeyPrefix(milestone)

	d.Subject = tr(loc, key+"_subject")
	d.HeaderTitle = tr(loc, key+"_title")
	d.Greeting = greet(loc, userName)
	d.IntroHTML = asHTML(`<p style="margin:0 0 12px 0;">` + escapeHTML(tr(loc, key+"_intro")) + `</p>` +
		`<p style="margin:0 0 12px 0;">` + escapeHTML(tr(loc, key+"_body")) + `</p>`)
	d.CTALabel = tr(loc, key+"_cta")
	d.CTAURL = ctaURL
	d.FootnoteHTML = asHTML(escapeHTML(tr(loc, "onboarding_nudge_footnote")))
	d.PlainText = tr(loc, key+"_intro") + "\n\n" +
		tr(loc, key+"_body") + "\n\n" + ctaURL +
		"\n\n" + tr(loc, "onboarding_nudge_footnote") + plaintextFooter(loc)

	subject, htmlBody, textBody, err := renderEmail(d)
	if err != nil {
		return err
	}
	return a.send(context.Background(), to, subject, htmlBody, textBody, false)
}

func (a *SMTPAdapter) SendTestEmail(to, userName string) error {
	loc := LocalePtBR
	d := a.brand(loc)

	d.Subject = tr(loc, "smtp_test_subject")
	d.HeaderTitle = tr(loc, "smtp_test_title")
	d.Greeting = greet(loc, userName)
	d.IntroHTML = asHTML(`<p style="margin:0 0 12px 0;">` + escapeHTML(tr(loc, "smtp_test_intro")) + `</p>`)
	d.FootnoteHTML = asHTML(escapeHTML(tr(loc, "smtp_test_footnote")))
	d.PlainText = tr(loc, "smtp_test_intro") + plaintextFooter(loc)

	subject, htmlBody, textBody, err := renderEmail(d)
	if err != nil {
		return err
	}
	// Bound the request — see smtpTimeout godoc above.
	ctx, cancel := context.WithTimeout(context.Background(), smtpTimeout)
	defer cancel()
	return a.send(ctx, to, subject, htmlBody, textBody, true)
}

// _ silences the unused import warning for html/template when the
// branded composers happen to drop their template.HTML usage during
// refactors. We import it through asHTML / template.URL helpers and
// want the package available unconditionally.
var _ template.HTML = ""
