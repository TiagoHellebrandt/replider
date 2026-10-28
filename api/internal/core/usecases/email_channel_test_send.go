package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// TestSendEmailChannelUseCase proves an email channel's credentials by sending
// one message through them (US-M003 follow-up).
//
// Two jobs, and the second is the reason it exists at all:
//
//  1. Tell the customer NOW whether the host, port, user and password they
//     typed actually work. Before this, a wrong password surfaced on their
//     first real proposal — the operator approved a message, the send failed,
//     and the thing they were trying to look professional about was the thing
//     that broke.
//  2. Record the send as PROOF of the sending identity. Their provider
//     accepted an authenticated session and relayed mail signed with its own
//     DKIM for their domain; that is the honest authentication for this
//     transport, and it is what lifts the cold-send block without asking
//     anybody to touch DNS.
//
// The probe goes to the channel's OWN from_address. Nobody outside the account
// receives it, so a customer testing four times has not mailed a stranger four
// times — and a self-send still exercises the whole path (auth, TLS, envelope,
// headers, the provider's acceptance rules).
type TestSendEmailChannelUseCase struct {
	channelRepo domain.ChannelRepository
	sender      domain.MessageProvider
	// mailboxRepo records the proof. Optional: a nil repo still tests the
	// credentials and says so — losing the record of a successful test is
	// worse than refusing to run one, because the customer can always test
	// again but cannot debug a send they were never allowed to try.
	mailboxRepo domain.EmailMailboxRepository
	now         func() time.Time
}

func NewTestSendEmailChannelUseCase(
	channelRepo domain.ChannelRepository,
	sender domain.MessageProvider,
) *TestSendEmailChannelUseCase {
	return &TestSendEmailChannelUseCase{
		channelRepo: channelRepo,
		sender:      sender,
		now:         time.Now,
	}
}

// SetMailboxRepo wires the proof recorder.
func (uc *TestSendEmailChannelUseCase) SetMailboxRepo(r domain.EmailMailboxRepository) {
	uc.mailboxRepo = r
}

// TestSendResult is what the screen reports back.
type TestSendResult struct {
	// SentTo is the address that received the probe — always the channel's own
	// from_address. Echoed so the UI can say where to look for it: a success
	// message with no destination sends people hunting through the wrong inbox.
	SentTo string `json:"sent_to"`
	// ColdSendUnlocked reports whether this test lifted the cold-send block.
	// False when the proof could not be recorded, which is a different
	// situation from a failed send and must not be reported as success-with-no-
	// consequences.
	ColdSendUnlocked bool `json:"cold_send_unlocked"`
	// DailyCap is what the mailbox may send per day now. Stated because the
	// answer to "it worked, can I send my list?" is a number, and the customer
	// should learn it here rather than from a refusal halfway through.
	DailyCap int `json:"daily_cap"`
}

// ErrChannelNotEmail is returned for a test send against a channel that has no
// SMTP credentials to prove.
var ErrChannelNotEmail = &domain.AppError{
	Code:    "CHANNEL_NOT_EMAIL",
	Message: "só canais de e-mail têm envio de teste",
}

// ErrEmailTestSendFailed carries the provider's own refusal.
//
// The provider's words are kept verbatim in the message: "535 Authentication
// failed" tells the customer to check the password, and a generic "falha no
// envio de teste" tells them to open a support ticket.
var ErrEmailTestSendFailed = &domain.AppError{
	Code:    "EMAIL_TEST_SEND_FAILED",
	Message: "o servidor de e-mail recusou o envio de teste",
}

// Execute sends the probe and, on success, records the proof.
func (uc *TestSendEmailChannelUseCase) Execute(ctx context.Context, accountID, channelID string) (*TestSendResult, error) {
	channel, err := uc.channelRepo.GetByID(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if channel == nil || channel.AccountID != accountID {
		return nil, domain.ErrChannelNotFound
	}
	if channel.Type != "email" {
		return nil, ErrChannelNotEmail
	}

	from := strings.ToLower(strings.TrimSpace(channelStringField(channel, "from_address")))
	if from == "" {
		return nil, &domain.AppError{
			Code:    "CHANNEL_MISSING_CONFIG",
			Message: "canal de e-mail sem from_address configurado",
		}
	}

	body := testSendBody
	msg := &domain.Message{
		Type:      "text",
		Direction: "outbound",
		From:      "bot",
		Content:   &body,
		Metadata:  map[string]any{"subject": testSendSubject},
	}
	if err := uc.sender.SendMessage(ctx, channel, from, msg); err != nil {
		return nil, &domain.AppError{
			Code:    ErrEmailTestSendFailed.Code,
			Message: fmt.Sprintf("%s: %v", ErrEmailTestSendFailed.Message, err),
		}
	}

	result := &TestSendResult{SentTo: from}
	if uc.mailboxRepo == nil {
		return result, nil
	}

	now := uc.now()
	mailbox, err := uc.mailboxRepo.EnsureForChannel(ctx, accountID, channel.ID, from)
	if err != nil {
		// The send WORKED, which is the answer the customer asked for. Losing
		// the proof only means they stay on the blocked tier — reported
		// honestly via ColdSendUnlocked rather than swallowed.
		slog.WarnContext(ctx, "email test send: could not read mailbox",
			"channel_id", channel.ID, "error", err)
		return result, nil
	}
	if err := uc.mailboxRepo.MarkSMTPProven(ctx, accountID, from, now); err != nil {
		slog.WarnContext(ctx, "email test send: could not record proof",
			"channel_id", channel.ID, "error", err)
		return result, nil
	}

	// Reflect the stamp locally instead of re-reading: one round-trip less, and
	// the cap the customer is told is computed from the same rules the sender
	// will apply. MarkSMTPProven also stamps warmup_started_at, so the local
	// mirror does too.
	mailbox.SMTPVerifiedAt = &now
	if mailbox.WarmupStartedAt == nil {
		mailbox.WarmupStartedAt = &now
	}
	result.ColdSendUnlocked = mailbox.CanSend(now, true).Allowed
	result.DailyCap = mailbox.EffectiveDailyCap(now)
	return result, nil
}

// The probe is deliberately plain and self-describing: it lands in the
// customer's own inbox, and an unexplained empty message from their own
// address reads as a compromise.
const (
	testSendSubject = "Replider: teste de envio"
	testSendBody    = "Este é um teste de configuração do seu canal de e-mail no Replider.\n\n" +
		"Se você recebeu esta mensagem, o envio está funcionando e nenhuma outra pessoa foi contatada.\n\n" +
		"Você pode apagar este e-mail."
)
