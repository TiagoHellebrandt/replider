package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-M003/M004. Email is the only channel here with a subject and a thread,
// and both are deliverability-relevant: a missing subject is a spam signal,
// and missing In-Reply-To/References makes every reply start a new thread in
// the recipient's client — which reads as a bot and kills the conversation.

type captureTransport struct {
	sends int
	from  string
	to    []string
	raw   string
	err   error
}

func (c *captureTransport) Send(_ context.Context, from string, to []string, raw []byte) error {
	c.sends++
	c.from = from
	c.to = to
	c.raw = string(raw)
	return c.err
}

func emailChannel(cfg map[string]any) *domain.Channel {
	raw, _ := json.Marshal(cfg)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return &domain.Channel{
		ID:        "ch-1",
		AccountID: "acc-1",
		Name:      "Comercial",
		Type:      "email",
		Config:    m,
	}
}

func validConfig() map[string]any {
	return map[string]any{
		"from_address":  "comercial@clinicasaolucas.com.br",
		"from_name":     "Equipe Comercial",
		"smtp_host":     "smtp.provider.com",
		"smtp_username": "apikey",
		"smtp_password": "secret",
	}
}

func newTestAdapter(t *testing.T) (*Adapter, *captureTransport) {
	t.Helper()
	cap := &captureTransport{}
	a := NewAdapter().WithTransport(func(smtpConfig) transport { return cap })
	return a, cap
}

func str(s string) *string { return &s }

func TestEmailAdapter_SendsOneMessageWithHeaders(t *testing.T) {
	a, cap := newTestAdapter(t)
	msg := &domain.Message{
		Type:      "text",
		Direction: "outbound",
		Content:   str("Bom dia! Vi que a clínica atende convênios.\n\nPodemos conversar?"),
		Metadata:  map[string]any{MetaSubject: "Equipamentos para a Clínica São Lucas"},
	}

	err := a.SendMessage(context.Background(), emailChannel(validConfig()), "compras@hospital.com.br", msg)
	require.NoError(t, err)

	// ONE email, never bubble-split: three emails in a row reads as a
	// malfunction, not as a chat.
	assert.Equal(t, 1, cap.sends)
	assert.Equal(t, "comercial@clinicasaolucas.com.br", cap.from)
	assert.Equal(t, []string{"compras@hospital.com.br"}, cap.to)

	assert.Contains(t, cap.raw, `From: "Equipe Comercial" <comercial@clinicasaolucas.com.br>`)
	assert.Contains(t, cap.raw, "To: compras@hospital.com.br")
	assert.Contains(t, cap.raw, "Content-Type: text/plain; charset=UTF-8")
	// Body normalized to CRLF — a bare LF breaks strict SMTP servers.
	assert.Contains(t, cap.raw, "Podemos conversar?\r\n")

	// The Message-ID is rooted at the SENDER's domain: a mismatch with the
	// envelope is a spam signal.
	meta, ok := msg.Metadata.(map[string]any)
	require.True(t, ok)
	msgID, _ := meta[MetaMessageID].(string)
	assert.True(t, strings.HasSuffix(msgID, "@clinicasaolucas.com.br>"), "got %q", msgID)
	assert.Contains(t, cap.raw, "Message-ID: "+msgID)
}

// A subject with accents must survive transport; a raw UTF-8 subject header
// is not legal and gets mangled by some servers.
func TestEmailAdapter_EncodesAccentedSubject(t *testing.T) {
	a, cap := newTestAdapter(t)
	msg := &domain.Message{
		Content:  str("olá"),
		Metadata: map[string]any{MetaSubject: "Proposta de manutenção — São Paulo"},
	}
	require.NoError(t, a.SendMessage(context.Background(), emailChannel(validConfig()), "x@y.com.br", msg))

	assert.Contains(t, cap.raw, "Subject: =?UTF-8?q?")
	assert.NotContains(t, cap.raw, "Subject: Proposta de manutenção")
}

// No subject at all is a spam signal, so the adapter always supplies one.
func TestEmailAdapter_FallsBackToADefaultSubject(t *testing.T) {
	a, cap := newTestAdapter(t)
	msg := &domain.Message{Content: str("corpo")}
	require.NoError(t, a.SendMessage(context.Background(), emailChannel(validConfig()), "x@y.com.br", msg))

	assert.Contains(t, cap.raw, "Subject: ")
	assert.NotContains(t, cap.raw, "Subject: \r\n")
	meta, _ := msg.Metadata.(map[string]any)
	assert.NotEmpty(t, meta[MetaSubject])
}

// The whole point of threading: a reply must land inside the same thread,
// with References accumulating the chain (Gmail fragments long threads
// without it).
func TestEmailAdapter_ThreadsReplies(t *testing.T) {
	a, cap := newTestAdapter(t)
	msg := &domain.Message{
		Content: str("Perfeito, segue a proposta."),
		Metadata: map[string]any{
			MetaSubject:    "Re: Equipamentos",
			MetaInReplyTo:  "<abc@hospital.com.br>",
			MetaReferences: "<first@clinicasaolucas.com.br>",
		},
	}
	require.NoError(t, a.SendMessage(context.Background(), emailChannel(validConfig()), "compras@hospital.com.br", msg))

	assert.Contains(t, cap.raw, "In-Reply-To: <abc@hospital.com.br>")
	assert.Contains(t, cap.raw, "References: <first@clinicasaolucas.com.br> <abc@hospital.com.br>")
}

func TestEmailAdapter_ReferencesDefaultsToInReplyTo(t *testing.T) {
	a, cap := newTestAdapter(t)
	msg := &domain.Message{
		Content:  str("ok"),
		Metadata: map[string]any{MetaInReplyTo: "<abc@hospital.com.br>"},
	}
	require.NoError(t, a.SendMessage(context.Background(), emailChannel(validConfig()), "x@y.com.br", msg))
	assert.Contains(t, cap.raw, "References: <abc@hospital.com.br>")
}

// Metadata arrives from the DB as raw JSON, not as a Go map — reading it
// naively would silently drop the subject and the threading headers.
func TestEmailAdapter_ReadsMetadataFromRawJSON(t *testing.T) {
	a, cap := newTestAdapter(t)
	msg := &domain.Message{
		Content:  str("corpo"),
		Metadata: json.RawMessage(`{"subject":"Assunto vindo do banco","in_reply_to":"<db@x.com>"}`),
	}
	require.NoError(t, a.SendMessage(context.Background(), emailChannel(validConfig()), "x@y.com.br", msg))

	assert.Contains(t, cap.raw, "In-Reply-To: <db@x.com>")
	assert.Contains(t, cap.raw, mimeEncodedContains(t, cap.raw, "Assunto"))
}

// mimeEncodedContains is a small helper: the subject is encoded, so assert on
// the header's presence rather than on the plain text.
func mimeEncodedContains(t *testing.T, raw, _ string) string {
	t.Helper()
	for _, line := range strings.Split(raw, "\r\n") {
		if strings.HasPrefix(line, "Subject: ") {
			return line
		}
	}
	return "Subject:"
}

func TestEmailAdapter_RejectsEmptyBody(t *testing.T) {
	a, cap := newTestAdapter(t)
	err := a.SendMessage(context.Background(), emailChannel(validConfig()), "x@y.com.br",
		&domain.Message{Content: str("   ")})

	require.Error(t, err)
	assert.Zero(t, cap.sends, "an empty email burns a send and looks broken to the recipient")
}

func TestEmailAdapter_RejectsInvalidRecipient(t *testing.T) {
	a, cap := newTestAdapter(t)
	err := a.SendMessage(context.Background(), emailChannel(validConfig()), "not-an-address",
		&domain.Message{Content: str("oi")})

	require.Error(t, err)
	assert.Zero(t, cap.sends)
}

func TestEmailAdapter_RejectsNonEmailChannelAndMissingConfig(t *testing.T) {
	a, _ := newTestAdapter(t)

	err := a.SendMessage(context.Background(), &domain.Channel{Type: "whatsapp"}, "x@y.com", &domain.Message{Content: str("oi")})
	assert.ErrorIs(t, err, ErrChannelNotEmail)

	incomplete := validConfig()
	delete(incomplete, "smtp_host")
	err = a.SendMessage(context.Background(), emailChannel(incomplete), "x@y.com", &domain.Message{Content: str("oi")})
	assert.ErrorIs(t, err, ErrEmailNotConfigured)
}

// Inbound email arrives on the webhook route, not through the provider
// interface — same contract as the internal CS channel.
func TestEmailAdapter_WebhookIsRejected(t *testing.T) {
	a, _ := newTestAdapter(t)
	err := a.HandleWebhook(context.Background(), emailChannel(validConfig()), []byte("{}"))
	assert.ErrorIs(t, err, ErrEmailNoWebhook)
}

// Channel-level validation (US-M003) refuses senders that cannot be
// authenticated: cold outbound from a consumer mailbox fails DMARC alignment
// and lands in spam.
func TestDomain_FreeMailProvidersAreRejectedAsSenders(t *testing.T) {
	for _, addr := range []string{"vendas@gmail.com", "contato@hotmail.com", "x@uol.com.br"} {
		assert.True(t, domain.IsFreeMailProvider(addr), addr)
	}
	assert.False(t, domain.IsFreeMailProvider("comercial@clinicasaolucas.com.br"))
	assert.Equal(t, "clinicasaolucas.com.br", domain.EmailDomain("Comercial@ClinicaSaoLucas.com.br "))
}

// A token the provider refused is dropped from the cache, so the next send
// refreshes from the grant instead of failing for the rest of the hour.
type forgettingTokens struct {
	forgot []string
}

func (f *forgettingTokens) AccessToken(context.Context, *domain.Channel) (string, error) {
	return "ya29.dead", nil
}
func (f *forgettingTokens) Forget(channelID string) { f.forgot = append(f.forgot, channelID) }

func TestEmailAdapter_ForgetsCachedTokenWhenTheProviderRefusesIt(t *testing.T) {
	a, cap := newTestAdapter(t)
	cap.err = fmt.Errorf("%w: 535 invalid credentials", domain.ErrEmailOAuthNotConnected)
	tokens := &forgettingTokens{}
	a.SetOAuthTokens(tokens)
	ch := emailChannel(map[string]any{
		"auth_method":         "google_oauth",
		"oauth_refresh_token": "refresh",
		"from_address":        "dona@gmail.com",
	})

	err := a.SendMessage(context.Background(), ch, "cliente@hospital.com.br", &domain.Message{Content: str("Olá")})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrEmailOAuthNotConnected)
	assert.Equal(t, []string{"ch-1"}, tokens.forgot)

	// Any other transport failure keeps the (still valid) token.
	tokens.forgot = nil
	cap.err = errors.New("smtp RCPT: 450 try later")
	err = a.SendMessage(context.Background(), ch, "cliente@hospital.com.br", &domain.Message{Content: str("Olá")})
	require.Error(t, err)
	assert.Empty(t, tokens.forgot)
}
