package email

import (
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Two shapes of failure, four providers, and one rule that outranks all of
// them: never classify HARD on a guess. A wrongly suppressed address is
// silent — nobody notices the email that was never sent.

func TestParseBounce_ResendHardBounce(t *testing.T) {
	body := []byte(`{
	  "type": "email.bounced",
	  "data": {
	    "to": ["dono@clinicafechada.com.br"],
	    "bounce": {
	      "type": "Permanent",
	      "subType": "General",
	      "diagnosticCode": "smtp; 550 5.1.1 <dono@clinicafechada.com.br>: Recipient address rejected: User unknown"
	    }
	  }
	}`)

	b, ok := ParseBounce(body)

	require.True(t, ok)
	assert.Equal(t, domain.BounceHard, b.Kind)
	assert.Equal(t, "dono@clinicafechada.com.br", b.Recipient)
	assert.Contains(t, b.Diagnostic, "550 5.1.1")
}

func TestParseBounce_ResendComplaintIsNotJustAHardBounce(t *testing.T) {
	body := []byte(`{"type":"email.complained","data":{"to":["dono@clinica.com.br"]}}`)

	b, ok := ParseBounce(body)

	require.True(t, ok)
	assert.Equal(t, domain.BounceComplaint, b.Kind)
	assert.True(t, b.Kind.Permanent())
}

func TestParseBounce_MailgunTemporaryIsSoft(t *testing.T) {
	body := []byte(`{
	  "event-data": {
	    "event": "failed",
	    "severity": "temporary",
	    "recipient": "dono@clinica.com.br",
	    "delivery-status": {"code": 452, "description": "Mailbox full"}
	  }
	}`)

	b, ok := ParseBounce(body)

	require.True(t, ok)
	assert.Equal(t, domain.BounceSoft, b.Kind)
	assert.False(t, b.Kind.Permanent(), "a full mailbox is emptied; the address is fine")
}

// SendGrid posts a batch of events.
func TestParseBounce_SendGridBatch(t *testing.T) {
	body := []byte(`[
	  {"email":"a@x.com","event":"delivered"},
	  {"email":"dono@clinica.com.br","event":"bounce","reason":"550 unknown user","status":"5.1.1"}
	]`)

	b, ok := ParseBounce(body)

	require.True(t, ok)
	assert.Equal(t, "dono@clinica.com.br", b.Recipient)
	assert.Equal(t, domain.BounceHard, b.Kind)
}

func TestParseBounce_PostmarkHardBounce(t *testing.T) {
	body := []byte(`{
	  "RecordType": "Bounce",
	  "Type": "HardBounce",
	  "Email": "dono@clinicafechada.com.br",
	  "Description": "The server was unable to deliver your message",
	  "Details": "550 5.1.1 mailbox unavailable"
	}`)

	b, ok := ParseBounce(body)

	require.True(t, ok)
	assert.Equal(t, domain.BounceHard, b.Kind)
	assert.Equal(t, "dono@clinicafechada.com.br", b.Recipient)
}

// An unsubscribe is not a complaint. Recording one as "reported us as spam"
// is a lie the customer would read in the panel.
func TestParseBounce_PostmarkSubscriptionChangeIsNotABounce(t *testing.T) {
	_, ok := ParseBounce([]byte(`{"RecordType":"SubscriptionChange","Email":"a@x.com"}`))
	assert.False(t, ok)
}

// The whole reason event names are matched exactly: a success must never be
// read as a failure, or every delivered message would suppress its recipient.
func TestParseBounce_SuccessEventsAreNotBounces(t *testing.T) {
	for _, body := range []string{
		`{"type":"email.delivered","data":{"to":["a@x.com"]}}`,
		`{"type":"email.sent","data":{"to":["a@x.com"]}}`,
		`{"type":"email.opened","data":{"to":["a@x.com"]}}`,
		`{"event-data":{"event":"delivered","recipient":"a@x.com"}}`,
		`[{"email":"a@x.com","event":"open"}]`,
	} {
		_, ok := ParseBounce([]byte(body))
		assert.False(t, ok, "must not read a success as a bounce: %s", body)
	}
}

// An ordinary inbound message must fall through to the normal parser.
func TestParseBounce_OrdinaryMessageIsNotABounce(t *testing.T) {
	_, ok := ParseBounce([]byte(`{"from":"dono@clinica.com.br","text":"Bom dia!"}`))
	assert.False(t, ok)
}

func TestParseBounce_NoRecipientIsUnusable(t *testing.T) {
	_, ok := ParseBounce([]byte(`{"type":"email.bounced","data":{}}`))
	assert.False(t, ok, "a bounce naming nobody cannot suppress anything")
}

func TestParseBounce_PicksUpTheOriginalMessageID(t *testing.T) {
	body := []byte(`{
	  "type":"email.bounced",
	  "data":{
	    "to":["dono@clinica.com.br"],
	    "headers":[{"name":"Message-ID","value":"<abc@clinica.com.br>"}]
	  }
	}`)

	b, _ := ParseBounce(body)

	assert.Equal(t, "<abc@clinica.com.br>", b.OriginalMessageID)
}

// Mailgun echoes the id without brackets; ours carry them, so an unbracketed
// id would never match the message we sent.
func TestParseBounce_RestoresStrippedAngleBrackets(t *testing.T) {
	body := []byte(`{
	  "event-data":{
	    "event":"failed","severity":"permanent","recipient":"dono@clinica.com.br",
	    "message":{"headers":{"message-id":"abc@clinica.com.br"}}
	  }
	}`)

	b, _ := ParseBounce(body)

	assert.Equal(t, "<abc@clinica.com.br>", b.OriginalMessageID)
}

// ── Classification ────────────────────────────────────────────────────────

func TestClassifyBounce_StatusCodeBeatsProse(t *testing.T) {
	// The prose says "temporarily", the code says permanent. The code wins:
	// prose is written by whichever server rejected us and is not a contract.
	assert.Equal(t, domain.BounceHard,
		domain.ClassifyBounce("", "550 5.1.1 temporarily unknown user"))
}

func TestClassifyBounce_FourHundredIsTransient(t *testing.T) {
	assert.Equal(t, domain.BounceSoft, domain.ClassifyBounce("", "4.2.2 over quota"))
	assert.Equal(t, domain.BounceSoft, domain.ClassifyBounce("", "452 too many connections"))
}

// The rule that protects real addresses: silence is SOFT. A soft bounce costs
// one retry; a wrong hard bounce costs the address forever.
func TestClassifyBounce_UnknownIsSoft(t *testing.T) {
	for _, d := range []string{"", "algo deu errado", "delivery problem"} {
		assert.Equal(t, domain.BounceSoft, domain.ClassifyBounce("", d), "input %q", d)
	}
}

func TestClassifyBounce_ProseFallbackWhenNoCode(t *testing.T) {
	assert.Equal(t, domain.BounceHard, domain.ClassifyBounce("", "User unknown"))
	assert.Equal(t, domain.BounceHard, domain.ClassifyBounce("", "usuário desconhecido"))
	assert.Equal(t, domain.BounceSoft, domain.ClassifyBounce("", "mailbox full, try again"))
}

// ── DSN (a bounce arriving as an ordinary email) ──────────────────────────

const dsnBody = `This is the mail system at host mx.provider.com.

I'm sorry to have to inform you that your message could not
be delivered to one or more recipients.

Final-Recipient: rfc822; dono@clinicafechada.com.br
Original-Message-ID: <enviado123@clinicasaolucas.com.br>
Action: failed
Status: 5.1.1
Diagnostic-Code: smtp; 550 5.1.1 User unknown
`

func TestLooksLikeDSN_RecognisesAMailerDaemonReport(t *testing.T) {
	e := InboundEmail{
		From:    "mailer-daemon@provider.com",
		Subject: "Undelivered Mail Returned to Sender",
		Body:    dsnBody,
	}

	assert.True(t, LooksLikeDSN(e))
}

// Without this the report becomes a conversation with a robot as the
// contact, and the agent replies to a mail server.
func TestParseDSN_ExtractsTheFailedAddressNotTheDaemon(t *testing.T) {
	e := InboundEmail{
		From:    "mailer-daemon@provider.com",
		Subject: "Undelivered Mail Returned to Sender",
		Body:    dsnBody,
	}

	b, ok := ParseDSN(e)

	require.True(t, ok)
	assert.Equal(t, "dono@clinicafechada.com.br", b.Recipient)
	assert.NotEqual(t, "mailer-daemon@provider.com", b.Recipient)
	assert.Equal(t, domain.BounceHard, b.Kind)
	assert.Equal(t, "<enviado123@clinicasaolucas.com.br>", b.OriginalMessageID)
}

// A person writing "a entrega falhou" is not a delivery report.
func TestLooksLikeDSN_LeavesRealPeopleAlone(t *testing.T) {
	e := InboundEmail{
		From:    "dono@clinica.com.br",
		Subject: "Re: proposta — a entrega falhou aqui",
		Body:    "Bom dia! O material não chegou, podem reenviar?",
	}

	assert.False(t, LooksLikeDSN(e))
}

// noreply@ is used by plenty of legitimate senders. The sender alone must
// never be enough.
func TestLooksLikeDSN_NoreplyAloneIsNotAReport(t *testing.T) {
	e := InboundEmail{
		From:    "noreply@fornecedor.com.br",
		Subject: "Seu pedido foi confirmado",
		Body:    "Obrigado pela compra.",
	}

	assert.False(t, LooksLikeDSN(e))
}

// A machine-readable report is proof no matter who relayed it.
func TestLooksLikeDSN_ReportBodyIsProofOnItsOwn(t *testing.T) {
	e := InboundEmail{
		From:    "bounces@relay.com.br",
		Subject: "(no subject)",
		Body:    dsnBody,
	}

	assert.True(t, LooksLikeDSN(e))
}

// A delayed-delivery warning is NOT a failure — the message is still queued.
func TestParseDSN_DelayedIsSoft(t *testing.T) {
	e := InboundEmail{
		From:    "mailer-daemon@provider.com",
		Subject: "Delivery Status Notification (Delay)",
		Body: `Final-Recipient: rfc822; dono@clinica.com.br
Action: delayed
Status: 4.4.1
Diagnostic-Code: smtp; 421 connection timed out
`,
	}

	b, ok := ParseDSN(e)

	require.True(t, ok)
	assert.Equal(t, domain.BounceSoft, b.Kind)
	assert.False(t, b.Kind.Permanent())
}

// A report we cannot read the failed address out of suppresses nothing —
// suppressing the mail server itself would be worse than dropping it.
func TestParseDSN_WithoutAFailedAddressIsUnusable(t *testing.T) {
	e := InboundEmail{
		From:    "mailer-daemon@provider.com",
		Subject: "Undelivered Mail Returned to Sender",
		Body:    "Something went wrong. Status: 5.1.1",
	}

	_, ok := ParseDSN(e)

	assert.False(t, ok)
}

// Falls back to the threading header when the body omits the original id.
func TestParseDSN_FallsBackToInReplyTo(t *testing.T) {
	e := InboundEmail{
		From:      "mailer-daemon@provider.com",
		Subject:   "Undeliverable",
		InReplyTo: "<enviado999@clinicasaolucas.com.br>",
		Body: `Final-Recipient: rfc822; dono@clinica.com.br
Status: 5.2.1
`,
	}

	b, _ := ParseDSN(e)

	assert.Equal(t, "<enviado999@clinicasaolucas.com.br>", b.OriginalMessageID)
}

// ── Auto-replies ──────────────────────────────────────────────────────────

// An out-of-office is not a reply. Stored, it becomes a contact, a lead in
// "responded" and the end of a cadence — three wrong facts from a message
// nobody wrote.
func TestIsAutoSubmitted_RecognisesTheMarkers(t *testing.T) {
	cases := []struct {
		name string
		e    InboundEmail
		want bool
	}{
		{"a plain reply", InboundEmail{}, false},
		{"Auto-Submitted: no is the RFC's way of saying a person wrote it",
			InboundEmail{AutoSubmitted: "no"}, false},
		{"Auto-Submitted: auto-replied", InboundEmail{AutoSubmitted: "auto-replied"}, true},
		{"Auto-Submitted: auto-generated", InboundEmail{AutoSubmitted: "auto-generated"}, true},
		{"case does not matter", InboundEmail{AutoSubmitted: "AUTO-REPLIED"}, true},
		{"parameters after the verdict", InboundEmail{AutoSubmitted: "auto-replied; owner-token=abc"}, true},
		{"a vendor header, whatever its value", InboundEmail{HasAutoReplyHeader: true}, true},
		{"Precedence: bulk", InboundEmail{Precedence: "bulk"}, true},
		{"Precedence: Bulk", InboundEmail{Precedence: "Bulk"}, true},
		{"Precedence: list", InboundEmail{Precedence: "list"}, true},
		{"Precedence: junk", InboundEmail{Precedence: "junk"}, true},
		{"Precedence: auto_reply", InboundEmail{Precedence: "auto_reply"}, true},
		{"Precedence: normal", InboundEmail{Precedence: "normal"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsAutoSubmitted(tc.e))
		})
	}
}

// IMAP is the path that actually sees these headers, so the raw parser has
// to lift them out or the gate downstream has nothing to read.
func TestParseRawInbound_CapturesTheAutoReplyMarkers(t *testing.T) {
	raw := "From: Ana Souza <ana@clinicaaurora.com.br>\r\n" +
		"To: contato@fornecedor.com.br\r\n" +
		"Subject: Resposta automática: proposta de luvas\r\n" +
		"Auto-Submitted: auto-replied\r\n" +
		"Precedence: bulk\r\n" +
		"X-Autoreply: yes\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Estou de férias até 15/09.\r\n"

	e, err := ParseRawInbound([]byte(raw))
	require.NoError(t, err)

	assert.Equal(t, "auto-replied", e.AutoSubmitted)
	assert.Equal(t, "bulk", e.Precedence)
	assert.True(t, e.HasAutoReplyHeader)
	assert.True(t, IsAutoSubmitted(e))
}

// A person's reply carries none of them, and must not trip the gate.
func TestParseRawInbound_APlainReplyIsNotAutoSubmitted(t *testing.T) {
	raw := "From: Ana Souza <ana@clinicaaurora.com.br>\r\n" +
		"Subject: Re: proposta de luvas\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Pode mandar o orçamento.\r\n"

	e, err := ParseRawInbound([]byte(raw))
	require.NoError(t, err)

	assert.False(t, IsAutoSubmitted(e))
}

// A provider that forwards the raw headers gets the same treatment; one that
// does not leaves the fields empty, which is "not marked".
func TestParseInboundWebhook_ReadsTheAutoReplyMarkersFromForwardedHeaders(t *testing.T) {
	e, ok := ParseInboundWebhook([]byte(`{
		"from": "ana@clinicaaurora.com.br",
		"text": "Estou de férias",
		"headers": {"Auto-Submitted": "auto-replied", "X-Auto-Response-Suppress": "All"}
	}`))
	require.True(t, ok)
	assert.Equal(t, "auto-replied", e.AutoSubmitted)
	assert.True(t, e.HasAutoReplyHeader)
	assert.True(t, IsAutoSubmitted(e))

	plain, ok := ParseInboundWebhook([]byte(`{"from": "ana@clinicaaurora.com.br", "text": "Oi"}`))
	require.True(t, ok)
	assert.False(t, IsAutoSubmitted(plain))
}
