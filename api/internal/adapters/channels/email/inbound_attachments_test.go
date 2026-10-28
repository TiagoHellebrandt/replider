package email

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Attachments used to be dropped silently: a reply saying "segue em anexo o
// termo de referência" produced a message with nothing attached and no sign
// that anything had gone missing. These pin that they survive parsing.

func TestParseInbound_KeepsAttachments(t *testing.T) {
	payload := []byte(`{
	  "from": "compras@clinicavidanova.com.br",
	  "subject": "Re: monitores",
	  "text": "Segue em anexo o termo de referência.",
	  "attachments": [
	    {"filename": "termo.pdf", "content_type": "application/pdf",
	     "content": "` + base64.StdEncoding.EncodeToString([]byte("%PDF-1.7 fake")) + `"}
	  ]
	}`)

	in, ok := ParseInboundWebhook(payload)
	require.True(t, ok)
	require.Len(t, in.Attachments, 1)
	assert.Equal(t, "termo.pdf", in.Attachments[0].Filename)
	assert.Equal(t, "application/pdf", in.Attachments[0].ContentType)
	assert.Equal(t, "%PDF-1.7 fake", string(in.Attachments[0].Content))
	assert.Equal(t, int64(13), in.Attachments[0].Size, "size is derived when the provider omits it")
}

// Postmark's PascalCase must work like Resend's snake_case — one parser, not
// one package per vendor.
func TestParseInbound_AcceptsPascalCaseAttachments(t *testing.T) {
	payload := []byte(`{
	  "From": "dr@hospital.com.br",
	  "Attachments": [
	    {"Name": "orcamento.xlsx",
	     "ContentType": "application/vnd.ms-excel",
	     "ContentLength": 4,
	     "Content": "` + base64.StdEncoding.EncodeToString([]byte("xlsx")) + `"}
	  ]
	}`)

	in, ok := ParseInboundWebhook(payload)
	require.True(t, ok)
	require.Len(t, in.Attachments, 1)
	assert.Equal(t, "orcamento.xlsx", in.Attachments[0].Filename)
}

// An entry with neither bytes nor a URL is not an attachment — keeping it
// would put a row in the UI that cannot be opened.
func TestParseInbound_DropsUnusableAttachmentEntries(t *testing.T) {
	payload := []byte(`{"from":"a@b.com","attachments":[{"filename":"ghost.pdf"}]}`)

	in, ok := ParseInboundWebhook(payload)
	require.True(t, ok)
	assert.Empty(t, in.Attachments)
}

// A signature logo is not what a person means by "anexo". Counting it trains
// them to ignore the indicator, which is exactly when the real contract
// slips past.
func TestRealAttachments_ExcludesInlineFurniture(t *testing.T) {
	in := InboundEmail{Attachments: []InboundAttachment{
		// Inline follows the flag, not the mere presence of a ContentID —
		// Gmail stamps ids on regular attachments too.
		{Filename: "logo.png", ContentID: "logo@sig", Inline: true,
			Content: []byte("x")},
		{Filename: "contrato.pdf", Content: []byte("y")},
	}}

	real := in.RealAttachments()
	require.Len(t, real, 1)
	assert.Equal(t, "contrato.pdf", real[0].Filename)
}

// The filename comes from the SENDER, so it is hostile input.
func TestSafeFilename(t *testing.T) {
	cases := map[string]string{
		"../../etc/passwd":     "passwd",
		"/tmp/x/relatorio.pdf": "relatorio.pdf",
		"nota\nfiscal.pdf":     "nota_fiscal.pdf",
		"":                     "anexo",
		"...":                  "anexo",
		"Orçamento (2).pdf":    "Orçamento (2).pdf",
	}
	for in, want := range cases {
		assert.Equal(t, want, InboundAttachment{Filename: in}.SafeFilename(), "input %q", in)
	}
}

func TestSafeFilename_TruncatesButKeepsExtension(t *testing.T) {
	long := ""
	for i := 0; i < 300; i++ {
		long += "a"
	}
	got := InboundAttachment{Filename: long + ".pdf"}.SafeFilename()
	assert.LessOrEqual(t, len(got), 180)
	assert.Contains(t, got, ".pdf", "the extension is what tells the reader what the file is")
}

// ── Authentication ────────────────────────────────────────────────────────

func TestParseAuthResults_StandardHeader(t *testing.T) {
	auth := ParseAuthResults(
		"mx.google.com; spf=pass smtp.mailfrom=clinica.com.br; dkim=pass header.i=@clinica.com.br; dmarc=pass",
		nil)

	assert.Equal(t, AuthPass, auth.SPF)
	assert.Equal(t, AuthPass, auth.DKIM)
	assert.Equal(t, AuthPass, auth.DMARC)
	assert.True(t, auth.Trustworthy())
	assert.False(t, auth.Suspicious())
}

// The shape a forged sender takes.
func TestParseAuthResults_ForgedSenderIsSuspicious(t *testing.T) {
	auth := ParseAuthResults("mx; spf=fail; dkim=none; dmarc=fail", nil)

	assert.False(t, auth.Trustworthy())
	assert.True(t, auth.Suspicious())
}

// The most important case: nobody checked. It must not read as verified —
// that is the exact failure this feature exists to remove.
func TestParseAuthResults_UnknownIsNotTrustworthy(t *testing.T) {
	auth := ParseAuthResults("", nil)

	assert.Equal(t, AuthUnknown, auth.SPF)
	assert.Equal(t, AuthUnknown, auth.DKIM)
	assert.Equal(t, AuthUnknown, auth.DMARC)
	assert.False(t, auth.Trustworthy(), "an unchecked message is not a checked one")
	assert.False(t, auth.Suspicious(), "nor is it evidence of forgery")
}

// DMARC pass requires an aligned SPF or DKIM pass, so it subsumes both —
// including the forwarding case where SPF legitimately breaks.
func TestAuth_DMARCPassSurvivesBrokenSPF(t *testing.T) {
	auth := ParseAuthResults("mx; spf=softfail; dkim=pass; dmarc=pass", nil)

	assert.True(t, auth.Trustworthy())
	assert.False(t, auth.Suspicious(), "softfail is weak, not a failure")
}

// A DMARC failure outranks a passing SPF: the domain published a policy and
// this message broke it.
func TestAuth_DMARCFailOverridesSPFPass(t *testing.T) {
	auth := ParseAuthResults("mx; spf=pass; dmarc=fail", nil)

	assert.False(t, auth.Trustworthy())
	assert.True(t, auth.Suspicious())
}

func TestParseAuthResults_VendorFieldsFillTheGaps(t *testing.T) {
	auth := ParseAuthResults("", map[string]string{"SPF": "pass", "dkim": "pass"})

	assert.Equal(t, AuthPass, auth.SPF)
	assert.Equal(t, AuthPass, auth.DKIM)
	assert.True(t, auth.Trustworthy())
}

// A real Authentication-Results header always beats a vendor's summary.
func TestParseAuthResults_HeaderWinsOverVendor(t *testing.T) {
	auth := ParseAuthResults("mx; spf=fail", map[string]string{"spf": "pass"})

	assert.Equal(t, AuthFail, auth.SPF)
}

func TestParseInbound_ReadsAuthFromHeaders(t *testing.T) {
	payload := []byte(`{
	  "from": "compras@clinica.com.br",
	  "headers": {"Authentication-Results": "mx; spf=pass; dkim=pass; dmarc=pass"}
	}`)

	in, ok := ParseInboundWebhook(payload)
	require.True(t, ok)
	assert.True(t, in.Auth.Trustworthy())
}

// The original markup is kept, but ONLY in its own field — Body stays plain
// text so nothing can render markup by accident.
func TestParseInbound_KeepsOriginalHTMLSeparateFromBody(t *testing.T) {
	payload := []byte(`{"from":"a@b.com","html":"<p>Olá <b>doutor</b></p><script>alert(1)</script>"}`)

	in, ok := ParseInboundWebhook(payload)
	require.True(t, ok)
	assert.Contains(t, in.HTML, "<b>doutor</b>", "the original is preserved for 'ver original'")
	assert.NotContains(t, in.Body, "<b>", "the body a bubble renders stays text")
	assert.NotContains(t, in.Body, "alert(1)")
}

// Reply-To is where an ANSWER would go, which is not always where the message
// came from. A mismatch is the mechanic of a redirected reply, and it is a
// different header from In-Reply-To, which is threading.
func TestParseInbound_ExtractsReplyTo(t *testing.T) {
	payload := []byte(`{
	  "from": "compras@clinica.com.br",
	  "reply_to": "Financeiro <financeiro@outrodominio.com>",
	  "In-Reply-To": "<abc@clinica.com.br>"
	}`)

	in, ok := ParseInboundWebhook(payload)
	require.True(t, ok)
	assert.Equal(t, "financeiro@outrodominio.com", in.ReplyTo)
	assert.Equal(t, "<abc@clinica.com.br>", in.InReplyTo,
		"threading must not be confused with the reply address")
}

func TestParseInbound_ReplyToFromHeaders(t *testing.T) {
	payload := []byte(`{"from":"a@b.com","headers":{"Reply-To":"c@d.com"}}`)

	in, ok := ParseInboundWebhook(payload)
	require.True(t, ok)
	assert.Equal(t, "c@d.com", in.ReplyTo)
}
