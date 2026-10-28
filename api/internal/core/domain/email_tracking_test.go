package domain

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A tracked link is by definition "our URL that sends you somewhere else",
// which is an open redirect unless the destination is signed. These tests are
// mostly about that.

const trackSecret = "segredo-de-teste"

func clickToken(url string) TrackToken {
	return TrackToken{
		Kind: TrackClick, MessageID: "msg1", TemplateID: "tpl1",
		Version: 2, URL: url,
	}
}

func TestTrackToken_RoundTrips(t *testing.T) {
	in := clickToken("https://clinica.com.br/proposta")

	got, err := ParseTrackToken(trackSecret, MintTrackToken(trackSecret, in))

	require.NoError(t, err)
	assert.Equal(t, in, got)
}

// The whole point: nobody can point a link on our domain at their own page.
//
// Written as the actual attack — decode a legitimate token, swap the
// destination, keep the original signature — rather than by scrambling
// characters, which can flip nothing and pass by accident.
func TestTrackToken_DestinationCannotBeSwapped(t *testing.T) {
	legit := MintTrackToken(trackSecret, clickToken("https://clinica.com.br"))

	raw, err := base64.RawURLEncoding.DecodeString(legit)
	require.NoError(t, err)
	parts := strings.Split(string(raw), "\x1f")
	require.Len(t, parts, 6)

	// Point it at the attacker's page, keep everything else including the
	// signature.
	parts[4] = "https://phishing.example/roubo"
	forged := base64.RawURLEncoding.EncodeToString([]byte(strings.Join(parts, "\x1f")))

	_, err = ParseTrackToken(trackSecret, forged)

	assert.ErrorIs(t, err, ErrInvalidTrackToken)
}

// And the same for the message it claims to belong to: a valid token must not
// be re-pointed at another message to poison its numbers.
func TestTrackToken_MessageCannotBeSwapped(t *testing.T) {
	legit := MintTrackToken(trackSecret, clickToken("https://clinica.com.br"))

	raw, _ := base64.RawURLEncoding.DecodeString(legit)
	parts := strings.Split(string(raw), "\x1f")
	parts[1] = "outra-mensagem"
	forged := base64.RawURLEncoding.EncodeToString([]byte(strings.Join(parts, "\x1f")))

	_, err := ParseTrackToken(trackSecret, forged)

	assert.ErrorIs(t, err, ErrInvalidTrackToken)
}

func TestTrackToken_RejectsAForeignSignature(t *testing.T) {
	tok := MintTrackToken("outro-segredo", clickToken("https://clinica.com.br"))

	_, err := ParseTrackToken(trackSecret, tok)

	assert.ErrorIs(t, err, ErrInvalidTrackToken)
}

func TestTrackToken_RejectsGarbage(t *testing.T) {
	for _, raw := range []string{"", "   ", "não-base64!!", "YWJj"} {
		_, err := ParseTrackToken(trackSecret, raw)
		assert.ErrorIs(t, err, ErrInvalidTrackToken, "input %q", raw)
	}
}

// The signature guarantees nobody TAMPERED with the destination. It
// guarantees nothing about what we ourselves signed — so a bug that wrapped a
// javascript: href would produce a valid token pointing at a script.
func TestTrackToken_RefusesANonHTTPDestinationEvenWhenSigned(t *testing.T) {
	for _, bad := range []string{
		"javascript:steal()",
		"data:text/html,<script>x</script>",
		"file:///etc/passwd",
		"//evil.com",
	} {
		tok := MintTrackToken(trackSecret, clickToken(bad))
		_, err := ParseTrackToken(trackSecret, tok)
		assert.ErrorIs(t, err, ErrInvalidTrackToken, "destination %q", bad)
	}
}

// A click with no destination cannot redirect anywhere, and an open with one
// would be a redirect wearing a pixel's clothes.
func TestTrackToken_KindAndDestinationMustAgree(t *testing.T) {
	clickWithout := MintTrackToken(trackSecret,
		TrackToken{Kind: TrackClick, MessageID: "m", Version: 1})
	_, err := ParseTrackToken(trackSecret, clickWithout)
	assert.ErrorIs(t, err, ErrInvalidTrackToken)

	openWith := MintTrackToken(trackSecret,
		TrackToken{Kind: TrackOpen, MessageID: "m", Version: 1, URL: "https://x.com"})
	_, err = ParseTrackToken(trackSecret, openWith)
	assert.ErrorIs(t, err, ErrInvalidTrackToken)
}

// ── Link wrapping ─────────────────────────────────────────────────────────

func TestWrapLinks_RewritesHTTPLinks(t *testing.T) {
	html := `<p>Veja a <a href="https://clinica.com.br/proposta">proposta</a></p>`

	got := WrapLinks(html, trackSecret, "https://track.replider.com",
		TrackToken{MessageID: "msg1", TemplateID: "tpl1", Version: 2})

	assert.Contains(t, got, "https://track.replider.com/t/c/")
	assert.NotContains(t, got, `href="https://clinica.com.br/proposta"`)
	// The anchor text is untouched: the reader must see where they are going.
	assert.Contains(t, got, ">proposta</a>")
}

// A mailto: or tel: link is not a click worth measuring, and routing one
// through a web redirect would simply break it.
func TestWrapLinks_LeavesNonWebLinksAlone(t *testing.T) {
	html := `<a href="mailto:a@b.com">e-mail</a><a href="tel:+5515999990000">ligar</a>`

	got := WrapLinks(html, trackSecret, "https://track.replider.com", TrackToken{})

	assert.Equal(t, html, got)
}

// A broken opt-out is not a lost metric — it is a complaint to the mailbox
// provider and a hit to the sender's reputation.
func TestWrapLinks_NeverWrapsAnUnsubscribeLink(t *testing.T) {
	for _, url := range []string{
		"https://clinica.com.br/unsubscribe?x=1",
		"https://clinica.com.br/descadastrar",
		"https://clinica.com.br/opt-out",
		"https://clinica.com.br/cancelar-inscricao",
	} {
		html := `<a href="` + url + `">sair</a>`
		got := WrapLinks(html, trackSecret, "https://track.replider.com", TrackToken{})
		assert.Equal(t, html, got, "must not wrap %q", url)
	}
}

// No tracking domain configured means no tracking, not a broken link.
func TestWrapLinks_NoBaseURLIsANoOp(t *testing.T) {
	html := `<a href="https://clinica.com.br">x</a>`

	assert.Equal(t, html, WrapLinks(html, trackSecret, "", TrackToken{}))
	assert.Empty(t, OpenPixelTag(trackSecret, "", TrackToken{}))
}

// A wrapped link must survive a round trip to the exact destination it
// started as.
func TestWrapLinks_DestinationSurvivesTheRoundTrip(t *testing.T) {
	const dest = "https://clinica.com.br/proposta?id=42&v=2"
	html := `<a href="` + dest + `">ver</a>`

	wrapped := WrapLinks(html, trackSecret, "https://track.x.com",
		TrackToken{MessageID: "msg1", TemplateID: "tpl1", Version: 3})

	start := strings.Index(wrapped, "/t/c/") + len("/t/c/")
	end := strings.Index(wrapped[start:], `"`) + start
	got, err := ParseTrackToken(trackSecret, wrapped[start:end])

	require.NoError(t, err)
	assert.Equal(t, dest, got.URL)
	assert.Equal(t, 3, got.Version)
}

func TestOpenPixelTag_IsInvisibleAndNotBroken(t *testing.T) {
	got := OpenPixelTag(trackSecret, "https://track.x.com",
		TrackToken{MessageID: "msg1", Version: 1})

	assert.Contains(t, got, "/t/o/")
	// Without explicit dimensions and alt some clients render a broken-image
	// placeholder, and a mystery broken image in a cold email is exactly what
	// makes a stranger delete it.
	assert.Contains(t, got, `width="1"`)
	assert.Contains(t, got, `height="1"`)
	assert.Contains(t, got, `alt=""`)
}
