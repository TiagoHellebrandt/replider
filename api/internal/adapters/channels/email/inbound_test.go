package email

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-M005. Inbound mail is the one payload in this product written by a
// stranger and rendered in an attendant's browser, so the parser's job is as
// much containment as it is extraction.

func TestParseInbound_NormalizesCommonProviderShapes(t *testing.T) {
	// Resend/Mailgun-ish
	a, ok := ParseInboundWebhook([]byte(`{
		"from":"Setor de Compras <Compras@Hospital.com.BR>",
		"to":"comercial@clinica.com.br",
		"subject":"Re: Orçamento",
		"text":"Bom dia, pode enviar a proposta?",
		"message_id":"<abc@hospital.com.br>",
		"in_reply_to":"<xyz@clinica.com.br>"
	}`))
	require.True(t, ok)
	assert.Equal(t, "compras@hospital.com.br", a.From, "address is lowercased for identity resolution")
	assert.Equal(t, "Setor de Compras", a.FromName)
	assert.Equal(t, "Bom dia, pode enviar a proposta?", a.Body)
	assert.Equal(t, "<xyz@clinica.com.br>", a.InReplyTo)

	// Postmark-ish (different spellings, headers nested)
	b, ok := ParseInboundWebhook([]byte(`{
		"From":"x@y.com",
		"TextBody":"corpo",
		"MessageID":"<pm@y.com>",
		"headers":{"Subject":"Assunto","In-Reply-To":"<prev@y.com>","From":"x@y.com"}
	}`))
	require.True(t, ok)
	assert.Equal(t, "x@y.com", b.From)
	assert.Equal(t, "Assunto", b.Subject, "subject falls back to the raw headers")
	assert.Equal(t, "<prev@y.com>", b.InReplyTo)
}

func TestParseInbound_RejectsPayloadWithNoSender(t *testing.T) {
	_, ok := ParseInboundWebhook([]byte(`{"subject":"sem remetente","text":"oi"}`))
	assert.False(t, ok, "an unattributable message cannot become a conversation")

	_, ok = ParseInboundWebhook([]byte(`not json`))
	assert.False(t, ok)
}

// V4: inbound HTML is rendered in the dashboard. The parser converts to text
// rather than sanitizing tags — being right by construction beats being right
// about every tag, attribute and encoding trick forever.
func TestParseInbound_HTMLBecomesTextAndScriptsDieWithTheirContent(t *testing.T) {
	got, ok := ParseInboundWebhook([]byte(`{
		"from":"a@b.com",
		"html":"<html><head><style>.x{color:red}</style></head><body><p>Olá,</p><script>alert('xss')</script><p>segue a proposta.</p><br>Abraço</body></html>"
	}`))
	require.True(t, ok)
	assert.True(t, got.HadHTMLOnly)

	assert.NotContains(t, got.Body, "<")
	assert.NotContains(t, got.Body, "alert", "script CONTENT must die with the tag, not survive as text")
	assert.NotContains(t, got.Body, "color:red", "style content likewise")
	assert.Contains(t, got.Body, "Olá,")
	assert.Contains(t, got.Body, "segue a proposta.")
	assert.Contains(t, got.Body, "Abraço")
}

// Entities are decoded AFTER stripping: decoding first would let an escaped
// tag become a real one once the stripper had already run.
func TestParseInbound_EncodedTagsCannotResurrect(t *testing.T) {
	got, ok := ParseInboundWebhook([]byte(`{"from":"a@b.com","html":"<p>&lt;script&gt;alert(1)&lt;/script&gt; texto</p>"}`))
	require.True(t, ok)
	assert.NotContains(t, strings.ToLower(got.Body), "<script")
	assert.Contains(t, got.Body, "texto")
}

// A text/plain part always wins: it is what the sender's client produced,
// and converting HTML is lossy.
func TestParseInbound_PrefersPlainTextPart(t *testing.T) {
	got, ok := ParseInboundWebhook([]byte(`{"from":"a@b.com","text":"versão texto","html":"<p>versão html</p>"}`))
	require.True(t, ok)
	assert.Equal(t, "versão texto", got.Body)
	assert.False(t, got.HadHTMLOnly)
}

// Without this every reply re-feeds the whole thread to the AI (paying the
// same tokens again) and shows a wall of quoted text in the dashboard.
func TestStripQuotedReply_RemovesHistory(t *testing.T) {
	ptBR := "Perfeito, pode enviar.\n\nEm 12 de julho de 2026, Comercial <comercial@clinica.com.br> escreveu:\n> Segue a proposta em anexo\n> Abraço"
	assert.Equal(t, "Perfeito, pode enviar.", StripQuotedReply(ptBR))

	en := "Sounds good.\n\nOn Mon, Jul 12, 2026 at 9:00 AM Sales <s@c.com> wrote:\n> original"
	assert.Equal(t, "Sounds good.", StripQuotedReply(en))

	outlook := "Aprovado.\n\n-----Mensagem original-----\nDe: Comercial\nAssunto: Proposta"
	assert.Equal(t, "Aprovado.", StripQuotedReply(outlook))

	// Clients that quote with no separator at all.
	bare := "Ok, obrigado!\n> linha antiga\n> outra linha"
	assert.Equal(t, "Ok, obrigado!", StripQuotedReply(bare))

	// A message that is ONLY a quote must not be silently emptied into
	// something the pipeline mistakes for a real reply... it becomes empty,
	// and the handler skips it.
	assert.Empty(t, StripQuotedReply("> só citação\n> mais citação"))
}

func TestInboundEmail_ThreadAndReplyMetadata(t *testing.T) {
	in := InboundEmail{
		Subject:    "Orçamento de equipamentos",
		MessageID:  "<abc@hospital.com.br>",
		References: "<first@clinica.com.br>",
	}

	var stored map[string]any
	require.NoError(t, json.Unmarshal(in.ThreadMetadata(), &stored))
	assert.Equal(t, "<abc@hospital.com.br>", stored[MetaMessageID])

	reply := in.ReplyMetadata()
	assert.Equal(t, "Re: Orçamento de equipamentos", reply[MetaSubject])
	assert.Equal(t, "<abc@hospital.com.br>", reply[MetaInReplyTo])
	assert.Equal(t, "<first@clinica.com.br> <abc@hospital.com.br>", reply[MetaReferences],
		"References accumulates the chain")

	// "Re:" is added once — clients dislike "Re: Re: Re:".
	already := InboundEmail{Subject: "Re: Orçamento", MessageID: "<x@y.com>"}
	assert.Equal(t, "Re: Orçamento", already.ReplyMetadata()[MetaSubject])
}
