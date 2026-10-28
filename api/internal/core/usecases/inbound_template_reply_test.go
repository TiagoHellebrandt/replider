package usecases

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// A reply is the only signal in email tracking worth planning around. These
// tests pin the hop that makes it countable: the customer's mail client
// echoes In-Reply-To, that resolves to the message we sent, and the message
// we sent remembers which template it used.

type recordingTemplateStats struct {
	domain.EmailTemplateRepository
	replies []string
	version int
}

func (r *recordingTemplateStats) RecordReply(_ context.Context, id string, version int) error {
	r.replies = append(r.replies, id)
	r.version = version
	return nil
}

func emailReplyDTO(inReplyTo string) InboundMessageDTO {
	dto := InboundMessageDTO{
		ChannelToken: "tok",
		SenderPhone:  "dono@clinica.com.br",
		MessageType:  "text",
		Content:      "Bom dia, pode me mandar os valores?",
	}
	if inReplyTo != "" {
		dto.Metadata, _ = json.Marshal(map[string]string{"in_reply_to": inReplyTo})
	}
	return dto
}

func inboundForReply(t *testing.T, stats domain.EmailTemplateRepository) (*InboundWebhookUseCase, *MockMessageRepo, *domain.Channel) {
	t.Helper()
	channelRepo := new(MockChannelRepo)
	convRepo := new(MockConversationRepo)
	convRepo.On("FindByContact", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	convRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo := new(MockMessageRepo)
	msgRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	uc := NewInboundWebhookUseCase(channelRepo, convRepo, msgRepo, nil, new(MockAgentRepo))
	if stats != nil {
		uc.SetEmailTemplateStats(stats)
	}
	return uc, msgRepo, &domain.Channel{
		AccountID: "acc-1", ID: "chan-email", Type: "email", IsActive: true,
	}
}

func sentWithTemplate(meta any) *domain.Message {
	return &domain.Message{ID: "msg-sent", Metadata: meta}
}

func TestSaveInbound_ReplyCreditsTheTemplateThatEarnedIt(t *testing.T) {
	stats := &recordingTemplateStats{}
	uc, msgRepo, channel := inboundForReply(t, stats)
	msgRepo.On("GetByExternalID", mock.Anything, "chan-email", "<abc@clinica.com.br>").
		Return(sentWithTemplate(map[string]any{
			"template_id": "tpl-frio-1", "template_version": float64(3),
		}), nil)

	_, err := uc.SaveInbound(context.Background(), channel, emailReplyDTO("<abc@clinica.com.br>"))

	assert.NoError(t, err)
	assert.Equal(t, []string{"tpl-frio-1"}, stats.replies)
	assert.Equal(t, 3, stats.version)
}

// Metadata read back from Postgres is raw JSONB, not a map. This is exactly
// the shape mismatch that made "view original" invisible in production once —
// same class of bug, one layer over.
func TestSaveInbound_ReplyCreditWorksOnMetadataFromPostgres(t *testing.T) {
	stats := &recordingTemplateStats{}
	uc, msgRepo, channel := inboundForReply(t, stats)
	msgRepo.On("GetByExternalID", mock.Anything, "chan-email", "<abc@clinica.com.br>").
		Return(sentWithTemplate(json.RawMessage(
			`{"subject":"Proposta","template_id":"tpl-frio-1","template_version":2}`)), nil)

	_, err := uc.SaveInbound(context.Background(), channel, emailReplyDTO("<abc@clinica.com.br>"))

	assert.NoError(t, err)
	assert.Equal(t, []string{"tpl-frio-1"}, stats.replies)
	assert.Equal(t, 2, stats.version)
}

// A message we sent by hand has no template, and there is nothing to credit.
func TestSaveInbound_ReplyToAHandWrittenMessageCreditsNothing(t *testing.T) {
	stats := &recordingTemplateStats{}
	uc, msgRepo, channel := inboundForReply(t, stats)
	msgRepo.On("GetByExternalID", mock.Anything, "chan-email", "<abc@clinica.com.br>").
		Return(sentWithTemplate(map[string]any{"subject": "Proposta"}), nil)

	_, err := uc.SaveInbound(context.Background(), channel, emailReplyDTO("<abc@clinica.com.br>"))

	assert.NoError(t, err)
	assert.Empty(t, stats.replies)
}

// A first inbound — nobody replying to anything — must not look up a parent
// at all, let alone credit one.
func TestSaveInbound_FirstContactCreditsNothing(t *testing.T) {
	stats := &recordingTemplateStats{}
	uc, msgRepo, channel := inboundForReply(t, stats)

	_, err := uc.SaveInbound(context.Background(), channel, emailReplyDTO(""))

	assert.NoError(t, err)
	assert.Empty(t, stats.replies)
	msgRepo.AssertNotCalled(t, "GetByExternalID", mock.Anything, mock.Anything, mock.Anything)
}

// The message is already persisted by the time this runs. A missing parent —
// a reply to something sent from the salesperson's own mailbox, say — is an
// ordinary case, not an error to surface.
func TestSaveInbound_UnknownParentIsNotAnError(t *testing.T) {
	stats := &recordingTemplateStats{}
	uc, msgRepo, channel := inboundForReply(t, stats)
	msgRepo.On("GetByExternalID", mock.Anything, "chan-email", "<sumiu@x.com>").
		Return(nil, nil)

	_, err := uc.SaveInbound(context.Background(), channel, emailReplyDTO("<sumiu@x.com>"))

	assert.NoError(t, err)
	assert.Empty(t, stats.replies)
}

// An opt-out is a termination, not a response — it must not be counted as a
// template's success. Mirrors the existing markLeadResponded rule.
func TestSaveInbound_OptOutIsNotCreditedAsAReply(t *testing.T) {
	stats := &recordingTemplateStats{}
	uc, msgRepo, channel := inboundForReply(t, stats)
	dto := emailReplyDTO("<abc@clinica.com.br>")
	dto.Content = "DESCADASTRAR"
	msgRepo.On("GetByExternalID", mock.Anything, mock.Anything, mock.Anything).
		Return(sentWithTemplate(map[string]any{
			"template_id": "tpl-frio-1", "template_version": float64(1),
		}), nil).Maybe()

	_, err := uc.SaveInbound(context.Background(), channel, dto)

	assert.NoError(t, err)
	assert.Empty(t, stats.replies)
}

// Unwired counters are a valid state — the numbers simply are not kept.
func TestSaveInbound_ReplyWithoutStatsWiredIsFine(t *testing.T) {
	uc, msgRepo, channel := inboundForReply(t, nil)
	// Threading (2026-09-01) legitimately looks the parent message up to
	// route the reply into its conversation; with no stats wired that is
	// the ONLY reason the lookup runs, and a miss must not be an error.
	msgRepo.On("GetByExternalID", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	_, err := uc.SaveInbound(context.Background(), channel, emailReplyDTO("<abc@clinica.com.br>"))

	assert.NoError(t, err)
}
