package handlers

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// This is the one place the platform renders HTML written by a stranger it
// prospected itself. The sanitiser is the first of three layers — the CSP
// denies script execution outright — but "the browser will catch it" is not a
// reason to ship a stripper that misses the obvious cases.

func TestSanitizeOriginal_RemovesScripts(t *testing.T) {
	cases := []string{
		`<p>oi</p><script>alert(1)</script>`,
		`<p>oi</p><SCRIPT SRC="//evil.com/x.js"></SCRIPT>`,
		// Unclosed: the body would otherwise survive as visible text, and a
		// browser would still treat it as script.
		`<p>oi</p><script>alert(1)`,
	}
	for _, in := range cases {
		out := sanitizeOriginalHTML(in, false)
		assert.NotContains(t, strings.ToLower(out), "<script", "input %q", in)
		assert.NotContains(t, out, "alert(1)", "input %q", in)
		assert.Contains(t, out, "<p>oi</p>", "the real content must survive")
	}
}

func TestSanitizeOriginal_RemovesEventHandlers(t *testing.T) {
	cases := []string{
		`<div onclick="steal()">x</div>`,
		`<div onclick='steal()'>x</div>`,
		`<div onclick=steal()>x</div>`,
		`<img ONERROR="steal()" src="data:image/gif;base64,R0lGOD">`,
		`<body onload="steal()">x</body>`,
	}
	for _, in := range cases {
		out := strings.ToLower(sanitizeOriginalHTML(in, true))
		assert.NotContains(t, out, "onclick", "input %q", in)
		assert.NotContains(t, out, "onerror", "input %q", in)
		assert.NotContains(t, out, "onload", "input %q", in)
	}
}

func TestSanitizeOriginal_NeutralisesDangerousURLSchemes(t *testing.T) {
	cases := []string{
		`<a href="javascript:steal()">clique</a>`,
		`<a href='JavaScript:steal()'>clique</a>`,
		`<a href="vbscript:steal()">clique</a>`,
		`<a href="data:text/html,<script>steal()</script>">clique</a>`,
	}
	for _, in := range cases {
		out := strings.ToLower(sanitizeOriginalHTML(in, false))
		assert.NotContains(t, out, "javascript:", "input %q", in)
		assert.NotContains(t, out, "vbscript:", "input %q", in)
		// The anchor stays, pointing nowhere: dropping the attribute
		// entirely can re-open the tag it lived in.
		assert.Contains(t, out, `href="#"`, "input %q", in)
	}
}

func TestSanitizeOriginal_RemovesFramesAndForms(t *testing.T) {
	in := `<iframe src="//evil"></iframe><object data="x"></object><embed src="y">` +
		`<form action="//evil"><input name="senha"></form><base href="//evil">`
	out := strings.ToLower(sanitizeOriginalHTML(in, false))

	for _, tag := range []string{"<iframe", "<object", "<embed", "<form", "<base"} {
		assert.NotContains(t, out, tag)
	}
}

// A remote image in an email is a read receipt. Off by default.
func TestSanitizeOriginal_StripsRemoteImagesUnlessAsked(t *testing.T) {
	in := `<p>Bom dia</p><img src="https://tracker.example/pixel.gif" width="1" height="1">`

	blocked := sanitizeOriginalHTML(in, false)
	assert.NotContains(t, blocked, "tracker.example",
		"a remote image tells the sender the message was opened")
	assert.Contains(t, blocked, "<p>Bom dia</p>")

	allowed := sanitizeOriginalHTML(in, true)
	assert.Contains(t, allowed, "tracker.example",
		"the reader asked for images, so they load")
}

// Embedded images are part of the message and tell the sender nothing, so
// they survive either way.
func TestSanitizeOriginal_KeepsEmbeddedImages(t *testing.T) {
	in := `<img src="data:image/png;base64,iVBORw0KGgo=">`

	assert.Contains(t, sanitizeOriginalHTML(in, false), "data:image/png")
}

// Formatting is the entire reason this view exists.
func TestSanitizeOriginal_KeepsFormatting(t *testing.T) {
	in := `<div style="color:#333"><h2>Proposta</h2><table><tr><td><b>Item</b></td></tr></table></div>`
	out := sanitizeOriginalHTML(in, false)

	for _, keep := range []string{"<h2>", "<table>", "<b>", `style="color:#333"`} {
		assert.Contains(t, out, keep)
	}
}

// A style attribute is not an event handler, and a word starting with "on"
// is not one either. Over-stripping breaks legitimate mail.
func TestSanitizeOriginal_DoesNotEatInnocentAttributes(t *testing.T) {
	in := `<td colspan="2" align="left"><span>online</span></td>`
	out := sanitizeOriginalHTML(in, false)

	assert.Contains(t, out, `colspan="2"`)
	assert.Contains(t, out, `align="left"`)
	assert.Contains(t, out, "online")
}

// Metadata arrives as a map from a freshly built message and as raw JSONB
// bytes from one read back out of Postgres. Handling only the map is how this
// silently returned nothing for every persisted message — which is exactly
// the bug the e2e caught.
func TestMetadataString_ReadsEveryShapeTheDriverReturns(t *testing.T) {
	const want = "accounts/a/email-html/x"
	raw := []byte(`{"email_html_path":"` + want + `"}`)

	assert.Equal(t, want, metadataString(raw, "email_html_path"))
	assert.Equal(t, want, metadataString(json.RawMessage(raw), "email_html_path"))
	assert.Equal(t, want, metadataString(string(raw), "email_html_path"))
	assert.Empty(t, metadataString([]byte("nao e json"), "email_html_path"))
}

func TestMetadataString(t *testing.T) {
	assert.Equal(t, "accounts/a/email-html/x",
		metadataString(map[string]any{"email_html_path": " accounts/a/email-html/x "}, "email_html_path"))
	assert.Empty(t, metadataString(map[string]any{}, "email_html_path"))
	assert.Empty(t, metadataString(nil, "email_html_path"))
	assert.Empty(t, metadataString("not a map", "email_html_path"))
	assert.Empty(t, metadataString(map[string]any{"email_html_path": 42}, "email_html_path"))
}

// The value is judged after decoding entities and stripping the whitespace
// browsers ignore inside a scheme — both are standard ways to smuggle one
// past a naive check.
func TestIsSafeURLValue_ObfuscatedSchemes(t *testing.T) {
	unsafe := []string{
		"javascript:steal()",
		"JaVaScRiPt:steal()",
		" javascript:steal()",
		"java\tscript:steal()",
		"java\nscript:steal()",
		"&#106;avascript:steal()",
		"vbscript:steal()",
		"data:text/html,<b>x</b>",
		"data:application/xhtml+xml,x",
		"data:,plain",
	}
	for _, v := range unsafe {
		assert.False(t, isSafeURLValue(v), "must be refused: %q", v)
	}
}

// Over-blocking breaks real mail: relative links, mailto and anchors are the
// normal contents of a message.
func TestIsSafeURLValue_KeepsOrdinaryURLs(t *testing.T) {
	safe := []string{
		"https://clinica.com.br/proposta",
		"http://clinica.com.br",
		"mailto:compras@clinica.com.br",
		"tel:+5515999990000",
		"/relativo/caminho",
		"#ancora",
		"",
		"data:image/png;base64,iVBORw0KGgo=",
		"data:image/gif;base64,R0lGOD",
	}
	for _, v := range safe {
		assert.True(t, isSafeURLValue(v), "must be kept: %q", v)
	}
}

// The regression the embedded-image test caught: blanket-blocking data:
// destroyed every inline image, which is most of what makes this view worth
// opening at all.
func TestSanitizeOriginal_EmbeddedImageSurvivesAlongsideABlockedLink(t *testing.T) {
	in := `<a href="javascript:x()">clique</a><img src="data:image/png;base64,iVBORw0KGgo=">`
	out := sanitizeOriginalHTML(in, false)

	assert.Contains(t, out, "data:image/png")
	assert.NotContains(t, strings.ToLower(out), "javascript:")
}

// ── cid: resolution (2026-08-26) ────────────────────────────────────────────
//
// Inline email images arrive referenced by cid:, a scheme only a mail client
// resolves — the stored original rendered every embedded photo and logo
// broken, in both the inline sandboxed view and the external one.

func TestRewriteCID_KnownIDBecomesDataURI(t *testing.T) {
	html := `<p>oi</p><img src="cid:foto1@gmail" width="400">`
	out := rewriteCIDReferences(html, map[string]string{
		"foto1@gmail": "data:image/jpeg;base64,AAAA",
	})
	if !strings.Contains(out, `src="data:image/jpeg;base64,AAAA"`) {
		t.Fatalf("cid not rewritten: %s", out)
	}
	// And the sanitiser must let the result through — data:image/ is its
	// explicit allowance.
	safe := sanitizeOriginalHTML(out, false)
	if !strings.Contains(safe, "data:image/jpeg;base64,AAAA") {
		t.Fatalf("sanitiser stripped the inlined image: %s", safe)
	}
}

func TestRewriteCID_UnknownIDNeutralised(t *testing.T) {
	// Superseded by removal: an unresolvable image leaves NOTHING, not a
	// broken-icon ghost. An <a href="cid:..."> keeps the neutralised "#".
	out := rewriteCIDReferences(`<img src="cid:sumiu"><a href="cid:sumiu">x</a>`, nil)
	if strings.Contains(out, "<img") {
		t.Fatalf("unresolvable img must be removed: %s", out)
	}
	if !strings.Contains(out, `href="#"`) {
		t.Fatalf("anchor keeps the neutralised href: %s", out)
	}
}

// The angle-bracket form some MUAs write must match the stored, stripped id.
func TestRewriteCID_AngleBracketsAndSingleQuotes(t *testing.T) {
	out := rewriteCIDReferences(`<img src='cid:<logo@assinatura>'>`, map[string]string{
		"logo@assinatura": "data:image/png;base64,BB",
	})
	if !strings.Contains(out, "data:image/png;base64,BB") {
		t.Fatalf("bracketed cid not resolved: %s", out)
	}
}

// A cid: whose bytes are gone (stored before content_id existed) must not
// leave a broken-image ghost with the filename as alt text.
func TestRewriteCID_UnresolvableImageIsRemoved(t *testing.T) {
	out := rewriteCIDReferences(
		`<p>olha</p><img src="cid:sumiu" alt="foto.jpg" width="400">`, nil)
	if strings.Contains(out, "<img") {
		t.Fatalf("ghost image should be removed: %s", out)
	}
	if !strings.Contains(out, "<p>olha</p>") {
		t.Fatalf("the rest of the document must survive: %s", out)
	}
}

// The fast path: cid: references become signed media-proxy URLs, and the
// rewrite runs AFTER sanitising — the sanitiser strips remote <img> when
// load_images is off, and a proxy URL injected before it would be stripped
// with the trackers it exists to block.
func TestRewriteCID_ProxyURLSurvivesSanitiseOrder(t *testing.T) {
	in := `<p>foto</p><img src="cid:foto@mail">` +
		`<img src="https://tracker.example/pixel.gif">`

	safe := sanitizeOriginalHTML(in, false)
	out := rewriteCIDReferences(safe, map[string]string{
		"foto@mail": "https://api.replider.com/v1/media/file?a=acc&p=x&e=1&s=ab",
	})

	assert.Contains(t, out, `src="https://api.replider.com/v1/media/file`)
	assert.NotContains(t, out, "tracker.example")
	assert.NotContains(t, out, "cid:")
}

// The rendered document is cached: the message is immutable and the render
// used to cost two GCS round-trips per open.
func TestOriginalCache_StoresAndExpires(t *testing.T) {
	h := &EmailOriginalHandler{}

	_, ok := h.cachedOriginal("m1|false")
	assert.False(t, ok, "empty cache misses")

	h.storeOriginal("m1|false", "<p>doc</p>")
	got, ok := h.cachedOriginal("m1|false")
	assert.True(t, ok)
	assert.Equal(t, "<p>doc</p>", got)

	// The load_images variant is a different document under a different key.
	_, ok = h.cachedOriginal("m1|true")
	assert.False(t, ok)

	// Expired entries miss.
	h.cacheMu.Lock()
	e := h.cache["m1|false"]
	e.storedAt = e.storedAt.Add(-originalCacheTTL - time.Minute)
	h.cache["m1|false"] = e
	h.cacheMu.Unlock()
	_, ok = h.cachedOriginal("m1|false")
	assert.False(t, ok)
}

// A data-URI fallback document can reach MBs; those skip the cache rather
// than pinning megabytes per message in process memory.
func TestOriginalCache_OversizedDocumentIsNotCached(t *testing.T) {
	h := &EmailOriginalHandler{}
	h.storeOriginal("big|false", strings.Repeat("x", originalCacheMaxEntryBytes+1))
	_, ok := h.cachedOriginal("big|false")
	assert.False(t, ok)
}

func TestOriginalCache_EvictsOldestAtCapacity(t *testing.T) {
	h := &EmailOriginalHandler{}
	for i := 0; i < originalCacheMaxEntries; i++ {
		h.storeOriginal("k"+strconv.Itoa(i), "doc")
		// Make insertion order the age order despite coarse clocks.
		h.cacheMu.Lock()
		e := h.cache["k"+strconv.Itoa(i)]
		e.storedAt = e.storedAt.Add(time.Duration(i-originalCacheMaxEntries) * time.Second)
		h.cache["k"+strconv.Itoa(i)] = e
		h.cacheMu.Unlock()
	}
	h.storeOriginal("newest", "doc")

	_, oldest := h.cachedOriginal("k0")
	_, newest := h.cachedOriginal("newest")
	assert.False(t, oldest, "the oldest entry is evicted")
	assert.True(t, newest)
	h.cacheMu.Lock()
	assert.LessOrEqual(t, len(h.cache), originalCacheMaxEntries)
	h.cacheMu.Unlock()
}
