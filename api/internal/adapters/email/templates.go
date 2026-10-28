package email

import (
	"bytes"
	"embed"
	"fmt"
	"html"
	"html/template"
	"strings"
)

//go:embed templates/*.html
var templateFS embed.FS

// logoPNG is the inline brand logo, attached to every branded email
// via CID so the message renders self-contained — no cross-origin
// fetch required, no broken-image fallback when the email client
// proxies images and our cfg.AppBaseURL doesn't serve /icons (the
// dashboard origin does, but cfg.AppBaseURL is the API origin in
// prod). Stored under templates/ so the embed.FS pattern stays
// consistent.
//
//go:embed templates/logo.png
var logoPNG []byte

// logoCID is the Content-ID we attach the embedded logo with. Templates
// reference it as <img src="cid:{{logoCID}}">. The "@replider.local"
// suffix is a pure label — RFC 2392 only requires that CIDs be unique
// within the message envelope, and Gmail/Outlook/Apple Mail all match
// on the bare token before the <>.
const logoCID = "logo@replider.local"

// Locale of the email body. We default to pt-BR (primary market) and
// fall back when callers pass an empty string. English is supported so
// future inbound users from non-Brazilian Stripe checkouts get readable
// copy without a translator.
type Locale string

const (
	LocalePtBR Locale = "pt-BR"
	LocaleEn   Locale = "en"
)

func resolveLocale(raw string) Locale {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "en", "en-us", "en-gb":
		return LocaleEn
	default:
		return LocalePtBR
	}
}

// renderEmail produces the (subject, html, plaintext) tuple for a
// pre-built emailData. We use ONE base.html template — every email
// type pre-builds its content blocks in Go (via emailData.IntroHTML /
// BodyHTML / etc.) and renders through the shared shell. Avoiding
// per-type templates keeps the rendering surface small and the diff
// of "what does this email actually contain?" readable in the source.
//
// Render errors are surfaced — callers should treat them as fatal
// because we'd rather drop the email than ship a truncated stub.
func renderEmail(data emailData) (subject, htmlBody, textBody string, err error) {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"safeURL": func(s string) template.URL { return template.URL(s) },
	}).ParseFS(templateFS, "templates/base.html")
	if err != nil {
		return "", "", "", fmt.Errorf("parse base template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base.html", data); err != nil {
		return "", "", "", fmt.Errorf("render base template: %w", err)
	}
	return data.Subject, buf.String(), data.PlainText, nil
}

// emailData is the catch-all variable bag the templates render against.
// Optional fields (CTAURL, BodyHTML) stay zero-valued when not used.
// Strings are pre-localized — i18n lookup happens at the call site so
// the templates themselves don't drag a translation map in.
type emailData struct {
	Locale         Locale
	BrandName      string
	BrandLogoURL   string
	BrandColor     string // accent — header bar + CTA background
	HeaderTitle    string
	Greeting       string // e.g. "Olá, João!"
	Subject        string // mail subject (rendered by caller, mirrored here for plaintext part)
	IntroHTML      template.HTML
	BodyHTML       template.HTML // optional middle block (warning notes, lists, etc.)
	CTALabel       string
	CTAURL         string
	FootnoteHTML   template.HTML // small print under the CTA
	FooterTagline  string
	UnsubscribeURL string // future: list-unsubscribe link
	Year           string
	PlainText      string // pre-rendered text/plain fallback
}

// brandDefaults returns the static branding fields shared by every
// template. Centralized so changing the logo or accent color only
// touches one spot.
//
// BrandLogoURL deliberately points at `cid:<logo@replider.local>` —
// the logo PNG is attached inline (multipart/related) by buildMime
// instead of fetched from the network. Embedding makes the email
// self-contained: works offline, doesn't depend on the API origin
// serving /icons, doesn't break when an email client's image proxy
// can't reach the originating host. The `appBaseURL` argument stays
// in the signature because callers pass it for their CTA links.
func brandDefaults(appBaseURL string) emailData {
	_ = appBaseURL // reserved for future per-tenant logo overrides
	return emailData{
		BrandName:     "Replider",
		BrandLogoURL:  "cid:" + logoCID,
		BrandColor:    "#3B82F6", // matches design system bluePrimary
		FooterTagline: "",        // localized per locale at call site
		Year:          "2026",
	}
}

// escapeHTML wraps html.EscapeString so call sites that build BodyHTML
// can interpolate user-supplied strings without opening XSS vectors.
// Templates use {{.Field}} which auto-escapes — but for fields typed as
// template.HTML (BodyHTML, FootnoteHTML) the responsibility moves to
// the caller.
func escapeHTML(s string) string { return html.EscapeString(s) }
