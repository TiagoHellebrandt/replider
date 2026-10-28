package email

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A message read off a mailbox arrives as bytes a stranger's mail client wrote.
// These pin the parts a reply is actually made of, because each one has a
// specific way of going wrong that ends with the salesperson never seeing an
// answer they were sent.

func TestParseRawInbound_ReadsAPlainReply(t *testing.T) {
	raw := strings.Join([]string{
		"From: Ana Souza <ana@clinicaaurora.com.br>",
		"To: contato@fornecedor.com.br",
		"Subject: Re: proposta de luvas",
		"Message-Id: <reply-1@clinicaaurora.com.br>",
		"In-Reply-To: <original-1@fornecedor.com.br>",
		"References: <original-1@fornecedor.com.br>",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Bom dia, pode mandar o orçamento.",
	}, "\r\n")

	got, err := ParseRawInbound([]byte(raw))

	require.NoError(t, err)
	assert.Equal(t, "ana@clinicaaurora.com.br", got.From)
	assert.Equal(t, "Ana Souza", got.FromName)
	assert.Equal(t, "Re: proposta de luvas", got.Subject)
	assert.Contains(t, got.Body, "pode mandar o orçamento")
	// Threading: without these the recipient's client files our next message as
	// a new conversation, and the thread the salesperson is reading splits in
	// two.
	assert.Equal(t, "<original-1@fornecedor.com.br>", got.InReplyTo)
	assert.Equal(t, "<original-1@fornecedor.com.br>", got.References)
	assert.Equal(t, "<reply-1@clinicaaurora.com.br>", got.MessageID)
}

// The subject is where accented Portuguese lives, and it arrives encoded.
// Leaving it raw shows the operator "=?utf-8?B?T3LDp2FtZW50bw==?=".
func TestParseRawInbound_DecodesEncodedHeaders(t *testing.T) {
	raw := strings.Join([]string{
		"From: =?utf-8?B?Sm9zw6kgQW5kcmFkZQ==?= <jose@clinica.com.br>",
		"Subject: =?utf-8?B?T3LDp2FtZW50byBhcHJvdmFkbw==?=",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"ok",
	}, "\r\n")

	got, err := ParseRawInbound([]byte(raw))

	require.NoError(t, err)
	assert.Equal(t, "Orçamento aprovado", got.Subject)
	assert.Equal(t, "José Andrade", got.FromName)
}

// Quoted-printable is what nearly every client uses for accented text.
// Undecoded, "orçamento" reads as "or=C3=A7amento".
func TestParseRawInbound_DecodesQuotedPrintable(t *testing.T) {
	raw := strings.Join([]string{
		"From: ana@clinica.com.br",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		"Aprovado o or=C3=A7amento, pode faturar.",
	}, "\r\n")

	got, err := ParseRawInbound([]byte(raw))

	require.NoError(t, err)
	assert.Contains(t, got.Body, "orçamento")
}

// Older Brazilian clients still send Latin-1, and that is exactly where the
// cedilla and the tildes are.
func TestParseRawInbound_ReadsLatin1Bodies(t *testing.T) {
	body := latin1Bytes("Aprovado o orçamento, pode faturar.")
	raw := append([]byte(strings.Join([]string{
		"From: ana@clinica.com.br",
		"Content-Type: text/plain; charset=iso-8859-1",
		"",
		"",
	}, "\r\n")), body...)

	got, err := ParseRawInbound(raw)

	require.NoError(t, err)
	assert.Contains(t, got.Body, "orçamento")
}

// multipart/alternative: the plain part is the one to store, and the HTML is
// kept separately for "view original" — never rendered directly.
func TestParseRawInbound_PrefersThePlainPartAndKeepsTheHTML(t *testing.T) {
	raw := strings.Join([]string{
		"From: ana@clinica.com.br",
		"Content-Type: multipart/alternative; boundary=BOUND",
		"",
		"--BOUND",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"versão em texto",
		"--BOUND",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<p>versão em <b>html</b></p>",
		"--BOUND--",
	}, "\r\n")

	got, err := ParseRawInbound([]byte(raw))

	require.NoError(t, err)
	assert.Contains(t, got.Body, "versão em texto")
	assert.Contains(t, got.HTML, "<b>html</b>")
	assert.False(t, got.HadHTMLOnly)
}

// HTML-only is common from marketing tools and from Outlook. The body has to be
// derived rather than left empty, and the derivation flagged.
func TestParseRawInbound_DerivesTextWhenOnlyHTMLCame(t *testing.T) {
	raw := strings.Join([]string{
		"From: ana@clinica.com.br",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<p>Podemos fechar <b>hoje</b>.</p>",
	}, "\r\n")

	got, err := ParseRawInbound([]byte(raw))

	require.NoError(t, err)
	assert.Contains(t, got.Body, "Podemos fechar")
	assert.NotContains(t, got.Body, "<b>")
	assert.True(t, got.HadHTMLOnly, "the operator should know the text was derived")
}

// "Segue em anexo" with nothing attached is the mistake this product flags in
// its own drafts. Dropping a real attachment on the way IN produces the same
// confusion from the other side: the customer says they sent the document and
// the screen shows nothing.
func TestParseRawInbound_KeepsAttachmentsAndTheirNames(t *testing.T) {
	raw := strings.Join([]string{
		"From: ana@clinica.com.br",
		"Content-Type: multipart/mixed; boundary=BOUND",
		"",
		"--BOUND",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Segue o pedido.",
		"--BOUND",
		"Content-Type: application/pdf",
		`Content-Disposition: attachment; filename="pedido 1234.pdf"`,
		"Content-Transfer-Encoding: base64",
		"",
		"JVBERi0xLjQK",
		"--BOUND--",
	}, "\r\n")

	got, err := ParseRawInbound([]byte(raw))

	require.NoError(t, err)
	assert.Contains(t, got.Body, "Segue o pedido")
	require.Len(t, got.Attachments, 1)
	assert.Equal(t, "pedido 1234.pdf", got.Attachments[0].Filename)
	assert.Equal(t, "application/pdf", got.Attachments[0].ContentType)
	// Decoded, not the base64 text: the bytes are what gets stored and handed
	// back to the operator.
	assert.Equal(t, "%PDF-1.4\n", string(got.Attachments[0].Content))
}

// A signature logo is not "an anexo". Conflating them makes every reply look
// like it carried a document.
func TestParseRawInbound_SeparatesInlineFurnitureFromRealFiles(t *testing.T) {
	raw := strings.Join([]string{
		"From: ana@clinica.com.br",
		"Content-Type: multipart/related; boundary=BOUND",
		"",
		"--BOUND",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Obrigada!",
		"--BOUND",
		"Content-Type: image/png",
		"Content-Disposition: inline",
		"Content-Id: <logo@clinica>",
		"Content-Transfer-Encoding: base64",
		"",
		"iVBORw0KGgo=",
		"--BOUND--",
	}, "\r\n")

	got, err := ParseRawInbound([]byte(raw))

	require.NoError(t, err)
	require.Len(t, got.Attachments, 1)
	assert.Equal(t, "logo@clinica", got.Attachments[0].ContentID)
	assert.Empty(t, got.RealAttachments(), "a signature logo is not a document")
}

// A .txt or .csv attachment is a FILE. Treating it as the body would replace
// the customer's reply with the contents of their spreadsheet.
func TestParseRawInbound_ANamedTextPartIsAFileNotTheBody(t *testing.T) {
	raw := strings.Join([]string{
		"From: ana@clinica.com.br",
		"Content-Type: multipart/mixed; boundary=BOUND",
		"",
		"--BOUND",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Segue a lista.",
		"--BOUND",
		"Content-Type: text/csv",
		`Content-Disposition: attachment; filename="itens.csv"`,
		"",
		"sku,qtd",
		"--BOUND--",
	}, "\r\n")

	got, err := ParseRawInbound([]byte(raw))

	require.NoError(t, err)
	assert.Equal(t, "Segue a lista.", strings.TrimSpace(got.Body))
	require.Len(t, got.Attachments, 1)
	assert.Equal(t, "itens.csv", got.Attachments[0].Filename)
}

// The verdict the CUSTOMER's own server reached, reading the real sending IP.
// This is what IMAP preserves and a forwarding bridge destroys.
func TestParseRawInbound_CarriesTheReceivingServersVerdict(t *testing.T) {
	raw := strings.Join([]string{
		"From: ana@clinicaaurora.com.br",
		"Authentication-Results: mx.locaweb.com.br; spf=pass smtp.mailfrom=clinicaaurora.com.br; dkim=pass header.d=clinicaaurora.com.br; dmarc=pass",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"ok",
	}, "\r\n")

	got, err := ParseRawInbound([]byte(raw))

	require.NoError(t, err)
	assert.True(t, got.Auth.Trustworthy())
	assert.False(t, got.Auth.Suspicious())
}

func TestParseRawInbound_FlagsBulkMailByItsHeader(t *testing.T) {
	raw := strings.Join([]string{
		"From: news@fornecedor.com.br",
		"List-Unsubscribe: <https://fornecedor.com.br/sair>",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"novidades",
	}, "\r\n")

	got, err := ParseRawInbound([]byte(raw))

	require.NoError(t, err)
	assert.True(t, got.ListUnsubscribe)
}

// The pipeline keys a contact on the sender address, so a message with no
// readable From has nowhere to go. Refused here rather than filed under an
// empty identity, where it would silently merge strangers into one contact.
func TestParseRawInbound_RefusesWhatItCannotAttribute(t *testing.T) {
	for name, raw := range map[string]string{
		"no from":       "Subject: oi\r\n\r\ncorpo",
		"not a message": "isto não é um e-mail",
		"empty":         "",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseRawInbound([]byte(raw))
			assert.ErrorIs(t, err, ErrRawEmailUnparseable)
		})
	}
}

// A reply quoting the original produces several text parts. Joining them glues
// the customer's answer to a copy of our own message.
func TestParseRawInbound_TakesTheFirstTextPartNotAllOfThem(t *testing.T) {
	raw := strings.Join([]string{
		"From: ana@clinica.com.br",
		"Content-Type: multipart/mixed; boundary=BOUND",
		"",
		"--BOUND",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Aprovado.",
		"--BOUND",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Em 8 de agosto, Fornecedor escreveu: nossa proposta é...",
		"--BOUND--",
	}, "\r\n")

	got, err := ParseRawInbound([]byte(raw))

	require.NoError(t, err)
	assert.Equal(t, "Aprovado.", strings.TrimSpace(got.Body))
	assert.NotContains(t, got.Body, "nossa proposta")
}

// Nesting is bounded: a crafted message can nest thousands deep, and each level
// costs a reader and a buffer.
func TestParseRawInbound_BoundsNesting(t *testing.T) {
	var b strings.Builder
	b.WriteString("From: ana@clinica.com.br\r\n")
	b.WriteString("Content-Type: multipart/mixed; boundary=B0\r\n\r\n")
	depth := maxMIMEDepth + 5
	for i := 0; i < depth; i++ {
		b.WriteString("--B0\r\n")
		b.WriteString("Content-Type: multipart/mixed; boundary=B0\r\n\r\n")
	}
	b.WriteString("--B0--\r\n")

	// The only requirement is that it returns instead of recursing forever.
	_, err := ParseRawInbound([]byte(b.String()))
	assert.NoError(t, err)
}

func latin1Bytes(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r < 256 {
			out = append(out, byte(r))
		}
	}
	return out
}

// The 256KB cap exists for TEXT (quoted-thread bodies). Applied to
// attachments it silently dropped every phone photo: no row, no log, and
// the email view had nothing to render — found in production on the first
// real photo a customer sent (2026-08-26).
func TestParseRawInbound_AttachmentBiggerThanTextCapSurvives(t *testing.T) {
	big := bytes.Repeat([]byte{0xFF}, 300<<10) // > maxRawTextBytes
	b64 := base64.StdEncoding.EncodeToString(big)
	raw := strings.Join([]string{
		"From: tiago@gmail.com",
		"Content-Type: multipart/related; boundary=BOUND",
		"",
		"--BOUND",
		"Content-Type: text/plain",
		"",
		"olha a imagem",
		"--BOUND",
		"Content-Type: image/jpeg",
		"Content-Disposition: inline; filename=foto.jpg",
		"Content-Id: <foto1@gmail>",
		"Content-Transfer-Encoding: base64",
		"",
		b64,
		"--BOUND--",
	}, "\r\n")

	parsed, err := ParseRawInbound([]byte(raw))
	require.NoError(t, err)
	require.Len(t, parsed.Attachments, 1)
	att := parsed.Attachments[0]
	assert.Equal(t, len(big), len(att.Content), "the full bytes, not a truncation")
	assert.True(t, att.IsInline())
	assert.Equal(t, "foto1@gmail", att.ContentID)
}

// Gmail stamps a Content-ID on REGULAR attachments too. The id alone must
// not flag a contract as message furniture — that hid it from the
// attachment counter, which is exactly the indicator the anti-scam posture
// trains people to trust.
func TestParseRawInbound_ContentIDAloneDoesNotMakeADocumentInline(t *testing.T) {
	raw := strings.Join([]string{
		"From: tiago@gmail.com",
		"Content-Type: multipart/mixed; boundary=BOUND",
		"",
		"--BOUND",
		"Content-Type: text/plain",
		"",
		"segue o contrato",
		"--BOUND",
		"Content-Type: application/pdf",
		"Content-Disposition: attachment; filename=contrato.pdf",
		"Content-Id: <f_abc123>",
		"Content-Transfer-Encoding: base64",
		"",
		base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 fake")),
		"--BOUND--",
	}, "\r\n")

	parsed, err := ParseRawInbound([]byte(raw))
	require.NoError(t, err)
	require.Len(t, parsed.Attachments, 1)
	att := parsed.Attachments[0]
	assert.False(t, att.IsInline(), "a PDF is a document, whatever ids Gmail stamps on it")
	assert.Equal(t, "f_abc123", att.ContentID, "the id still travels for cid: rewriting")
}
