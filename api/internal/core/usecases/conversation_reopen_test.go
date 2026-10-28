package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// reopeningConvRepo is MockConversationRepo plus the ConversationReopener
// port the postgres repository satisfies. The plain mock lacks it on
// purpose — that is what keeps the ~70 existing SaveInbound expectations
// (FindByContact nil → Create) valid.
type reopeningConvRepo struct {
	*MockConversationRepo
	latest    *domain.Conversation
	latestErr error
	reopenOK  bool
	reopenErr error
	reopened  []string
}

func (r *reopeningConvRepo) FindLatestByContact(_ context.Context, _, _ string) (*domain.Conversation, error) {
	return r.latest, r.latestErr
}

func (r *reopeningConvRepo) Reopen(_ context.Context, conversationID, _ string) (bool, error) {
	r.reopened = append(r.reopened, conversationID)
	return r.reopenOK, r.reopenErr
}

func newReopenFixture(latest *domain.Conversation) (*InboundWebhookUseCase, *reopeningConvRepo, *MockMessageRepo) {
	convRepo := &reopeningConvRepo{MockConversationRepo: new(MockConversationRepo), latest: latest, reopenOK: true}
	convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetFirstResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	msgRepo := new(MockMessageRepo)
	uc := NewInboundWebhookUseCase(new(MockChannelRepo), convRepo, msgRepo, nil, new(MockAgentRepo))
	return uc, convRepo, msgRepo
}

var reopenChannel = &domain.Channel{
	ID: "chan-001", AccountID: "acc-001", Type: "email", Name: "Comercial", IsActive: true,
}

var reopenDTO = InboundMessageDTO{
	SenderPhone: "compras@hospital.com.br", SenderName: "Compras",
	Content: "Voltei a pensar na proposta", MessageType: "text",
}

// The customer answers a thread the attendant resolved: the reply lands on
// THAT thread, reopened, not on a fresh conversation without the history.
func TestSaveInbound_ReopensResolvedThreadInsteadOfCreatingANewOne(t *testing.T) {
	ctx := context.Background()
	resolved := time.Now().Add(-time.Hour)
	uc, convRepo, msgRepo := newReopenFixture(&domain.Conversation{
		ID: "conv-closed", AccountID: "acc-001", ChannelID: "chan-001",
		ContactID: "compras@hospital.com.br", Status: domain.ConversationClosed, ResolvedAt: &resolved,
	})
	convRepo.On("FindByContact", ctx, "chan-001", "compras@hospital.com.br").Return(nil, nil)
	convRepo.On("UpdateLastCustomerMessageAt", ctx, "conv-closed", "acc-001").Return(nil)
	msgRepo.On("Create", ctx, mock.MatchedBy(func(m *domain.Message) bool {
		return m.ConversationID != nil && *m.ConversationID == "conv-closed" && m.Direction == "inbound"
	})).Return(nil)

	result, err := uc.SaveInbound(ctx, reopenChannel, reopenDTO)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "conv-closed", result.ConversationID)
	assert.Equal(t, []string{"conv-closed"}, convRepo.reopened)
	convRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	convRepo.AssertExpectations(t)
	msgRepo.AssertExpectations(t)
}

// Nothing closed for this sender → the ordinary create path, untouched.
func TestSaveInbound_NoClosedThreadStillCreates(t *testing.T) {
	ctx := context.Background()
	uc, convRepo, msgRepo := newReopenFixture(nil)
	convRepo.On("FindByContact", ctx, "chan-001", "compras@hospital.com.br").Return(nil, nil)
	convRepo.On("Create", ctx, mock.AnythingOfType("*domain.Conversation")).Return(nil)
	msgRepo.On("Create", ctx, mock.AnythingOfType("*domain.Message")).Return(nil)

	_, err := uc.SaveInbound(ctx, reopenChannel, reopenDTO)

	require.NoError(t, err)
	assert.Empty(t, convRepo.reopened)
	convRepo.AssertExpectations(t)
}

func TestReopenIfClosed(t *testing.T) {
	ctx := context.Background()

	t.Run("repository without the port keeps the old behaviour", func(t *testing.T) {
		uc := NewInboundWebhookUseCase(new(MockChannelRepo), new(MockConversationRepo), new(MockMessageRepo), nil, new(MockAgentRepo))
		conv, err := uc.reopenIfClosed(ctx, "chan-001", "x")
		require.NoError(t, err)
		assert.Nil(t, conv)
	})

	t.Run("latest thread not closed → nothing to reopen", func(t *testing.T) {
		uc, convRepo, _ := newReopenFixture(&domain.Conversation{ID: "c", Status: domain.ConversationPendingHuman})
		conv, err := uc.reopenIfClosed(ctx, "chan-001", "x")
		require.NoError(t, err)
		assert.Nil(t, conv)
		assert.Empty(t, convRepo.reopened)
	})

	t.Run("reopened thread comes back open with waiting_since restarted", func(t *testing.T) {
		resolved := time.Now()
		uc, _, _ := newReopenFixture(&domain.Conversation{
			ID: "c", AccountID: "acc-001", Status: domain.ConversationClosed, ResolvedAt: &resolved,
		})
		conv, err := uc.reopenIfClosed(ctx, "chan-001", "x")
		require.NoError(t, err)
		require.NotNil(t, conv)
		assert.Equal(t, domain.ConversationOpen, conv.Status)
		assert.Nil(t, conv.ResolvedAt)
		assert.NotNil(t, conv.WaitingSince)
	})

	t.Run("a parallel inbound opened a thread meanwhile → defer to the caller's race path", func(t *testing.T) {
		uc, convRepo, _ := newReopenFixture(&domain.Conversation{ID: "c", Status: domain.ConversationClosed})
		convRepo.reopenErr = domain.ErrConversationAlreadyOpen
		conv, err := uc.reopenIfClosed(ctx, "chan-001", "x")
		require.NoError(t, err)
		assert.Nil(t, conv)
	})

	t.Run("lookup failure is an error, not a silent new thread", func(t *testing.T) {
		uc, convRepo, _ := newReopenFixture(nil)
		convRepo.latestErr = errors.New("db down")
		_, err := uc.reopenIfClosed(ctx, "chan-001", "x")
		require.Error(t, err)
	})
}
