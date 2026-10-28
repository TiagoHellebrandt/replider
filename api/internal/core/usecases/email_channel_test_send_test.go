package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The test send has two jobs, and the tests below separate them on purpose:
// telling the customer whether their credentials work, and RECORDING that they
// do. The second is what lifts the cold-send block with no DNS, so a success
// that failed to record must not be reported as if it had.

type stubEmailSender struct {
	err error
	// to and subject capture what actually left, because "it sent" is not the
	// claim under test — "it sent to the account's own address" is.
	to      string
	subject string
	body    string
	calls   int
}

func (s *stubEmailSender) SendMessage(_ context.Context, _ *domain.Channel, to string, msg *domain.Message) error {
	s.calls++
	s.to = to
	if msg.Content != nil {
		s.body = *msg.Content
	}
	if m, ok := msg.Metadata.(map[string]any); ok {
		s.subject, _ = m["subject"].(string)
	}
	return s.err
}

func (s *stubEmailSender) HandleWebhook(context.Context, *domain.Channel, []byte) error { return nil }

type stubChannelGetter struct {
	channel *domain.Channel
	err     error
	domain.ChannelRepository
}

func (s *stubChannelGetter) GetByID(context.Context, string) (*domain.Channel, error) {
	return s.channel, s.err
}

func emailChannelFixture(t *testing.T, from string) *domain.Channel {
	t.Helper()
	cfg, err := json.Marshal(map[string]string{
		"from_address":  from,
		"smtp_host":     "email-ssl.com.br",
		"smtp_port":     "465",
		"smtp_username": from,
		"smtp_password": "segredo",
	})
	require.NoError(t, err)
	return &domain.Channel{
		ID: "ch-1", AccountID: "acc-1", Type: "email", Config: json.RawMessage(cfg),
	}
}

func newTestSendFixture(t *testing.T, from string) (*TestSendEmailChannelUseCase, *stubEmailSender, *fakeMailboxRepo) {
	t.Helper()
	sender := &stubEmailSender{}
	mailboxes := &fakeMailboxRepo{mailbox: &domain.EmailMailbox{
		ID: "mb-1", AccountID: "acc-1", ChannelID: "ch-1", Address: from,
	}}
	uc := NewTestSendEmailChannelUseCase(
		&stubChannelGetter{channel: emailChannelFixture(t, from)}, sender)
	uc.SetMailboxRepo(mailboxes)
	uc.now = func() time.Time { return time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC) }
	return uc, sender, mailboxes
}

// The probe goes to the account's OWN address. A test that mails a stranger is
// a test nobody runs twice, and the whole point is that it is safe to retry
// until the credentials are right.
func TestEmailTestSend_ProbesTheAccountsOwnAddress(t *testing.T) {
	uc, sender, _ := newTestSendFixture(t, "contato@clinica.com.br")

	out, err := uc.Execute(context.Background(), "acc-1", "ch-1")

	require.NoError(t, err)
	assert.Equal(t, 1, sender.calls)
	assert.Equal(t, "contato@clinica.com.br", sender.to)
	assert.Equal(t, "contato@clinica.com.br", out.SentTo)
	// Self-describing: this lands in their inbox, and an unexplained message
	// from their own address reads as a compromised account.
	assert.NotEmpty(t, sender.subject)
	assert.Contains(t, sender.body, "teste")
	assert.Contains(t, sender.body, "nenhuma outra pessoa foi contatada")
}

// A successful send IS the authentication for this transport: the customer's
// provider accepted an authenticated session and signs the mail with its own
// DKIM. So the test lifts the cold-send block, with no DNS anywhere.
func TestEmailTestSend_SuccessProvesTheIdentityAndUnlocksColdSending(t *testing.T) {
	uc, _, mailboxes := newTestSendFixture(t, "contato@clinica.com.br")

	out, err := uc.Execute(context.Background(), "acc-1", "ch-1")

	require.NoError(t, err)
	require.NotNil(t, mailboxes.smtpProvenAt, "the proof has to be recorded")
	assert.True(t, out.ColdSendUnlocked)
	// The conservative ceiling, not the configurable one: we are borrowing
	// someone else's mailbox under a per-day limit their provider sets and
	// does not tell us.
	assert.Equal(t, domain.SMTPProvenDailyCap, out.DailyCap)
}

// The provider's own words survive to the customer. "535 Authentication
// failed" says check the password; "falha no envio" says open a ticket.
func TestEmailTestSend_CarriesTheProvidersRefusalVerbatim(t *testing.T) {
	uc, sender, mailboxes := newTestSendFixture(t, "contato@clinica.com.br")
	sender.err = errors.New("smtp auth: 535 Authentication failed")

	_, err := uc.Execute(context.Background(), "acc-1", "ch-1")

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "EMAIL_TEST_SEND_FAILED", appErr.Code)
	assert.Contains(t, appErr.Message, "535 Authentication failed")
	// A failed send proves nothing, so nothing is stamped — otherwise a wrong
	// password would unlock cold sending.
	assert.Nil(t, mailboxes.smtpProvenAt)
}

// The send worked; only our bookkeeping failed. Reporting that as a plain
// success would tell the customer they may now send, and the next approach
// would be refused with no explanation.
func TestEmailTestSend_AnUnrecordedProofIsNotReportedAsUnlocked(t *testing.T) {
	uc, _, mailboxes := newTestSendFixture(t, "contato@clinica.com.br")
	mailboxes.proveErr = errors.New("db down")

	out, err := uc.Execute(context.Background(), "acc-1", "ch-1")

	// Still no error: the question was "do my credentials work", and they do.
	require.NoError(t, err)
	assert.Equal(t, "contato@clinica.com.br", out.SentTo)
	assert.False(t, out.ColdSendUnlocked)
}

func TestEmailTestSend_RefusesChannelsWithNothingToProve(t *testing.T) {
	sender := &stubEmailSender{}
	uc := NewTestSendEmailChannelUseCase(
		&stubChannelGetter{channel: &domain.Channel{
			ID: "ch-1", AccountID: "acc-1", Type: "telegram",
		}}, sender)

	_, err := uc.Execute(context.Background(), "acc-1", "ch-1")

	assert.ErrorIs(t, err, ErrChannelNotEmail)
	assert.Zero(t, sender.calls)
}

// Another tenant's channel id is a not-found, never a send: the probe would
// otherwise let anyone make an arbitrary account's mail server dial out.
func TestEmailTestSend_AnotherAccountsChannelIsNotFound(t *testing.T) {
	uc, sender, _ := newTestSendFixture(t, "contato@clinica.com.br")

	_, err := uc.Execute(context.Background(), "outra-acc", "ch-1")

	assert.ErrorIs(t, err, domain.ErrChannelNotFound)
	assert.Zero(t, sender.calls)
}

func TestEmailTestSend_RefusesAChannelWithNoFromAddress(t *testing.T) {
	sender := &stubEmailSender{}
	uc := NewTestSendEmailChannelUseCase(
		&stubChannelGetter{channel: &domain.Channel{
			ID: "ch-1", AccountID: "acc-1", Type: "email",
			Config: json.RawMessage(`{"smtp_host":"email-ssl.com.br"}`),
		}}, sender)

	_, err := uc.Execute(context.Background(), "acc-1", "ch-1")

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "CHANNEL_MISSING_CONFIG", appErr.Code)
	assert.Zero(t, sender.calls)
}
