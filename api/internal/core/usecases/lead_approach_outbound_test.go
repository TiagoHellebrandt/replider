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

// claimingLeadRepo is a lead repo that ALSO implements the atomic claim, the
// way the postgres repository does. `claimed` is what ClaimForSend answers.
type claimingLeadRepo struct {
	*mockLeadRepo
	claimed bool
	claims  int
}

func (c *claimingLeadRepo) ClaimForSend(_ context.Context, _ string) (bool, error) {
	c.claims++
	return c.claimed, nil
}

// A funnel born without a channel (every default funnel in production on
// 2026-09-01) must still send — through the account's active email channel.
func TestApproveSend_FallsBackToTheAccountEmailChannel(t *testing.T) {
	warmed := time.Now().Add(-60 * 24 * time.Hour)
	uc, _, sender := emailApprovalFixture(t, mailboxAt(true, &warmed, 25, 0))

	funnelRepo := new(MockSalesFunnelRepo)
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", IsDefault: true}, nil)
	uc.funnelRepo = funnelRepo

	channelRepo := uc.channelRepo.(*MockChannelRepo)
	older := time.Now().Add(-48 * time.Hour)
	channelRepo.On("ListByAccountID", mock.Anything, "acc1").Return([]*domain.Channel{
		{ID: "ch-wa", AccountID: "acc1", Type: "whatsapp", IsActive: true, CreatedAt: older.Add(-time.Hour)},
		{ID: "ch-email", AccountID: "acc1", Type: "email", IsActive: true, CreatedAt: older,
			Config: map[string]any{"from_address": "comercial@clinica.com.br"}},
		{ID: "ch-email-2", AccountID: "acc1", Type: "email", IsActive: true, CreatedAt: time.Now()},
	}, nil)

	var sentThrough *domain.Channel
	sender.On("SendMessageAndGetID", mock.Anything, mock.Anything, "compras@hospital.com.br", mock.Anything).
		Run(func(args mock.Arguments) { sentThrough = args.Get(1).(*domain.Channel) }).
		Return("<mid@clinica.com.br>", nil)

	lead, err := uc.ApproveSend(context.Background(), "acc1", "lead-email",
		ApproachSendInput{Text: "Olá, tudo bem?", Subject: "Proposta"})

	require.NoError(t, err)
	assert.Equal(t, domain.LeadStatusContacted, lead.Status)
	require.NotNil(t, sentThrough)
	// The OLDEST active email channel, not the newest: deterministic, and
	// a reconnect never silently moves the account to another mailbox.
	assert.Equal(t, "ch-email", sentThrough.ID)
}

// No email channel at all is still a refusal — with the code the app maps to
// "connect an email in Settings".
func TestApproveSend_NoEmailChannelAnywhereIsStillRefused(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, _, _, _, _, _, _, _, sender := newApproachUC()
	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "x@y.com", Status: domain.LeadStatusPendingApproval}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1"}, nil)
	channelRepo.On("ListByAccountID", mock.Anything, "acc1").Return([]*domain.Channel{
		{ID: "ch-old", AccountID: "acc1", Type: "email", IsActive: false},
	}, nil)

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead1", ApproachSendInput{Text: "Olá"})
	assert.ErrorIs(t, err, domain.ErrLeadFunnelNoChannel)
	sender.AssertNotCalled(t, "SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Two approvers race: the one who loses the claim gets "not pending" and
// NOTHING is sent twice.
func TestApproveSend_LosingTheClaimSendsNothing(t *testing.T) {
	warmed := time.Now().Add(-60 * 24 * time.Hour)
	uc, _, sender := emailApprovalFixture(t, mailboxAt(true, &warmed, 25, 0))
	repo := &claimingLeadRepo{mockLeadRepo: uc.leadRepo.(*mockLeadRepo), claimed: false}
	uc.leadRepo = repo

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead-email",
		ApproachSendInput{Text: "Olá, tudo bem?", Subject: "Proposta"})

	assert.ErrorIs(t, err, domain.ErrLeadNotPendingApproval)
	assert.Equal(t, 1, repo.claims)
	assert.Equal(t, 0, sender.sends, "a lost claim must never reach the provider")
}

// A send that fails after the claim puts the draft back in the queue.
func TestApproveSend_FailedSendReleasesTheClaim(t *testing.T) {
	warmed := time.Now().Add(-60 * 24 * time.Hour)
	uc, _, sender := emailApprovalFixture(t, mailboxAt(true, &warmed, 25, 0))
	inner := uc.leadRepo.(*mockLeadRepo)
	inner.On("UpdateStatus", mock.Anything, "lead-email", domain.LeadStatusPendingApproval).Return(nil)
	repo := &claimingLeadRepo{mockLeadRepo: inner, claimed: true}
	uc.leadRepo = repo

	sender.On("SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", errors.New("smtp: 421 try later"))

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead-email",
		ApproachSendInput{Text: "Olá, tudo bem?", Subject: "Proposta"})

	require.Error(t, err)
	inner.AssertCalled(t, "UpdateStatus", mock.Anything, "lead-email", domain.LeadStatusPendingApproval)
	inner.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

// The approved offer is written as an outbound message on the contact's
// conversation — created on first touch — with the provider id, so the
// timeline shows it, a bounce can find it and a reply threads onto it.
func TestApproveSend_RecordsTheOfferAsAnOutboundMessage(t *testing.T) {
	warmed := time.Now().Add(-60 * 24 * time.Hour)
	uc, _, sender := emailApprovalFixture(t, mailboxAt(true, &warmed, 25, 0))

	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	uc.SetOutboundRecorder(convRepo, msgRepo)

	convRepo.On("FindByContact", mock.Anything, "ch-email", "compras@hospital.com.br").Return(nil, nil)
	var created *domain.Conversation
	convRepo.On("Create", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { created = args.Get(1).(*domain.Conversation) }).Return(nil)
	var stored *domain.Message
	msgRepo.On("Create", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { stored = args.Get(1).(*domain.Message) }).Return(true, nil)

	sender.On("SendMessageAndGetID", mock.Anything, mock.Anything, "compras@hospital.com.br", mock.Anything).
		Run(func(args mock.Arguments) {
			// The adapter stamps the id it minted onto the message metadata.
			m := args.Get(3).(*domain.Message)
			m.Metadata = map[string]any{"subject": "Proposta", "message_id": "<mid@clinica.com.br>"}
		}).
		Return("<mid@clinica.com.br>", nil)

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead-email",
		ApproachSendInput{Text: "Olá, tudo bem?", Subject: "Proposta", ActorUserID: "user-ana"})
	require.NoError(t, err)

	require.NotNil(t, created, "first touch creates the conversation")
	assert.Equal(t, "acc1", created.AccountID)
	assert.Equal(t, "ch-email", created.ChannelID)
	assert.Equal(t, "compras@hospital.com.br", created.ContactID)
	assert.True(t, created.IsBotPaused, "an outbound-first thread is a human's, the bot stays out")

	require.NotNil(t, stored)
	assert.Equal(t, "outbound", stored.Direction)
	assert.Equal(t, "user-ana", stored.From, "the approver is the sender")
	require.NotNil(t, stored.ConversationID)
	assert.Equal(t, created.ID, *stored.ConversationID)
	require.NotNil(t, stored.ExternalMessageID)
	assert.Equal(t, "<mid@clinica.com.br>", *stored.ExternalMessageID)
	assert.Equal(t, "Olá, tudo bem?", *stored.Content)
	meta := stored.Metadata.(map[string]any)
	assert.Equal(t, "<mid@clinica.com.br>", meta["message_id"])
}

// An existing conversation is reused, never duplicated.
func TestApproveSend_ReusesTheOpenConversation(t *testing.T) {
	warmed := time.Now().Add(-60 * 24 * time.Hour)
	uc, _, sender := emailApprovalFixture(t, mailboxAt(true, &warmed, 25, 0))

	convRepo := new(MockConversationRepo)
	msgRepo := new(MockMessageRepo)
	uc.SetOutboundRecorder(convRepo, msgRepo)
	convRepo.On("FindByContact", mock.Anything, "ch-email", "compras@hospital.com.br").
		Return(&domain.Conversation{ID: "conv-1", AccountID: "acc1", ChannelID: "ch-email"}, nil)
	var stored *domain.Message
	msgRepo.On("Create", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { stored = args.Get(1).(*domain.Message) }).Return(true, nil)
	sender.On("SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("<mid-2@clinica.com.br>", nil)

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead-email",
		ApproachSendInput{Text: "Olá de novo", Subject: "Proposta"})
	require.NoError(t, err)
	convRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	require.NotNil(t, stored)
	assert.Equal(t, "conv-1", *stored.ConversationID)
	// No approver id travelled (MCP / scheduled replay): still a human row.
	assert.NotEqual(t, "bot", stored.From)
}

// The trial plan must be able to draft: with the cap at 0 every self-serve
// account got AI_DRAFTS_NOT_ALLOWED on its very first offer.
func TestTrialPlan_AllowsDrafts(t *testing.T) {
	plan := domain.TrialPlan()
	account := &domain.Account{ID: "acc-trial"}
	assert.Greater(t, domain.SpendCapFor(domain.SpendAIDraft, account, plan), 0)
}
