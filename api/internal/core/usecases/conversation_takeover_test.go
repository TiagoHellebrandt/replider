package usecases

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Stub broadcaster that records payloads by account so we can assert
// the right WS event fires. Safe to share across test runs because we
// reset it per subtest.
type recordingBroadcaster struct {
	mu       sync.Mutex
	payloads map[string][][]byte
}

func newRecordingBroadcaster() *recordingBroadcaster {
	return &recordingBroadcaster{payloads: make(map[string][][]byte)}
}

func (b *recordingBroadcaster) BroadcastToAccount(accountID string, payload []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.payloads[accountID] = append(b.payloads[accountID], payload)
}

func (b *recordingBroadcaster) Count(accountID string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.payloads[accountID])
}

func (b *recordingBroadcaster) Last(accountID string) []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	ps := b.payloads[accountID]
	if len(ps) == 0 {
		return nil
	}
	return ps[len(ps)-1]
}

// stubProvider captures SendMessage calls so tests can assert they
// actually reached the adapter layer.
type stubProvider struct {
	sent []*domain.Message
	to   []string
	err  error
}

func (s *stubProvider) SendMessage(ctx context.Context, ch *domain.Channel, to string, msg *domain.Message) error {
	s.sent = append(s.sent, msg)
	s.to = append(s.to, to)
	return s.err
}
func (s *stubProvider) HandleWebhook(ctx context.Context, ch *domain.Channel, payload []byte) error {
	return nil
}

func TestAssumeConversation(t *testing.T) {
	ctx := context.Background()
	accountID := "acc-1"
	convID := "conv-1"
	userID := "user-1"

	baseConv := &domain.Conversation{
		ID:        convID,
		AccountID: accountID,
		ChannelID: "chan-1",
	}
	now := time.Now()

	t.Run("happy path: assumes, pauses bot, broadcasts event", func(t *testing.T) {
		convRepo := new(MockConversationRepo)
		broadcaster := newRecordingBroadcaster()
		uc := NewConversationTakeoverUseCase(convRepo, nil, nil, nil, nil, broadcaster)

		convRepo.On("GetByID", ctx, convID, accountID).Return(baseConv, nil).Once()
		convRepo.On("AssumeConversation", ctx, convID, accountID, userID).Return(nil).Once()
		convRepo.On("GetAssignment", ctx, convID, accountID).Return(&domain.ConversationAssignment{
			ConversationID: convID,
			AssignedTo:     &userID,
			AssignedAt:     &now,
			IsBotPaused:    true,
		}, nil).Once()

		res, err := uc.Assume(ctx, convID, accountID, userID)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.True(t, res.Assignment.IsBotPaused)
		assert.Equal(t, 1, broadcaster.Count(accountID))
		convRepo.AssertExpectations(t)
	})

	t.Run("ownership: returns NOT_FOUND when account doesn't own conversation", func(t *testing.T) {
		convRepo := new(MockConversationRepo)
		uc := NewConversationTakeoverUseCase(convRepo, nil, nil, nil, nil, nil)

		convRepo.On("GetByID", ctx, convID, "other-account").Return(nil, nil).Once()

		_, err := uc.Assume(ctx, convID, "other-account", userID)
		assert.ErrorIs(t, err, domain.ErrConversationNotFound)
		convRepo.AssertNotCalled(t, "AssumeConversation", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("idempotency: assuming twice keeps the conversation under the same user", func(t *testing.T) {
		convRepo := new(MockConversationRepo)
		broadcaster := newRecordingBroadcaster()
		uc := NewConversationTakeoverUseCase(convRepo, nil, nil, nil, nil, broadcaster)

		// Two consecutive Assume calls — second one happens on a
		// conversation whose is_bot_paused is already TRUE. Both should
		// succeed because the SQL update is idempotent and the use case
		// does not branch on prior state.
		convRepo.On("GetByID", ctx, convID, accountID).Return(baseConv, nil).Twice()
		convRepo.On("AssumeConversation", ctx, convID, accountID, userID).Return(nil).Twice()
		convRepo.On("GetAssignment", ctx, convID, accountID).Return(&domain.ConversationAssignment{
			ConversationID: convID,
			AssignedTo:     &userID,
			AssignedAt:     &now,
			IsBotPaused:    true,
		}, nil).Twice()

		_, err := uc.Assume(ctx, convID, accountID, userID)
		assert.NoError(t, err)
		_, err = uc.Assume(ctx, convID, accountID, userID)
		assert.NoError(t, err)
		convRepo.AssertNumberOfCalls(t, "AssumeConversation", 2)
	})
}

func TestReleaseConversation(t *testing.T) {
	ctx := context.Background()
	accountID := "acc-1"
	convID := "conv-1"

	baseConv := &domain.Conversation{
		ID:        convID,
		AccountID: accountID,
	}

	t.Run("happy path: clears assignment and resumes bot", func(t *testing.T) {
		convRepo := new(MockConversationRepo)
		broadcaster := newRecordingBroadcaster()
		uc := NewConversationTakeoverUseCase(convRepo, nil, nil, nil, nil, broadcaster)

		convRepo.On("GetByID", ctx, convID, accountID).Return(baseConv, nil).Once()
		convRepo.On("ReleaseConversation", ctx, convID, accountID).Return(nil).Once()
		convRepo.On("GetAssignment", ctx, convID, accountID).Return(&domain.ConversationAssignment{
			ConversationID: convID,
			IsBotPaused:    false,
		}, nil).Once()

		res, err := uc.Release(ctx, convID, accountID)
		assert.NoError(t, err)
		assert.False(t, res.Assignment.IsBotPaused)
		assert.Nil(t, res.Assignment.AssignedTo)
		assert.Equal(t, 1, broadcaster.Count(accountID))
		convRepo.AssertExpectations(t)
	})

	t.Run("ownership: returns NOT_FOUND for foreign conversation", func(t *testing.T) {
		convRepo := new(MockConversationRepo)
		uc := NewConversationTakeoverUseCase(convRepo, nil, nil, nil, nil, nil)

		convRepo.On("GetByID", ctx, convID, "other-account").Return(nil, nil).Once()

		_, err := uc.Release(ctx, convID, "other-account")
		assert.ErrorIs(t, err, domain.ErrConversationNotFound)
	})
}

func TestSendHumanMessage(t *testing.T) {
	ctx := context.Background()
	accountID := "acc-1"
	convID := "conv-1"
	userID := "user-1"

	recentCustomerMsg := time.Now().Add(-5 * time.Minute)
	takenOver := &domain.Conversation{
		ID:                    convID,
		AccountID:             accountID,
		ChannelID:             "chan-1",
		ContactID:             "+5511999999999",
		IsBotPaused:           true,
		LastCustomerMessageAt: &recentCustomerMsg,
	}

	t.Run("happy path: persists outbound message + dispatches via adapter + broadcasts", func(t *testing.T) {
		convRepo := new(MockConversationRepo)
		msgRepo := new(MockMessageRepo)
		channelRepo := new(MockChannelRepo)
		broadcaster := newRecordingBroadcaster()
		provider := &stubProvider{}

		providers := map[string]domain.MessageProvider{"whatsapp": provider}
		uc := NewConversationTakeoverUseCase(convRepo, msgRepo, channelRepo, nil, providers, broadcaster)

		var persisted *domain.Message
		convRepo.On("GetByID", ctx, convID, accountID).Return(takenOver, nil).Once()
		msgRepo.On("Create", ctx, mock.MatchedBy(func(m *domain.Message) bool {
			persisted = m
			// The mission spec asks for role="attendant". In this
			// schema that's encoded by Direction=outbound + From=userID
			// (as opposed to From="bot" for AI replies). Assert both.
			return m.Direction == "outbound" && m.From == userID && m.Type == "text"
		})).Return(nil).Once()
		// Mission 6: SendHumanMessage now stamps first_response_by=human
		// (idempotent). Maybe() keeps the pre-Mission-6 tests valid.
		convRepo.On("SetFirstResponse", ctx, convID, accountID, "human").Return(nil).Maybe()
		convRepo.On("ClearWaitingSince", ctx, convID, accountID).Return(nil).Maybe()
		channelRepo.On("GetByID", ctx, "chan-1").Return(&domain.Channel{
			ID:        "chan-1",
			AccountID: accountID,
			Type:      "whatsapp",
		}, nil).Once()

		msg, err := uc.SendHumanMessage(ctx, SendHumanMessageDTO{
			ConversationID: convID,
			AccountID:      accountID,
			UserID:         userID,
			UserName:       "Alice",
			Content:        "Oi, em que posso ajudar?",
		})

		assert.NoError(t, err)
		assert.NotNil(t, msg)
		assert.Equal(t, userID, msg.From)
		assert.Equal(t, "outbound", msg.Direction)
		assert.NotNil(t, persisted)
		assert.Len(t, provider.sent, 1, "provider must receive the message")
		assert.Equal(t, 1, broadcaster.Count(accountID))
	})

	// US-009 (review fixes): a provider send failure used to be logged and
	// swallowed — the handler answered 200 with the message body and the
	// attendant believed it was delivered. Now the row is stamped failed,
	// a message_send_failed event fires, and the call errors out.
	t.Run("send failure: marks row failed, broadcasts, returns MESSAGE_SEND_FAILED", func(t *testing.T) {
		convRepo := new(MockConversationRepo)
		msgRepo := new(MockMessageRepo)
		channelRepo := new(MockChannelRepo)
		broadcaster := newRecordingBroadcaster()
		provider := &stubProvider{err: assert.AnError}

		providers := map[string]domain.MessageProvider{"whatsapp": provider}
		uc := NewConversationTakeoverUseCase(convRepo, msgRepo, channelRepo, nil, providers, broadcaster)

		var persistedID string
		convRepo.On("GetByID", ctx, convID, accountID).Return(takenOver, nil).Once()
		msgRepo.On("Create", ctx, mock.MatchedBy(func(m *domain.Message) bool {
			persistedID = m.ID
			return true
		})).Return(nil).Once()
		msgRepo.On("MarkSendFailed", ctx, mock.AnythingOfType("string"), assert.AnError.Error()).Return(nil).Once()
		convRepo.On("SetFirstResponse", ctx, convID, accountID, "human").Return(nil).Maybe()
		convRepo.On("ClearWaitingSince", ctx, convID, accountID).Return(nil).Maybe()
		channelRepo.On("GetByID", ctx, "chan-1").Return(&domain.Channel{
			ID:        "chan-1",
			AccountID: accountID,
			Type:      "whatsapp",
		}, nil).Once()

		_, err := uc.SendHumanMessage(ctx, SendHumanMessageDTO{
			ConversationID: convID,
			AccountID:      accountID,
			UserID:         userID,
			UserName:       "Alice",
			Content:        "did this arrive?",
		})

		assert.ErrorIs(t, err, domain.ErrMessageSendFailed,
			"provider failure must surface to the attendant, not a silent success")
		msgRepo.AssertCalled(t, "MarkSendFailed", ctx, persistedID, assert.AnError.Error())
		// new_message (optimistic) + message_send_failed (correction)
		assert.Equal(t, 2, broadcaster.Count(accountID))
		last := broadcaster.Last(accountID)
		assert.Contains(t, string(last), `"message_send_failed"`)
		assert.Contains(t, string(last), persistedID)
	})

	t.Run("blocks when conversation is not under takeover", func(t *testing.T) {
		convRepo := new(MockConversationRepo)
		msgRepo := new(MockMessageRepo)
		uc := NewConversationTakeoverUseCase(convRepo, msgRepo, nil, nil, nil, nil)

		botRunning := *takenOver
		botRunning.IsBotPaused = false
		convRepo.On("GetByID", ctx, convID, accountID).Return(&botRunning, nil).Once()

		_, err := uc.SendHumanMessage(ctx, SendHumanMessageDTO{
			ConversationID: convID,
			AccountID:      accountID,
			UserID:         userID,
			Content:        "ignored",
		})
		assert.ErrorIs(t, err, domain.ErrConversationNotAssumed)
		msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	// US-010 (review fixes): the 24h window is a Meta restriction — it
	// applies to WhatsApp and Instagram only. The old gate ran before the
	// channel was even resolved, wrongly blocking >24h human replies on
	// SMS and Telegram (no upstream window exists there).
	t.Run("24h window gated by channel capability", func(t *testing.T) {
		stale := time.Now().Add(-25 * time.Hour)

		cases := []struct {
			channelType string
			wantBlocked bool
		}{
			{"whatsapp", true},
			{"instagram", true},
			{"sms", false},      // FAILED before US-010: blocked pre-channel-lookup
			{"telegram", false}, // FAILED before US-010: blocked pre-channel-lookup
		}
		for _, tc := range cases {
			t.Run(tc.channelType, func(t *testing.T) {
				convRepo := new(MockConversationRepo)
				msgRepo := new(MockMessageRepo)
				channelRepo := new(MockChannelRepo)
				provider := &stubProvider{}
				providers := map[string]domain.MessageProvider{tc.channelType: provider}
				uc := NewConversationTakeoverUseCase(convRepo, msgRepo, channelRepo, nil, providers, nil)

				conv := *takenOver
				conv.LastCustomerMessageAt = &stale
				convRepo.On("GetByID", ctx, convID, accountID).Return(&conv, nil).Once()
				channelRepo.On("GetByID", ctx, "chan-1").Return(&domain.Channel{
					ID:        "chan-1",
					AccountID: accountID,
					Type:      tc.channelType,
				}, nil).Once()

				if !tc.wantBlocked {
					msgRepo.On("Create", ctx, mock.AnythingOfType("*domain.Message")).Return(nil).Once()
					convRepo.On("SetFirstResponse", ctx, convID, accountID, "human").Return(nil).Maybe()
					convRepo.On("ClearWaitingSince", ctx, convID, accountID).Return(nil).Maybe()
				}

				_, err := uc.SendHumanMessage(ctx, SendHumanMessageDTO{
					ConversationID: convID,
					AccountID:      accountID,
					UserID:         userID,
					Content:        "reply >24h after last customer message",
				})

				if tc.wantBlocked {
					assert.ErrorIs(t, err, domain.ErrOutside24hWindow,
						"%s has a Meta 24h window — stale reply must be blocked", tc.channelType)
					msgRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
					assert.Empty(t, provider.sent)
				} else {
					assert.NoError(t, err,
						"%s has no 24h window — stale reply must go through", tc.channelType)
					assert.Len(t, provider.sent, 1, "reply must reach the %s adapter", tc.channelType)
				}
			})
		}
	})

	t.Run("ownership: foreign account gets NOT_FOUND", func(t *testing.T) {
		convRepo := new(MockConversationRepo)
		uc := NewConversationTakeoverUseCase(convRepo, nil, nil, nil, nil, nil)

		convRepo.On("GetByID", ctx, convID, "other-account").Return(nil, nil).Once()

		_, err := uc.SendHumanMessage(ctx, SendHumanMessageDTO{
			ConversationID: convID,
			AccountID:      "other-account",
			UserID:         userID,
			Content:        "ignored",
		})
		assert.ErrorIs(t, err, domain.ErrConversationNotFound)
	})
}
