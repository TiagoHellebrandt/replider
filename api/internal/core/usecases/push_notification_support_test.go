package usecases

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePushSubRepo is a hand-rolled PushSubscriptionRepository: canned rows
// per user/account, no behavior beyond what the fan-out paths touch.
type fakePushSubRepo struct {
	byUser    map[string][]*domain.PushSubscription
	byAccount map[string][]*domain.PushSubscription
	disabled  []string
}

func (f *fakePushSubRepo) Upsert(ctx context.Context, sub *domain.PushSubscription) error {
	return nil
}
func (f *fakePushSubRepo) ListByAccountID(ctx context.Context, accountID string) ([]*domain.PushSubscription, error) {
	return f.byAccount[accountID], nil
}
func (f *fakePushSubRepo) ListByUserID(ctx context.Context, userID string) ([]*domain.PushSubscription, error) {
	return f.byUser[userID], nil
}
func (f *fakePushSubRepo) DeleteByEndpoint(ctx context.Context, userID, endpoint string) error {
	return nil
}
func (f *fakePushSubRepo) DisableByEndpoint(ctx context.Context, endpoint string) error {
	f.disabled = append(f.disabled, endpoint)
	return nil
}
func (f *fakePushSubRepo) Touch(ctx context.Context, id string) error { return nil }

// capturedSend is one recorded sender.Send call.
type capturedSend struct {
	sub     *domain.PushSubscription
	payload domain.PushPayload
}

// fakePushSender records sends thread-safely (NotifySupportMessage fans out
// from a detached goroutine) and signals `done` per send.
type fakePushSender struct {
	mu    sync.Mutex
	sends []capturedSend
	done  chan struct{}
}

func newFakePushSender(buf int) *fakePushSender {
	return &fakePushSender{done: make(chan struct{}, buf)}
}

func (f *fakePushSender) Send(ctx context.Context, sub *domain.PushSubscription, payload domain.PushPayload) error {
	f.mu.Lock()
	f.sends = append(f.sends, capturedSend{sub: sub, payload: payload})
	f.mu.Unlock()
	if f.done != nil {
		f.done <- struct{}{}
	}
	return nil
}

func (f *fakePushSender) VAPIDPublicKey() string { return "test-key" }

func (f *fakePushSender) captured() []capturedSend {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]capturedSend, len(f.sends))
	copy(out, f.sends)
	return out
}

func pushSub(id, userID, accountID string) *domain.PushSubscription {
	return &domain.PushSubscription{
		ID: id, UserID: userID, AccountID: accountID,
		Platform: "web", Endpoint: "https://push.example/" + id, Enabled: true,
	}
}

func TestPushNotifyUser_SendsOnlyToThatUsersDevices(t *testing.T) {
	owner := "01THREADOWNER0000000000000"
	repo := &fakePushSubRepo{byUser: map[string][]*domain.PushSubscription{
		owner:                        {pushSub("s1", owner, "acc-cust"), pushSub("s2", owner, "acc-cust")},
		"01SOMEONEELSE0000000000000": {pushSub("s3", "01SOMEONEELSE0000000000000", "acc-cust")},
	}}
	sender := newFakePushSender(4)
	uc := NewPushNotificationUseCase(repo, sender)

	uc.NotifyUser(context.Background(), owner, domain.PushPayload{Event: "support_message"})

	sends := sender.captured()
	require.Len(t, sends, 2, "only the thread owner's devices are poked")
	for _, s := range sends {
		assert.Equal(t, owner, s.sub.UserID)
		// AccountID stamped from the subscription row so the SW can route.
		assert.Equal(t, "acc-cust", s.payload.AccountID)
	}
}

func TestPushNotifySupportMessage_ComposesSupportPayload(t *testing.T) {
	owner := "01THREADOWNER0000000000000"
	repo := &fakePushSubRepo{byUser: map[string][]*domain.PushSubscription{
		owner: {pushSub("s1", owner, "acc-cust")},
	}}
	sender := newFakePushSender(1)
	uc := NewPushNotificationUseCase(repo, sender)

	content := "Oi! Seu gerente respondeu."
	convID := "conv-internal-1"
	uc.NotifySupportMessage(owner, convID, &domain.Message{
		ID: "msg-1", Type: "text", Direction: "outbound", Content: &content,
	})

	select {
	case <-sender.done:
	case <-time.After(2 * time.Second):
		t.Fatal("support push never dispatched")
	}

	sends := sender.captured()
	require.Len(t, sends, 1)
	p := sends[0].payload
	assert.Equal(t, "support_message", p.Event)
	assert.Equal(t, content, p.Body)
	assert.Equal(t, convID, p.ConversationID)
	assert.Equal(t, "support:"+convID, p.Tag)
	assert.Equal(t, "/support", p.ClickURL, "deep link opens the support widget")
	assert.Empty(t, p.Title, "title stays empty so the SW renders locale-aware copy")
}

func TestPushNotifySupportMessage_NilSafe(t *testing.T) {
	repo := &fakePushSubRepo{}
	sender := newFakePushSender(1)
	uc := NewPushNotificationUseCase(repo, sender)

	// nil message / empty user must not panic nor send.
	uc.NotifySupportMessage("", "conv", &domain.Message{})
	uc.NotifySupportMessage("user", "conv", nil)

	select {
	case <-sender.done:
		t.Fatal("no push should fire for nil/empty input")
	case <-time.After(50 * time.Millisecond):
	}
}
