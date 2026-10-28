package inapp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

// fakeResolver returns a canned conversation for any lookup.
type fakeResolver struct {
	conv *domain.Conversation
	err  error
}

func (f *fakeResolver) GetByID(ctx context.Context, id string, accountID string) (*domain.Conversation, error) {
	return f.conv, f.err
}

// fakeBroadcaster records BroadcastToAccount calls.
type fakeBroadcaster struct {
	accountIDs []string
	payloads   [][]byte
}

func (f *fakeBroadcaster) BroadcastToAccount(accountID string, payload []byte) {
	f.accountIDs = append(f.accountIDs, accountID)
	f.payloads = append(f.payloads, payload)
}

func TestSendMessage_NoNotifierIsNoOpSuccess(t *testing.T) {
	a := NewAdapter()
	err := a.SendMessage(context.Background(), &domain.Channel{Type: domain.ChannelTypeInternal}, "user1", &domain.Message{})
	assert.NoError(t, err)
}

func TestSendMessage_FiresNotifierHook(t *testing.T) {
	a := NewAdapter()
	var gotChannel *domain.Channel
	var gotTo string
	var gotMsg *domain.Message
	a.SetNotifier(func(ctx context.Context, channel *domain.Channel, to string, msg *domain.Message) {
		gotChannel, gotTo, gotMsg = channel, to, msg
	})

	ch := &domain.Channel{ID: "chI", Type: domain.ChannelTypeInternal}
	msg := &domain.Message{ID: "m1"}
	err := a.SendMessage(context.Background(), ch, "user1", msg)

	assert.NoError(t, err)
	assert.Same(t, ch, gotChannel)
	assert.Equal(t, "user1", gotTo)
	assert.Same(t, msg, gotMsg)
}

func TestSendMessage_MirrorsToCustomerAccount(t *testing.T) {
	a := NewAdapter()
	custAcc := "01CUSTOMERACCOUNT000000000"
	b := &fakeBroadcaster{}
	a.SetBroadcaster(b, &fakeResolver{conv: &domain.Conversation{
		ID:                "conv-1",
		AccountID:         "01PLATFORMACCOUNT000000000",
		ContactID:         "01CUSTOMERUSER000000000000",
		CustomerAccountID: &custAcc,
	}})

	convID := "conv-1"
	content := "resposta do vendedor"
	msg := &domain.Message{ID: "m1", ConversationID: &convID, Direction: "outbound", Content: &content}
	ch := &domain.Channel{ID: "chI", AccountID: "01PLATFORMACCOUNT000000000", Type: domain.ChannelTypeInternal}

	err := a.SendMessage(context.Background(), ch, "01CUSTOMERUSER000000000000", msg)

	assert.NoError(t, err)
	assert.Equal(t, []string{custAcc}, b.accountIDs, "mirror goes to the CUSTOMER account only")
	var payload map[string]interface{}
	assert.NoError(t, json.Unmarshal(b.payloads[0], &payload))
	assert.Equal(t, domain.SupportMessageEvent, payload["event"])
	assert.Equal(t, true, payload["internal_thread"])
	assert.Equal(t, "01CUSTOMERUSER000000000000", payload["user_id"])
	assert.Equal(t, "conv-1", payload["conversation_id"])
	assert.NotNil(t, payload["message"])
}

func TestSendMessage_NoMirrorWithoutCustomerAccount(t *testing.T) {
	a := NewAdapter()
	b := &fakeBroadcaster{}
	// Conversation without a customer_account_id stamp (not an internal
	// CS thread) → no customer-side broadcast.
	a.SetBroadcaster(b, &fakeResolver{conv: &domain.Conversation{ID: "conv-1"}})

	convID := "conv-1"
	msg := &domain.Message{ID: "m1", ConversationID: &convID}
	err := a.SendMessage(context.Background(), &domain.Channel{Type: domain.ChannelTypeInternal}, "u1", msg)

	assert.NoError(t, err)
	assert.Empty(t, b.accountIDs)
}

func TestSendMessage_MirrorDegradesOnLookupError(t *testing.T) {
	a := NewAdapter()
	b := &fakeBroadcaster{}
	a.SetBroadcaster(b, &fakeResolver{err: assert.AnError})

	convID := "conv-1"
	msg := &domain.Message{ID: "m1", ConversationID: &convID}
	err := a.SendMessage(context.Background(), &domain.Channel{Type: domain.ChannelTypeInternal}, "u1", msg)

	assert.NoError(t, err, "send must not fail when the mirror lookup errors")
	assert.Empty(t, b.accountIDs)
}

func TestHandleWebhook_Rejected(t *testing.T) {
	a := NewAdapter()
	err := a.HandleWebhook(context.Background(), &domain.Channel{Type: domain.ChannelTypeInternal}, []byte(`{}`))
	var appErr *domain.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, "CHANNEL_NO_WEBHOOK", appErr.Code)
}

// compile-time interface guarantee
var _ domain.MessageProvider = (*Adapter)(nil)
