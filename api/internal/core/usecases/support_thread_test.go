package usecases

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockCustomerAccountStamper is the narrow-port mock for the
// conversations.customer_account_id write-once stamp.
type mockCustomerAccountStamper struct{ mock.Mock }

func (m *mockCustomerAccountStamper) SetCustomerAccount(ctx context.Context, conversationID, customerAccountID string) error {
	args := m.Called(ctx, conversationID, customerAccountID)
	return args.Error(0)
}

const (
	stPlatformID = "01PLATFORMACCOUNT000000000"
	stCustomerID = "01CUSTOMERACCOUNT000000000"
	stUserID     = "01CUSTOMERUSER000000000000"
	stSellerID   = "01SELLERUSER00000000000000"
)

// capturedBroadcast is one recorded BroadcastToAccount call, with the
// payload decoded so tests assert on event fields directly.
type capturedBroadcast struct {
	accountID string
	payload   map[string]interface{}
}

// mockBroadcaster records every broadcast (thread-safe: the AI-reply
// broadcast may come from the dispatch goroutine in prod, and tests run
// it synchronously anyway).
type mockBroadcaster struct {
	mu     sync.Mutex
	events []capturedBroadcast
}

func (m *mockBroadcaster) BroadcastToAccount(accountID string, payload []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var p map[string]interface{}
	_ = json.Unmarshal(payload, &p)
	m.events = append(m.events, capturedBroadcast{accountID: accountID, payload: p})
}

func (m *mockBroadcaster) forAccount(accountID string) []capturedBroadcast {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []capturedBroadcast
	for _, e := range m.events {
		if e.accountID == accountID {
			out = append(out, e)
		}
	}
	return out
}

type supportThreadFixture struct {
	uc          *SupportThreadUseCase
	channelRepo *MockChannelRepo
	convRepo    *MockConversationRepo
	msgRepo     *MockMessageRepo
	accountRepo *MockAccountRepo
	userRepo    *MockUserRepo
	stamper     *mockCustomerAccountStamper
	inbound     *InboundWebhookUseCase
	dispatched  *bool
}

func newSupportThreadFixture(platformID string) *supportThreadFixture {
	f := &supportThreadFixture{
		channelRepo: new(MockChannelRepo),
		convRepo:    new(MockConversationRepo),
		msgRepo:     new(MockMessageRepo),
		accountRepo: new(MockAccountRepo),
		userRepo:    new(MockUserRepo),
		stamper:     new(mockCustomerAccountStamper),
		dispatched:  new(bool),
	}
	f.inbound = NewInboundWebhookUseCase(f.channelRepo, f.convRepo, f.msgRepo, nil, new(MockAgentRepo))
	f.uc = NewSupportThreadUseCase(platformID, f.channelRepo, f.convRepo, f.msgRepo, f.accountRepo, f.userRepo, f.inbound, f.stamper)
	// Capture the AI dispatch instead of running it — the ProcessAI
	// pipeline itself is covered by inbound_webhook_test.go.
	f.uc.dispatch = func(func()) { *f.dispatched = true }
	return f
}

func (f *supportThreadFixture) internalChannel() *domain.Channel {
	return &domain.Channel{
		ID:        "chan-internal",
		AccountID: stPlatformID,
		Type:      domain.ChannelTypeInternal,
		Name:      "Suporte Replider",
		IsActive:  true,
	}
}

func (f *supportThreadFixture) expectPlatformChannel() {
	f.channelRepo.On("ListByAccountID", mock.Anything, stPlatformID).
		Return([]*domain.Channel{f.internalChannel()}, nil)
}

func (f *supportThreadFixture) expectUser() {
	f.userRepo.On("GetByID", mock.Anything, stUserID).Return(&domain.User{
		ID:    stUserID,
		Name:  "Ana Cliente",
		Email: "ana@cliente.com",
	}, nil)
}

// allowInboundSideEffects stubs the best-effort calls SaveInbound makes
// on every message so each test only asserts what it cares about.
func (f *supportThreadFixture) allowInboundSideEffects() {
	f.convRepo.On("SetFirstInboundAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	f.convRepo.On("SetLanguage", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	f.convRepo.On("UpdateLastCustomerMessageAt", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
}

func TestSupportThread_SendMessage_NewThread_AssignsCarteiraSeller(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()
	f.expectUser()
	f.allowInboundSideEffects()

	// New thread: no open conversation for this user on the internal channel.
	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", stUserID).Return(nil, nil)
	f.convRepo.On("Create", mock.Anything, mock.MatchedBy(func(c *domain.Conversation) bool {
		return c.AccountID == stPlatformID && // conversation lives on the PLATFORM account
			c.ChannelID == "chan-internal" &&
			c.ContactID == stUserID && // thread keyed by the customer USER
			c.ContactName != nil && *c.ContactName == "Ana Cliente" &&
			c.ContactUsername != nil && *c.ContactUsername == "ana@cliente.com"
	})).Return(nil)
	f.msgRepo.On("Create", mock.Anything, mock.MatchedBy(func(m *domain.Message) bool {
		return m.Direction == "inbound" && m.From == stUserID &&
			m.Content != nil && *m.Content == "Preciso de ajuda com o plano"
	})).Return(true, nil)

	// Carteira: customer account has a linked seller → thread assigned to them.
	seller := stSellerID
	f.accountRepo.On("GetByID", mock.Anything, stCustomerID).Return(&domain.Account{
		ID:           stCustomerID,
		SellerUserID: &seller,
	}, nil)
	f.convRepo.On("AssignConversation", mock.Anything, mock.Anything, stPlatformID, mock.MatchedBy(func(u *string) bool {
		return u != nil && *u == stSellerID
	})).Return(nil)

	// Customer account stamped on the fresh thread.
	f.stamper.On("SetCustomerAccount", mock.Anything, mock.Anything, stCustomerID).Return(nil)

	result, err := f.uc.SendMessage(ctx, stCustomerID, stUserID, "Preciso de ajuda com o plano")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.ConversationID)
	assert.NotNil(t, result.Message)
	assert.True(t, *f.dispatched, "AI turn must be dispatched through the normal pipeline")
	f.convRepo.AssertExpectations(t)
	f.stamper.AssertExpectations(t)
}

func TestSupportThread_SendMessage_NoSeller_KeepsRodizioAssignment(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()
	f.expectUser()
	f.allowInboundSideEffects()

	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", stUserID).Return(nil, nil)
	f.convRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	f.msgRepo.On("Create", mock.Anything, mock.Anything).Return(true, nil)
	f.stamper.On("SetCustomerAccount", mock.Anything, mock.Anything, stCustomerID).Return(nil)

	// No carteira seller → the rodízio result from SaveInbound (when the
	// platform wires AutoAssign) must be left in place: no override call.
	f.accountRepo.On("GetByID", mock.Anything, stCustomerID).Return(&domain.Account{ID: stCustomerID}, nil)

	_, err := f.uc.SendMessage(ctx, stCustomerID, stUserID, "olá")

	assert.NoError(t, err)
	f.convRepo.AssertNotCalled(t, "AssignConversation", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestSupportThread_SendMessage_ExistingThread_NoStampNoReassign(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()
	f.expectUser()
	f.allowInboundSideEffects()

	existing := &domain.Conversation{
		ID:        "conv-existing",
		AccountID: stPlatformID,
		ChannelID: "chan-internal",
		ContactID: stUserID,
		Status:    domain.ConversationOpen,
	}
	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", stUserID).Return(existing, nil)
	f.msgRepo.On("Create", mock.Anything, mock.Anything).Return(true, nil)

	result, err := f.uc.SendMessage(ctx, stCustomerID, stUserID, "segunda mensagem")

	assert.NoError(t, err)
	assert.Equal(t, "conv-existing", result.ConversationID)
	f.stamper.AssertNotCalled(t, "SetCustomerAccount", mock.Anything, mock.Anything, mock.Anything)
	f.accountRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	f.convRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestSupportThread_SendMessage_UnavailableWhenPlatformUnset(t *testing.T) {
	f := newSupportThreadFixture("")
	_, err := f.uc.SendMessage(context.Background(), stCustomerID, stUserID, "oi")
	assert.ErrorIs(t, err, ErrSupportUnavailable)
}

func TestSupportThread_SendMessage_PlatformUsersHaveNoThread(t *testing.T) {
	f := newSupportThreadFixture(stPlatformID)
	_, err := f.uc.SendMessage(context.Background(), stPlatformID, stUserID, "oi")
	assert.ErrorIs(t, err, ErrSupportUnavailable)
}

func TestSupportThread_SendMessage_ValidatesContent(t *testing.T) {
	f := newSupportThreadFixture(stPlatformID)

	_, err := f.uc.SendMessage(context.Background(), stCustomerID, stUserID, "   ")
	assert.ErrorIs(t, err, ErrSupportEmptyMessage)

	long := make([]byte, supportMessageMaxLen+1)
	for i := range long {
		long[i] = 'a'
	}
	_, err = f.uc.SendMessage(context.Background(), stCustomerID, stUserID, string(long))
	assert.ErrorIs(t, err, ErrSupportMessageTooLong)
}

func TestSupportThread_SendMessage_BroadcastsBothSides(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()
	f.expectUser()
	f.allowInboundSideEffects()

	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", stUserID).Return(nil, nil)
	f.convRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	f.msgRepo.On("Create", mock.Anything, mock.Anything).Return(true, nil)
	f.stamper.On("SetCustomerAccount", mock.Anything, mock.Anything, stCustomerID).Return(nil)
	f.accountRepo.On("GetByID", mock.Anything, stCustomerID).Return(&domain.Account{ID: stCustomerID}, nil)

	b := &mockBroadcaster{}
	f.uc.SetBroadcaster(b)
	// Run the AI dispatch synchronously and fake the AI turn so the test
	// covers the reply broadcast without mocking the agent pipeline.
	f.uc.dispatch = func(fn func()) { fn() }
	aiContent := "Olá! Posso ajudar com o plano."
	f.uc.processAI = func(ctx context.Context, partial *InboundPartialResult, content string) *ProcessAIResult {
		return &ProcessAIResult{Message: &domain.Message{
			ID: "msg-ai", Direction: "outbound", Content: &aiContent,
		}}
	}

	_, err := f.uc.SendMessage(ctx, stCustomerID, stUserID, "Preciso de ajuda com o plano")
	assert.NoError(t, err)

	// Platform side: inbound + AI reply as standard new_message events.
	platform := b.forAccount(stPlatformID)
	assert.Len(t, platform, 2, "platform inbox gets inbound + AI reply")
	for _, e := range platform {
		assert.Equal(t, "new_message", e.payload["event"])
		assert.NotEmpty(t, e.payload["conversation_id"])
		assert.NotNil(t, e.payload["message"])
	}

	// Customer side: same two messages as support_message events carrying
	// the internal-thread marker + thread-owner user id.
	customer := b.forAccount(stCustomerID)
	assert.Len(t, customer, 2, "customer widget gets inbound echo + AI reply")
	for _, e := range customer {
		assert.Equal(t, domain.SupportMessageEvent, e.payload["event"])
		assert.Equal(t, true, e.payload["internal_thread"])
		assert.Equal(t, stUserID, e.payload["user_id"])
		assert.NotNil(t, e.payload["message"])
	}
}

func TestSupportThread_Broadcast_IsolatesUsersOfSameAccount(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()
	f.expectUser()
	f.allowInboundSideEffects()

	otherUser := "01OTHERUSER000000000000000"
	f.userRepo.On("GetByID", mock.Anything, otherUser).Return(&domain.User{
		ID: otherUser, Name: "Beto Cliente", Email: "beto@cliente.com",
	}, nil)

	// Each user gets their own fresh thread on the same customer account.
	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", stUserID).Return(nil, nil)
	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", otherUser).Return(nil, nil)
	f.convRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	f.msgRepo.On("Create", mock.Anything, mock.Anything).Return(true, nil)
	f.stamper.On("SetCustomerAccount", mock.Anything, mock.Anything, stCustomerID).Return(nil)
	f.accountRepo.On("GetByID", mock.Anything, stCustomerID).Return(&domain.Account{ID: stCustomerID}, nil)

	b := &mockBroadcaster{}
	f.uc.SetBroadcaster(b)

	res1, err := f.uc.SendMessage(ctx, stCustomerID, stUserID, "mensagem da Ana")
	assert.NoError(t, err)
	res2, err := f.uc.SendMessage(ctx, stCustomerID, otherUser, "mensagem do Beto")
	assert.NoError(t, err)
	assert.NotEqual(t, res1.ConversationID, res2.ConversationID, "each user has their own thread")

	// Both customer-side events land on the same account_id — isolation is
	// the user_id tag the widget filters on: each event carries ONLY its
	// own thread owner, never the colleague's.
	customer := b.forAccount(stCustomerID)
	assert.Len(t, customer, 2)
	assert.Equal(t, stUserID, customer[0].payload["user_id"])
	assert.Equal(t, res1.ConversationID, customer[0].payload["conversation_id"])
	assert.Equal(t, otherUser, customer[1].payload["user_id"])
	assert.Equal(t, res2.ConversationID, customer[1].payload["conversation_id"])
}

func TestSupportThread_ListMessages_ReturnsOnlyOwnThread(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()

	ownConv := &domain.Conversation{
		ID:        "conv-own",
		AccountID: stPlatformID,
		ChannelID: "chan-internal",
		ContactID: stUserID,
	}
	content := "olá"
	msgs := []*domain.Message{{ID: "msg-1", Direction: "inbound", Content: &content}}
	// The thread lookup is keyed by the CALLER's user id — there is no
	// parameter through which another user's thread can be addressed.
	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", stUserID).Return(ownConv, nil)
	f.msgRepo.On("ListByConversationID", mock.Anything, "conv-own", int32(50), int32(0)).Return(msgs, nil)

	page, err := f.uc.ListMessages(ctx, stCustomerID, stUserID, 50, 0)

	assert.NoError(t, err)
	assert.Equal(t, "conv-own", page.ConversationID)
	assert.Len(t, page.Messages, 1)

	// A second user of the SAME account gets their own (empty) thread —
	// never their colleague's messages.
	otherUser := "01OTHERUSER000000000000000"
	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", otherUser).Return(nil, nil)

	otherPage, err := f.uc.ListMessages(ctx, stCustomerID, otherUser, 50, 0)

	assert.NoError(t, err)
	assert.Empty(t, otherPage.ConversationID)
	assert.Empty(t, otherPage.Messages)
	f.msgRepo.AssertNumberOfCalls(t, "ListByConversationID", 1)
}

// ──────────────────────────── US-004: read tracking + AI-reply push ────────────────────────────

// fakeReadStamper records MarkThreadRead calls (thread-safe for parity with
// the dispatch goroutine even though tests run it synchronously).
type fakeReadStamper struct {
	mu    sync.Mutex
	convs []string
}

func (f *fakeReadStamper) MarkThreadRead(ctx context.Context, conversationID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.convs = append(f.convs, conversationID)
	return nil
}

func (f *fakeReadStamper) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.convs))
	copy(out, f.convs)
	return out
}

// fakeSupportPusher records NotifySupportMessage calls.
type fakeSupportPusher struct {
	mu    sync.Mutex
	calls []struct {
		userID, convID string
		msg            *domain.Message
	}
}

func (f *fakeSupportPusher) NotifySupportMessage(userID, conversationID string, msg *domain.Message) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, struct {
		userID, convID string
		msg            *domain.Message
	}{userID, conversationID, msg})
}

func TestSupportThread_ListMessages_StampsThreadRead(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()

	ownConv := &domain.Conversation{
		ID: "conv-own", AccountID: stPlatformID,
		ChannelID: "chan-internal", ContactID: stUserID,
	}
	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", stUserID).Return(ownConv, nil)
	f.msgRepo.On("ListByConversationID", mock.Anything, "conv-own", int32(50), int32(0)).Return([]*domain.Message{}, nil)

	stamper := &fakeReadStamper{}
	f.uc.SetReadStamper(stamper)

	_, err := f.uc.ListMessages(ctx, stCustomerID, stUserID, 50, 0)
	assert.NoError(t, err)
	assert.Equal(t, []string{"conv-own"}, stamper.calls(), "opening the thread marks it read")

	// A user with no thread yet stamps nothing.
	otherUser := "01OTHERUSER000000000000000"
	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", otherUser).Return(nil, nil)
	_, err = f.uc.ListMessages(ctx, stCustomerID, otherUser, 50, 0)
	assert.NoError(t, err)
	assert.Len(t, stamper.calls(), 1, "no thread → no stamp")
}

func TestSupportThread_SendMessage_StampsThreadRead(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()
	f.expectUser()
	f.allowInboundSideEffects()

	existing := &domain.Conversation{
		ID: "conv-existing", AccountID: stPlatformID,
		ChannelID: "chan-internal", ContactID: stUserID,
		Status: domain.ConversationOpen,
	}
	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", stUserID).Return(existing, nil)
	f.msgRepo.On("Create", mock.Anything, mock.Anything).Return(true, nil)

	stamper := &fakeReadStamper{}
	f.uc.SetReadStamper(stamper)

	_, err := f.uc.SendMessage(ctx, stCustomerID, stUserID, "escrevendo = lendo")
	assert.NoError(t, err)
	assert.Equal(t, []string{"conv-existing"}, stamper.calls(),
		"writing to the thread implies having read it")
}

func TestSupportThread_AIReply_PushesThreadOwnerOnly(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()
	f.expectUser()
	f.allowInboundSideEffects()

	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", stUserID).Return(nil, nil)
	f.convRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	f.msgRepo.On("Create", mock.Anything, mock.Anything).Return(true, nil)
	f.stamper.On("SetCustomerAccount", mock.Anything, mock.Anything, stCustomerID).Return(nil)
	f.accountRepo.On("GetByID", mock.Anything, stCustomerID).Return(&domain.Account{ID: stCustomerID}, nil)

	pusher := &fakeSupportPusher{}
	f.uc.SetSupportPusher(pusher)
	f.uc.dispatch = func(fn func()) { fn() }
	aiContent := "resposta do agente"
	f.uc.processAI = func(ctx context.Context, partial *InboundPartialResult, content string) *ProcessAIResult {
		return &ProcessAIResult{Message: &domain.Message{
			ID: "msg-ai", Direction: "outbound", Content: &aiContent,
		}}
	}

	res, err := f.uc.SendMessage(ctx, stCustomerID, stUserID, "oi")
	assert.NoError(t, err)

	// Exactly ONE push: the AI reply, aimed at the thread owner. The
	// customer's own inbound must never push back at them.
	assert.Len(t, pusher.calls, 1)
	assert.Equal(t, stUserID, pusher.calls[0].userID)
	assert.Equal(t, res.ConversationID, pusher.calls[0].convID)
	assert.Equal(t, "msg-ai", pusher.calls[0].msg.ID)
}

func TestSupportThread_AINoReply_NoPush(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()
	f.expectUser()
	f.allowInboundSideEffects()

	existing := &domain.Conversation{
		ID: "conv-existing", AccountID: stPlatformID,
		ChannelID: "chan-internal", ContactID: stUserID,
		Status: domain.ConversationOpen,
	}
	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", stUserID).Return(existing, nil)
	f.msgRepo.On("Create", mock.Anything, mock.Anything).Return(true, nil)

	pusher := &fakeSupportPusher{}
	f.uc.SetSupportPusher(pusher)
	f.uc.dispatch = func(fn func()) { fn() }
	f.uc.processAI = func(ctx context.Context, partial *InboundPartialResult, content string) *ProcessAIResult {
		return nil // bot paused / no agent on channel
	}

	_, err := f.uc.SendMessage(ctx, stCustomerID, stUserID, "oi")
	assert.NoError(t, err)
	assert.Empty(t, pusher.calls, "no AI reply → nothing to push")
}

// ──────────────────────────────── OpenThread (US-008) ────────────────────────────────

func (f *supportThreadFixture) expectCarteiraAccount(sellerID string) {
	var seller *string
	if sellerID != "" {
		seller = &sellerID
	}
	f.accountRepo.On("GetByID", mock.Anything, stCustomerID).Return(&domain.Account{
		ID:           stCustomerID,
		SellerUserID: seller,
	}, nil)
}

func (f *supportThreadFixture) expectAccountAdmin() {
	f.userRepo.On("GetAdminByAccountID", mock.Anything, stCustomerID).Return(&domain.User{
		ID:    stUserID,
		Name:  "Ana Cliente",
		Email: "ana@cliente.com",
	}, nil)
}

func TestSupportThread_OpenThread_CreatesPausedThreadAssignedToSeller(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()
	f.expectCarteiraAccount(stSellerID)
	f.expectAccountAdmin()

	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", stUserID).Return(nil, nil)
	f.convRepo.On("Create", mock.Anything, mock.MatchedBy(func(c *domain.Conversation) bool {
		return c.AccountID == stPlatformID &&
			c.ChannelID == "chan-internal" &&
			c.ContactID == stUserID && // thread keyed by the target admin user
			c.ContactName != nil && *c.ContactName == "Ana Cliente" &&
			c.ContactUsername != nil && *c.ContactUsername == "ana@cliente.com" &&
			c.IsBotPaused // seller-initiated → takeover mode from birth
	})).Return(nil)
	f.stamper.On("SetCustomerAccount", mock.Anything, mock.Anything, stCustomerID).Return(nil)
	f.convRepo.On("AssignConversation", mock.Anything, mock.Anything, stPlatformID, mock.MatchedBy(func(u *string) bool {
		return u != nil && *u == stSellerID
	})).Return(nil)

	result, err := f.uc.OpenThread(ctx, stSellerID, stCustomerID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Created)
	assert.True(t, result.IsBotPaused)
	assert.Equal(t, stUserID, result.UserID)
	assert.Equal(t, "Ana Cliente", result.UserName)
	assert.NotEmpty(t, result.ConversationID)
	assert.False(t, *f.dispatched, "no AI turn on an outbound-first open — nothing was said yet")
	f.convRepo.AssertExpectations(t)
	f.stamper.AssertExpectations(t)
}

func TestSupportThread_OpenThread_ExistingThreadReusedUntouched(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()
	f.expectCarteiraAccount(stSellerID)
	f.expectAccountAdmin()

	existing := &domain.Conversation{
		ID:          "conv-existing",
		AccountID:   stPlatformID,
		ChannelID:   "chan-internal",
		ContactID:   stUserID,
		IsBotPaused: false, // normal pipeline state is preserved
	}
	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", stUserID).Return(existing, nil)

	result, err := f.uc.OpenThread(ctx, stSellerID, stCustomerID)

	assert.NoError(t, err)
	assert.False(t, result.Created)
	assert.False(t, result.IsBotPaused)
	assert.Equal(t, "conv-existing", result.ConversationID)
	f.convRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	f.convRepo.AssertNotCalled(t, "AssignConversation", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	f.stamper.AssertNotCalled(t, "SetCustomerAccount", mock.Anything, mock.Anything, mock.Anything)
}

func TestSupportThread_OpenThread_NotCarteiraAccount_NotFound(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()

	// Account managed by ANOTHER seller: the caller must get the same
	// answer as a nonexistent account (no-leak convention).
	other := "01OTHERSELLER0000000000000"
	f.accountRepo.On("GetByID", mock.Anything, stCustomerID).Return(&domain.Account{
		ID:           stCustomerID,
		SellerUserID: &other,
	}, nil)

	_, err := f.uc.OpenThread(ctx, stSellerID, stCustomerID)

	assert.ErrorIs(t, err, domain.ErrAccountNotFound)
	f.convRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestSupportThread_OpenThread_NoSellerLinked_NotFound(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()
	f.expectCarteiraAccount("") // no carteira link at all

	_, err := f.uc.OpenThread(ctx, stSellerID, stCustomerID)

	assert.ErrorIs(t, err, domain.ErrAccountNotFound)
}

func TestSupportThread_OpenThread_NoAdmin_Errors(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()
	f.expectCarteiraAccount(stSellerID)
	f.userRepo.On("GetAdminByAccountID", mock.Anything, stCustomerID).Return(nil, nil)

	_, err := f.uc.OpenThread(ctx, stSellerID, stCustomerID)

	assert.ErrorIs(t, err, ErrSupportNoAdmin)
}

func TestSupportThread_OpenThread_CreateRace_ReusesWinner(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()
	f.expectCarteiraAccount(stSellerID)
	f.expectAccountAdmin()

	// First lookup misses; the INSERT loses to a parallel open (e.g. the
	// customer's first message landing at the same instant); the re-read
	// returns the winning row, reused untouched.
	winner := &domain.Conversation{
		ID:          "conv-winner",
		AccountID:   stPlatformID,
		ChannelID:   "chan-internal",
		ContactID:   stUserID,
		IsBotPaused: false,
	}
	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", stUserID).Return(nil, nil).Once()
	f.convRepo.On("Create", mock.Anything, mock.Anything).Return(domain.ErrConversationAlreadyOpen)
	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", stUserID).Return(winner, nil).Once()

	result, err := f.uc.OpenThread(ctx, stSellerID, stCustomerID)

	assert.NoError(t, err)
	assert.False(t, result.Created)
	assert.Equal(t, "conv-winner", result.ConversationID)
	f.convRepo.AssertNotCalled(t, "AssignConversation", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	f.stamper.AssertNotCalled(t, "SetCustomerAccount", mock.Anything, mock.Anything, mock.Anything)
}

func TestSupportThread_OpenThread_PlatformUnset_Unavailable(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture("")

	_, err := f.uc.OpenThread(ctx, stSellerID, stCustomerID)

	assert.ErrorIs(t, err, ErrSupportUnavailable)
}

// ──────────────────────────────── US-010: metering isolation ────────────────────────────────

// The internal thread lives on the PLATFORM account, so every meter the
// pipeline bumps — conversations_count at create time (SaveInbound) and
// AI tokens after the reply (ProcessAI charges partial.AccountID) —
// lands on the platform's usage row. The CUSTOMER account is never
// touched: no franquia burn, no plan-limit pressure, no billing.
func TestSupportThread_MeteringChargesPlatformAccountOnly(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()
	f.expectUser()
	f.allowInboundSideEffects()

	usageRepo := new(MockUsageRepo)
	f.inbound.SetUsageRepo(usageRepo)
	// The one and only bump: +1 conversation on the PLATFORM account.
	usageRepo.On("AddUsage", mock.Anything, stPlatformID,
		int64(0), int64(0), int64(0), int32(1)).Return(nil).Once()

	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", stUserID).Return(nil, nil)
	f.convRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	f.msgRepo.On("Create", mock.Anything, mock.Anything).Return(true, nil)
	f.accountRepo.On("GetByID", mock.Anything, stCustomerID).
		Return(&domain.Account{ID: stCustomerID}, nil) // no seller linked
	f.stamper.On("SetCustomerAccount", mock.Anything, mock.Anything, stCustomerID).Return(nil)

	// Run the AI dispatch synchronously and capture the partial the
	// pipeline hands to ProcessAI — its AccountID is the account the
	// token accounting (ProcessMessage.AddUsage) will charge.
	var aiChargedAccount string
	f.uc.dispatch = func(fn func()) { fn() }
	f.uc.processAI = func(ctx context.Context, partial *InboundPartialResult, content string) *ProcessAIResult {
		aiChargedAccount = partial.AccountID
		return nil
	}

	_, err := f.uc.SendMessage(ctx, stCustomerID, stUserID, "Quanto custa o plano Pro?")
	assert.NoError(t, err)

	// Conversation meter: exactly one bump, on the platform account.
	usageRepo.AssertExpectations(t)
	usageRepo.AssertNotCalled(t, "AddUsage", mock.Anything, stCustomerID,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	// Token meter: the AI turn is charged to the platform account too.
	assert.Equal(t, stPlatformID, aiChargedAccount)
}

// A follow-up message on an existing thread bumps nothing at all — the
// conversation counter only moves at create time.
func TestSupportThread_Metering_NoBumpOnExistingThread(t *testing.T) {
	ctx := context.Background()
	f := newSupportThreadFixture(stPlatformID)
	f.expectPlatformChannel()
	f.expectUser()
	f.allowInboundSideEffects()

	usageRepo := new(MockUsageRepo)
	f.inbound.SetUsageRepo(usageRepo)

	f.convRepo.On("FindByContact", mock.Anything, "chan-internal", stUserID).Return(&domain.Conversation{
		ID:        "conv-existing",
		AccountID: stPlatformID,
		ChannelID: "chan-internal",
		ContactID: stUserID,
	}, nil)
	f.msgRepo.On("Create", mock.Anything, mock.Anything).Return(true, nil)

	_, err := f.uc.SendMessage(ctx, stCustomerID, stUserID, "E o plano Core?")
	assert.NoError(t, err)

	usageRepo.AssertNotCalled(t, "AddUsage",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
