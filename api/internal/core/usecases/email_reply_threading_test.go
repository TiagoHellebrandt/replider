package usecases

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestEmailReplyMetadata(t *testing.T) {
	t.Run("threads onto the inbound and derives a single Re: subject", func(t *testing.T) {
		inbound := &domain.Message{Metadata: map[string]any{
			"message_id": "<abc@hospital.br>",
			"references": "<root@hospital.br>",
			"subject":    "Orçamento tomógrafo",
		}}
		got := EmailReplyMetadata(inbound, "")
		assert.Equal(t, "<abc@hospital.br>", got["in_reply_to"])
		assert.Equal(t, "<root@hospital.br> <abc@hospital.br>", got["references"])
		assert.Equal(t, "Re: Orçamento tomógrafo", got["subject"])
	})

	t.Run("does not stack Re: and keeps the attendant's subject", func(t *testing.T) {
		inbound := &domain.Message{Metadata: map[string]any{
			"message_id": "<abc@x.br>", "subject": "RE: já tem Re",
		}}
		assert.Equal(t, "RE: já tem Re", EmailReplyMetadata(inbound, "")["subject"])
		assert.Equal(t, "Custom", EmailReplyMetadata(inbound, "  Custom ")["subject"])
	})

	t.Run("references falls back to message_id and is not duplicated", func(t *testing.T) {
		got := EmailReplyMetadata(&domain.Message{Metadata: map[string]any{"message_id": "<a@x>"}}, "")
		assert.Equal(t, "<a@x>", got["references"])
		got = EmailReplyMetadata(&domain.Message{Metadata: map[string]any{
			"message_id": "<a@x>", "references": "<r@x> <a@x>",
		}}, "")
		assert.Equal(t, "<r@x> <a@x>", got["references"])
	})

	t.Run("reads raw JSON metadata as stored by the DB", func(t *testing.T) {
		raw := json.RawMessage(`{"message_id":"<db@x.br>","subject":"Pedido"}`)
		got := EmailReplyMetadata(&domain.Message{Metadata: raw}, "")
		assert.Equal(t, "<db@x.br>", got["in_reply_to"])
		assert.Equal(t, "Re: Pedido", got["subject"])
	})

	t.Run("nil inbound: only the caller's subject, nil when nothing", func(t *testing.T) {
		assert.Nil(t, EmailReplyMetadata(nil, ""))
		assert.Equal(t, map[string]any{"subject": "Olá"}, EmailReplyMetadata(nil, "Olá"))
	})
}

func TestEmailReplyRecipient(t *testing.T) {
	assert.Equal(t, "from@x.br", EmailReplyRecipient(nil, "from@x.br"))
	assert.Equal(t, "from@x.br", EmailReplyRecipient(&domain.Message{Metadata: map[string]any{"reply_to": "not an address"}}, "from@x.br"))
	assert.Equal(t, "compras@hospital.br", EmailReplyRecipient(&domain.Message{Metadata: map[string]any{"reply_to": "compras@hospital.br"}}, "from@x.br"))
	assert.Equal(t, "compras@hospital.br", EmailReplyRecipient(&domain.Message{Metadata: json.RawMessage(`{"reply_to":" compras@hospital.br "}`)}, "from@x.br"))
}

// The bug: a human reply to an inbound email went out without
// In-Reply-To / References, so Outlook filed it outside the thread.
func TestSendHumanMessage_EmailThreading(t *testing.T) {
	ctx := context.Background()
	accountID, convID, userID := "acc-1", "conv-1", "user-1"
	conv := &domain.Conversation{
		ID: convID, AccountID: accountID, ChannelID: "chan-email",
		ContactID: "comprador@hospital.br", IsBotPaused: false,
	}
	emailChannel := &domain.Channel{ID: "chan-email", AccountID: accountID, Type: "email"}

	newUC := func(provider *stubProvider) (*ConversationTakeoverUseCase, *MockConversationRepo, *MockMessageRepo, *MockChannelRepo) {
		convRepo := new(MockConversationRepo)
		msgRepo := new(MockMessageRepo)
		channelRepo := new(MockChannelRepo)
		uc := NewConversationTakeoverUseCase(convRepo, msgRepo, channelRepo, nil,
			map[string]domain.MessageProvider{"email": provider}, nil)
		convRepo.On("GetByID", ctx, convID, accountID).Return(conv, nil).Once()
		channelRepo.On("GetByID", ctx, "chan-email").Return(emailChannel, nil).Once()
		convRepo.On("SetFirstResponse", ctx, convID, accountID, "human").Return(nil).Maybe()
		convRepo.On("ClearWaitingSince", ctx, convID, accountID).Return(nil).Maybe()
		msgRepo.On("Create", ctx, mock.AnythingOfType("*domain.Message")).Return(true, nil).Once()
		return uc, convRepo, msgRepo, channelRepo
	}

	t.Run("email: in_reply_to, references and Re: subject from the last inbound", func(t *testing.T) {
		provider := &stubProvider{}
		uc, _, msgRepo, _ := newUC(provider)
		msgRepo.On("LastInboundInConversation", ctx, convID).Return(&domain.Message{
			Direction: "inbound",
			Metadata:  json.RawMessage(`{"message_id":"<in1@hospital.br>","references":"<root@replider>","subject":"Proposta"}`),
		}, nil).Once()

		msg, err := uc.SendHumanMessage(ctx, SendHumanMessageDTO{
			ConversationID: convID, AccountID: accountID, UserID: userID, Content: "Segue.",
		})
		assert.NoError(t, err)
		meta, ok := msg.Metadata.(map[string]any)
		assert.True(t, ok, "metadata must be a map: %T", msg.Metadata)
		assert.Equal(t, "<in1@hospital.br>", meta["in_reply_to"])
		assert.Equal(t, "<root@replider> <in1@hospital.br>", meta["references"])
		assert.Equal(t, "Re: Proposta", meta["subject"])
		assert.Equal(t, []string{"comprador@hospital.br"}, provider.to, "no reply_to: send to the contact")
		assert.Same(t, msg, provider.sent[0], "the adapter sees the threaded message")
	})

	t.Run("email: explicit subject wins, threading headers still set", func(t *testing.T) {
		provider := &stubProvider{}
		uc, _, msgRepo, _ := newUC(provider)
		msgRepo.On("LastInboundInConversation", ctx, convID).Return(&domain.Message{
			Metadata: map[string]any{"message_id": "<in2@x.br>", "subject": "Antigo"},
		}, nil).Once()

		msg, err := uc.SendHumanMessage(ctx, SendHumanMessageDTO{
			ConversationID: convID, AccountID: accountID, UserID: userID,
			Content: "x", Subject: "Novo assunto",
		})
		assert.NoError(t, err)
		meta := msg.Metadata.(map[string]any)
		assert.Equal(t, "Novo assunto", meta["subject"])
		assert.Equal(t, "<in2@x.br>", meta["in_reply_to"])
	})

	t.Run("email: valid reply_to overrides the contact address", func(t *testing.T) {
		provider := &stubProvider{}
		uc, _, msgRepo, _ := newUC(provider)
		msgRepo.On("LastInboundInConversation", ctx, convID).Return(&domain.Message{
			Metadata: map[string]any{"message_id": "<in3@x.br>", "reply_to": "compras@hospital.br"},
		}, nil).Once()

		_, err := uc.SendHumanMessage(ctx, SendHumanMessageDTO{
			ConversationID: convID, AccountID: accountID, UserID: userID, Content: "x",
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"compras@hospital.br"}, provider.to)
	})

	t.Run("email: no inbound yet — no threading, still sends to the contact", func(t *testing.T) {
		provider := &stubProvider{}
		uc, _, msgRepo, _ := newUC(provider)
		msgRepo.On("LastInboundInConversation", ctx, convID).Return(nil, nil).Once()

		msg, err := uc.SendHumanMessage(ctx, SendHumanMessageDTO{
			ConversationID: convID, AccountID: accountID, UserID: userID, Content: "x", Subject: "Oi",
		})
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"subject": "Oi"}, msg.Metadata)
		assert.Equal(t, []string{"comprador@hospital.br"}, provider.to)
	})

	t.Run("non-email channel: no inbound lookup, no metadata", func(t *testing.T) {
		convRepo := new(MockConversationRepo)
		msgRepo := new(MockMessageRepo)
		channelRepo := new(MockChannelRepo)
		provider := &stubProvider{}
		uc := NewConversationTakeoverUseCase(convRepo, msgRepo, channelRepo, nil,
			map[string]domain.MessageProvider{"telegram": provider}, nil)
		tg := *conv
		tg.IsBotPaused = true
		tg.ContactID = "12345"
		convRepo.On("GetByID", ctx, convID, accountID).Return(&tg, nil).Once()
		channelRepo.On("GetByID", ctx, "chan-email").Return(&domain.Channel{ID: "chan-email", Type: "telegram"}, nil).Once()
		convRepo.On("SetFirstResponse", ctx, convID, accountID, "human").Return(nil).Maybe()
		convRepo.On("ClearWaitingSince", ctx, convID, accountID).Return(nil).Maybe()
		msgRepo.On("Create", ctx, mock.AnythingOfType("*domain.Message")).Return(true, nil).Once()

		msg, err := uc.SendHumanMessage(ctx, SendHumanMessageDTO{
			ConversationID: convID, AccountID: accountID, UserID: userID, Content: "x", Subject: "ignored",
		})
		assert.NoError(t, err)
		assert.Nil(t, msg.Metadata)
		assert.Equal(t, []string{"12345"}, provider.to)
		msgRepo.AssertNotCalled(t, "LastInboundInConversation", mock.Anything, mock.Anything)
	})
}
