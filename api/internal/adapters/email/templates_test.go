package email

import (
	"strings"
	"testing"
)

// TestRenderEmail_ReturnsBrandedShell asserts the shared base template
// renders with the brand bar (Replider name + logo) and the supplied
// body block. We don't hard-code colors or HTML — the test fails when
// something blanks the entire shell, which would let us ship a near-
// empty email by accident.
func TestRenderEmail_ReturnsBrandedShell(t *testing.T) {
	d := brandDefaults("https://app.replider.com")
	d.Locale = LocalePtBR
	d.Subject = "Sub"
	d.HeaderTitle = "Cabeçalho"
	d.Greeting = "Olá!"
	d.IntroHTML = asHTML("<p>Olá teste</p>")
	d.CTALabel = "Clique"
	d.CTAURL = "https://app.replider.com/x"
	d.FooterTagline = tr(LocalePtBR, "footer_tagline")
	d.PlainText = "plain"

	subject, html, plain, err := renderEmail(d)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if subject != "Sub" {
		t.Errorf("subject not propagated, got %q", subject)
	}
	if plain != "plain" {
		t.Errorf("plaintext not propagated, got %q", plain)
	}
	for _, want := range []string{
		"Replider",                   // brand name
		"Icon-512.png",               // logo path
		"Cabeçalho",                  // header title
		"Olá teste",                  // intro
		"Clique",                     // CTA label
		"https://app.replider.com/x", // CTA URL
		"AI-powered omnichannel",     // footer (en) — false negative if pt-BR; check via locale
	} {
		_ = want
	}
	// Localized assertions: we set Locale=pt-BR, so the footer should
	// be the pt-BR copy, not the en fallback.
	if !strings.Contains(html, "Replider") {
		t.Error("brand name missing")
	}
	// Logo is referenced via Content-ID since we ship it as an inline
	// MIME attachment — the rendered HTML uses cid:<token>, not a
	// hosted URL. Asserting on the cid: scheme also locks in the
	// safeURL pipe in base.html (without it, html/template's URL
	// sanitizer rewrites unknown schemes to #ZgotmplZ).
	if !strings.Contains(html, "cid:"+logoCID) {
		t.Errorf("logo cid reference missing; html: %s", html)
	}
	if !strings.Contains(html, "Cabeçalho") {
		t.Error("header title missing")
	}
	if !strings.Contains(html, "Olá teste") {
		t.Error("intro html missing")
	}
	if !strings.Contains(html, "Clique") {
		t.Error("CTA label missing")
	}
	if !strings.Contains(html, "https://app.replider.com/x") {
		t.Error("CTA url missing")
	}
	if !strings.Contains(html, "Todos os direitos") {
		t.Error("pt-BR footer missing")
	}
}

func TestRenderEmail_HidesCTAWhenURLEmpty(t *testing.T) {
	// Some emails (SMTP test, plain notices) intentionally have no
	// CTA. The shared shell must not render an empty button stub
	// when CTAURL is unset.
	d := brandDefaults("")
	d.Locale = LocalePtBR
	d.Subject = "x"
	d.HeaderTitle = "Title"
	d.Greeting = "Olá!"
	d.IntroHTML = asHTML("<p>body</p>")
	d.PlainText = "plain"

	_, html, _, err := renderEmail(d)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// The CTA <a> tag would carry the literal CTALabel; if it leaked
	// through with no URL the rendered HTML would still contain the
	// padding-heavy button table. Search for the placeholder href to
	// confirm it stayed hidden.
	if strings.Contains(html, `padding:14px 28px;color:#FFFFFF`) {
		t.Error("CTA button rendered despite empty CTAURL")
	}
}

func TestLocaleFallback(t *testing.T) {
	// Unknown locale → pt-BR (primary market). Empty input → pt-BR.
	cases := map[string]Locale{
		"":      LocalePtBR,
		"pt":    LocalePtBR,
		"pt-BR": LocalePtBR,
		"PT-br": LocalePtBR,
		"en":    LocaleEn,
		"EN":    LocaleEn,
		"en-US": LocaleEn,
		"es":    LocalePtBR, // unsupported → fallback
	}
	for in, want := range cases {
		if got := resolveLocale(in); got != want {
			t.Errorf("resolveLocale(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestBuildMultipart_EmbedsLogoAsCID asserts the wire MIME has the
// nested multipart/related → multipart/alternative structure plus the
// inline image part with the matching Content-ID. If any of these
// boundaries break, every email arrives logo-less and image-proxied
// off the API origin (which doesn't serve /icons in prod) — exactly
// the bug Tiago hit on Apr 26.
func TestBuildMultipart_EmbedsLogoAsCID(t *testing.T) {
	msg, err := buildMultipart("a@x", "b@y", "subj", "<p>html cid:"+logoCID+"</p>", "plain")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	wantSubstrings := []string{
		"Content-Type: multipart/related;",
		"type=\"multipart/alternative\"",
		"Content-Type: multipart/alternative;",
		"Content-Type: text/plain;",
		"Content-Type: text/html;",
		"Content-Type: image/png;",
		"Content-Transfer-Encoding: base64",
		"Content-ID: <" + logoCID + ">",
		"Content-Disposition: inline;",
		"cid:" + logoCID,
	}
	for _, w := range wantSubstrings {
		if !strings.Contains(msg, w) {
			t.Errorf("MIME missing %q\n--- message ---\n%s", w, msg)
		}
	}
}

func TestStringFallback_PtBROnTypo(t *testing.T) {
	// A key that exists only in pt-BR should still resolve when
	// asked in en — the lookup falls through to the pt-BR map so
	// callers don't ship empty copy.
	stringsPtBR["___test_only_pt"] = "fallback ok"
	defer delete(stringsPtBR, "___test_only_pt")
	if got := tr(LocaleEn, "___test_only_pt"); got != "fallback ok" {
		t.Errorf("expected pt-BR fallback, got %q", got)
	}
}
