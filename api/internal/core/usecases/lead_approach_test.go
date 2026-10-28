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

// mockApproachSender records the last send and lets a test simulate a failure.
type mockApproachSender struct{ mock.Mock }

func (m *mockApproachSender) SendMessageAndGetID(ctx context.Context, channel *domain.Channel, to string, msg *domain.Message) (string, error) {
	args := m.Called(ctx, channel, to, msg)
	return args.String(0), args.Error(1)
}

// newApproachUC builds an ApproachUseCase with all fresh mocks.
func newApproachUC() (
	*ApproachUseCase,
	*mockLeadRepo, *MockSalesFunnelRepo, *MockChannelRepo, *MockContactRepo,
	*MockAccountRepo, *MockPlanRepo, *MockProductRepo, *MockUsageRepo,
	*MockAIFactory, *MockAIProvider, *mockApproachSender,
) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	channelRepo := new(MockChannelRepo)
	contactRepo := new(MockContactRepo)
	// US-F005: contact lookups try the opaque id first and fall back to
	// external_id. These fixtures use the legacy shape (ContactID == phone),
	// so the id lookup legitimately misses.
	contactRepo.On("GetByID", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	contactRepo.On("GetByExternalID", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	productRepo := new(MockProductRepo)
	// The empty-catalogue gate lists products on every draft; the harness
	// answers with one so existing scenarios keep testing their own concern
	// (the gate has its own dedicated test). Matcher tests that need a
	// SPECIFIC catalogue register it on a separate repo instance — a
	// specific expectation here would be shadowed by this generic one.
	productRepo.On("ListByAccountID", mock.Anything, mock.Anything).
		Return([]*domain.Product{{ID: "prod1", AccountID: "acc1", Name: "Tomógrafo"}}, nil).Maybe()
	usageRepo := new(MockUsageRepo)
	aiFactory := new(MockAIFactory)
	provider := new(MockAIProvider)
	sender := new(mockApproachSender)

	uc := NewApproachUseCase(leadRepo, funnelRepo, channelRepo, contactRepo,
		accountRepo, planRepo, productRepo, usageRepo, aiFactory, sender)
	return uc, leadRepo, funnelRepo, channelRepo, contactRepo, accountRepo,
		planRepo, productRepo, usageRepo, aiFactory, provider, sender
}

// --- Draft: generates a suggestion, queues it, and sends NOTHING -------------

func TestApproach_Draft_QueuesWithoutSending(t *testing.T) {
	uc, leadRepo, funnelRepo, _, contactRepo, accountRepo, _, _, _, aiFactory, provider, sender := newApproachUC()

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "+5511999999999", Status: domain.LeadStatusEnriched}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", Name: "Vendas", PersonaPrompt: "Seja consultivo"}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", "+5511999999999").Return(false, nil)
	contactRepo.On("GetByExternalID", mock.Anything, "acc1", "+5511999999999").Return(nil, nil)
	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1"}, nil)
	aiFactory.On("GetProvider", mock.Anything, mock.Anything).Return(provider, nil)
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&domain.AICallbackResponse{Content: "  Oi! Tudo bem? "}, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	lead, err := uc.Draft(context.Background(), "acc1", "lead1")
	assert.NoError(t, err)
	assert.Equal(t, domain.LeadStatusPendingApproval, lead.Status)
	assert.Equal(t, "Oi! Tudo bem?", lead.ApproachDraft)
	// The critical invariant: drafting never sends.
	sender.AssertNotCalled(t, "SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	leadRepo.AssertExpectations(t)
}

func TestApproach_Draft_OptedOutRejected(t *testing.T) {
	uc, leadRepo, funnelRepo, _, contactRepo, _, _, _, _, _, _, sender := newApproachUC()

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "+5511999999999", Status: domain.LeadStatusEnriched}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1"}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", "+5511999999999").Return(true, nil)

	_, err := uc.Draft(context.Background(), "acc1", "lead1")
	assert.ErrorIs(t, err, domain.ErrContactOptedOut)
	sender.AssertNotCalled(t, "SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestApproach_Draft_AlreadyContacted(t *testing.T) {
	uc, leadRepo, _, _, _, _, _, _, _, _, _, _ := newApproachUC()

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", Status: domain.LeadStatusContacted}, nil)
	// Funnel still loaded by ownedLeadAndFunnel.
	funnelRepo := uc.funnelRepo.(*MockSalesFunnelRepo)
	funnelRepo.On("GetByID", mock.Anything, "f1").Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1"}, nil)

	_, err := uc.Draft(context.Background(), "acc1", "lead1")
	assert.ErrorIs(t, err, domain.ErrLeadAlreadyContacted)
}

// --- ApproveSend: sends the human-edited text, then marks contacted ----------

func TestApproach_ApproveSend_SendsEditedText(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, contactRepo, accountRepo, planRepo, _, usageRepo, _, _, sender := newApproachUC()

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "+5511999999999", Status: domain.LeadStatusPendingApproval}, nil)
	ch := "ch1"
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", ChannelID: &ch}, nil)
	channelRepo.On("GetByID", mock.Anything, "ch1").
		Return(&domain.Channel{ID: "ch1", AccountID: "acc1", Type: "whatsapp"}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", "+5511999999999").Return(false, nil)
	planID := "plan1"
	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1", PlanID: &planID}, nil)
	planRepo.On("GetByID", mock.Anything, "plan1").Return(&domain.Plan{ID: "plan1", MaxBroadcastMessagesPerMonth: 100}, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc1").Return(&domain.AccountUsage{}, nil)

	var sentTo string
	var sentMsg *domain.Message
	sender.On("SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			sentTo = args.String(2)
			sentMsg = args.Get(3).(*domain.Message)
		}).Return("wamid-1", nil)
	usageRepo.On("AddBroadcastMessages", mock.Anything, "acc1", int32(1)).Return(nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	lead, err := uc.ApproveSend(context.Background(), "acc1", "lead1", ApproachSendInput{
		Text:         "  Olá Maria, tudo bem?  ",
		TemplateName: "approach_v1",
		Buttons:      []ApproachTemplateButton{{SubType: "quick_reply", Text: "Parar de receber", Payload: "STOP"}},
	})
	assert.NoError(t, err)
	assert.Equal(t, domain.LeadStatusContacted, lead.Status)
	assert.Equal(t, "+5511999999999", sentTo)
	// Cold approach must go out as a TEMPLATE, not a free-form text (which Meta
	// would reject outside the 24h window).
	assert.Equal(t, "template", sentMsg.Type)
	meta := sentMsg.Metadata.(map[string]interface{})
	assert.Equal(t, "approach_v1", meta["template_name"])
	assert.Equal(t, "MARKETING", meta["template_category"])
	// The HUMAN-EDITED text (trimmed) rides as the template's BODY param.
	assert.Equal(t, []string{"Olá Maria, tudo bem?"}, meta["template_body_params"])
	sender.AssertExpectations(t)
}

// A cold approach with no template is rejected before any send — a free-form
// text would be undeliverable outside the 24h WhatsApp window.
//
// The check now runs AFTER the channel is resolved (US-M009): it is a
// WhatsApp rule, and applying it up front made every EMAIL approval fail
// with a WhatsApp error before the email path could run.
func TestApproach_ApproveSend_RequiresTemplate(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, contactRepo, _, _, _, _, _, _, sender := newApproachUC()

	channelID := "ch1"
	leadRepo.On("GetByID", mock.Anything, "lead1").Return(&domain.Lead{
		ID: "lead1", AccountID: "acc1", FunnelID: "f1",
		ContactID: "+5511999999999", Status: domain.LeadStatusPendingApproval,
	}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").Return(&domain.SalesFunnel{
		ID: "f1", AccountID: "acc1", ChannelID: &channelID,
	}, nil)
	channelRepo.On("GetByID", mock.Anything, channelID).Return(&domain.Channel{
		ID: channelID, AccountID: "acc1", Type: "whatsapp",
	}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", mock.Anything).Return(false, nil)

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead1", ApproachSendInput{Text: "Olá"})
	assert.ErrorIs(t, err, domain.ErrLeadApproachTemplateRequired)
	sender.AssertNotCalled(t, "SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A phone-less prospect (keyed on its Google place_id) can't be messaged — the
// E.164 guard rejects it before any Meta call. Since US-M009 the guard runs
// after the channel is resolved (an EMAIL lead legitimately has no phone),
// so the fixture now needs a channel.
func TestApproach_ApproveSend_RejectsNonPhoneContact(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, _, _, _, _, _, _, _, sender := newApproachUC()

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "ChIJplace_id_only", Status: domain.LeadStatusPendingApproval}, nil)
	phoneCh := "ch1"
	funnelRepo.On("GetByID", mock.Anything, "f1").Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", ChannelID: &phoneCh}, nil)
	channelRepo.On("GetByID", mock.Anything, "ch1").Return(&domain.Channel{ID: "ch1", AccountID: "acc1", Type: "whatsapp"}, nil)

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead1", ApproachSendInput{Text: "Olá", TemplateName: "approach_v1"})
	assert.ErrorIs(t, err, domain.ErrLeadContactNotMessageable)
	sender.AssertNotCalled(t, "SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestApproach_ApproveSend_OptedOutBlocked(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, contactRepo, _, _, _, _, _, _, sender := newApproachUC()

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "+5511999999999", Status: domain.LeadStatusPendingApproval}, nil)
	ch := "ch1"
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", ChannelID: &ch}, nil)
	channelRepo.On("GetByID", mock.Anything, "ch1").
		Return(&domain.Channel{ID: "ch1", AccountID: "acc1", Type: "whatsapp"}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", "+5511999999999").Return(true, nil)

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead1", ApproachSendInput{Text: "Olá", TemplateName: "approach_v1"})
	assert.ErrorIs(t, err, domain.ErrContactOptedOut)
	sender.AssertNotCalled(t, "SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestApproach_ApproveSend_RequiresPendingApproval(t *testing.T) {
	uc, leadRepo, funnelRepo, _, _, _, _, _, _, _, _, sender := newApproachUC()

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", Status: domain.LeadStatusEnriched}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1"}, nil)

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead1", ApproachSendInput{Text: "Olá", TemplateName: "approach_v1"})
	assert.ErrorIs(t, err, domain.ErrLeadNotPendingApproval)
	sender.AssertNotCalled(t, "SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestApproach_ApproveSend_EmptyTextRejected(t *testing.T) {
	uc, _, _, _, _, _, _, _, _, _, _, sender := newApproachUC()

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead1", ApproachSendInput{Text: "   ", TemplateName: "approach_v1"})
	assert.ErrorIs(t, err, domain.ErrLeadApproachTextRequired)
	sender.AssertNotCalled(t, "SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestApproach_ApproveSend_SendFailureKeepsPending(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, contactRepo, accountRepo, planRepo, _, usageRepo, _, _, sender := newApproachUC()

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "+5511999999999", Status: domain.LeadStatusPendingApproval}, nil)
	ch := "ch1"
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", ChannelID: &ch}, nil)
	channelRepo.On("GetByID", mock.Anything, "ch1").
		Return(&domain.Channel{ID: "ch1", AccountID: "acc1", Type: "whatsapp"}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", "+5511999999999").Return(false, nil)
	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1"}, nil)
	planRepo.On("GetByID", mock.Anything, mock.Anything).Return(&domain.Plan{MaxBroadcastMessagesPerMonth: 100}, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc1").Return(&domain.AccountUsage{}, nil)
	sender.On("SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", errors.New("meta 131056 pair rate limit"))

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead1", ApproachSendInput{Text: "Olá", TemplateName: "approach_v1"})
	assert.Error(t, err)
	// Status write (Upsert) must NOT have happened — lead stays pending_approval.
	leadRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

func TestApproach_ApproveSend_NoChannelOnFunnel(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, _, _, _, _, _, _, _, sender := newApproachUC()

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "+5511999999999", Status: domain.LeadStatusPendingApproval}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1"}, nil)
	// A funnel without a channel falls back to the account's email channel
	// (analysis 2026-09-01); an account with none is still refused.
	channelRepo.On("ListByAccountID", mock.Anything, "acc1").Return([]*domain.Channel{}, nil)

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead1", ApproachSendInput{Text: "Olá", TemplateName: "approach_v1"})
	assert.ErrorIs(t, err, domain.ErrLeadFunnelNoChannel)
	sender.AssertNotCalled(t, "SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// --- DraftBatch: filters opted-out before enqueuing and reports skips --------

func TestApproach_DraftBatch_FiltersOptedOut(t *testing.T) {
	uc, leadRepo, funnelRepo, _, contactRepo, accountRepo, _, _, _, aiFactory, provider, sender := newApproachUC()

	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", Name: "Vendas"}, nil)
	leadRepo.On("ListByFunnel", mock.Anything, "f1", domain.LeadStatus(""), mock.Anything, mock.Anything).
		Return([]*domain.Lead{
			{ID: "l1", AccountID: "acc1", FunnelID: "f1", ContactID: "111", Status: domain.LeadStatusEnriched},
			{ID: "l2", AccountID: "acc1", FunnelID: "f1", ContactID: "222", Status: domain.LeadStatusProspected},
			{ID: "l3", AccountID: "acc1", FunnelID: "f1", ContactID: "333", Status: domain.LeadStatusContacted}, // not draftable
		}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", "111").Return(false, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", "222").Return(true, nil) // skipped
	contactRepo.On("GetByExternalID", mock.Anything, "acc1", mock.Anything).Return(nil, nil)
	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1"}, nil)
	aiFactory.On("GetProvider", mock.Anything, mock.Anything).Return(provider, nil)
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&domain.AICallbackResponse{Content: "Oi!"}, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	res, err := uc.DraftBatch(context.Background(), "acc1", "f1", 0)
	assert.NoError(t, err)
	assert.Equal(t, 1, res.Drafted)         // only l1
	assert.Equal(t, 1, res.SkippedOptedOut) // l2
	assert.Equal(t, []string{"l1"}, res.LeadIDs)
	// Drafting never sends — even in bulk.
	sender.AssertNotCalled(t, "SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestApproach_DraftBatch_OwnershipDenied(t *testing.T) {
	uc, _, funnelRepo, _, _, _, _, _, _, _, _, _ := newApproachUC()

	funnelRepo.On("GetByID", mock.Anything, "f1").Return(&domain.SalesFunnel{ID: "f1", AccountID: "other"}, nil)

	_, err := uc.DraftBatch(context.Background(), "acc1", "f1", 0)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

// ── Email approval path (US-M007/M009) ──────────────────────────────────────
//
// The guards only matter if they sit on the path a send actually takes.
// These drive ApproveSend with an EMAIL channel and assert the send is
// refused — or counted — for the right reason.

type fakeMailboxRepo struct {
	mailbox  *domain.EmailMailbox
	recorded int
	err      error
	// proveErr fails the proof write while the send itself succeeded — the
	// case where the customer's provider accepted the mail but we could not
	// remember that it did.
	proveErr     error
	smtpProvenAt *time.Time
	cleared      int
}

func (f *fakeMailboxRepo) EnsureForChannel(_ context.Context, _, _, _ string) (*domain.EmailMailbox, error) {
	return f.mailbox, f.err
}
func (f *fakeMailboxRepo) GetByAddress(context.Context, string, string) (*domain.EmailMailbox, error) {
	return f.mailbox, f.err
}
func (f *fakeMailboxRepo) ListByAccount(context.Context, string) ([]*domain.EmailMailbox, error) {
	return []*domain.EmailMailbox{f.mailbox}, f.err
}
func (f *fakeMailboxRepo) RecordSend(_ context.Context, _ string, _ time.Time) (int, error) {
	f.recorded++
	return f.recorded, nil
}
func (f *fakeMailboxRepo) MarkDNSVerified(context.Context, string, string, time.Time) error {
	return nil
}
func (f *fakeMailboxRepo) MarkSMTPProven(_ context.Context, _, _ string, at time.Time) error {
	f.smtpProvenAt = &at
	return f.proveErr
}
func (f *fakeMailboxRepo) ClearSMTPProof(context.Context, string, string) error {
	f.cleared++
	return nil
}
func (f *fakeMailboxRepo) StartWarmup(context.Context, string, string, time.Time) error { return nil }
func (f *fakeMailboxRepo) CompleteWarmup(context.Context, string, string, time.Time) error {
	return nil
}
func (f *fakeMailboxRepo) SetDailyCap(context.Context, string, string, int) error { return nil }
func (f *fakeMailboxRepo) SetBulkDomain(context.Context, string, string, string) error {
	return nil
}
func (f *fakeMailboxRepo) MarkBulkVerified(context.Context, string, string, time.Time) error {
	return nil
}

type countingSender struct {
	mock.Mock
	sends int
}

func (c *countingSender) SendMessageAndGetID(ctx context.Context, ch *domain.Channel, to string, msg *domain.Message) (string, error) {
	c.sends++
	args := c.Called(ctx, ch, to, msg)
	return args.String(0), args.Error(1)
}

// emailApprovalFixture wires a lead ready for approval on an email channel.
func emailApprovalFixture(t *testing.T, mailbox *domain.EmailMailbox) (
	*ApproachUseCase, *fakeMailboxRepo, *countingSender,
) {
	t.Helper()
	uc, leadRepo, funnelRepo, channelRepo, contactRepo, accountRepo, planRepo, _, usageRepo, _, _, _ := newApproachUC()

	// Broadcast budget: an approach is billed like a broadcast on every
	// channel, email included.
	planID := "plan1"
	accountRepo.On("GetByID", mock.Anything, "acc1").
		Return(&domain.Account{ID: "acc1", PlanID: &planID}, nil).Maybe()
	planRepo.On("GetByID", mock.Anything, "plan1").
		Return(&domain.Plan{ID: "plan1", MaxBroadcastMessagesPerMonth: 100}, nil).Maybe()
	usageRepo.On("GetByAccountID", mock.Anything, "acc1").Return(&domain.AccountUsage{}, nil).Maybe()
	usageRepo.On("AddBroadcastMessages", mock.Anything, "acc1", int32(1)).Return(nil).Maybe()

	leadRepo.On("GetByID", mock.Anything, "lead-email").Return(&domain.Lead{
		ID: "lead-email", AccountID: "acc1", FunnelID: "f1",
		ContactID: "compras@hospital.com.br", Status: domain.LeadStatusPendingApproval,
	}, nil)
	channelID := "ch-email"
	funnelRepo.On("GetByID", mock.Anything, "f1").Return(&domain.SalesFunnel{
		ID: "f1", AccountID: "acc1", ChannelID: &channelID,
	}, nil)
	channelRepo.On("GetByID", mock.Anything, channelID).Return(&domain.Channel{
		ID: channelID, AccountID: "acc1", Type: "email",
		Config: map[string]any{"from_address": "comercial@clinica.com.br"},
	}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", mock.Anything).Return(false, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	repo := &fakeMailboxRepo{mailbox: mailbox}
	sender := &countingSender{}
	uc.SetMailboxRepo(repo)
	uc.SetEmailSender(sender)
	return uc, repo, sender
}

func mailboxAt(dnsVerified bool, warmStart *time.Time, cap, sentToday int) *domain.EmailMailbox {
	m := &domain.EmailMailbox{
		ID: "mb-1", Address: "comercial@clinica.com.br", Domain: "clinica.com.br",
		DailyCap: cap, SentToday: sentToday, WarmupStartedAt: warmStart,
	}
	if dnsVerified {
		now := time.Now()
		m.DNSVerifiedAt = &now
	}
	if sentToday > 0 {
		today := time.Now()
		m.SentDate = &today
	}
	return m
}

// An unauthenticated domain must not send cold mail at all — the receiving
// side would reject or spam-folder it, and the attempt only costs reputation.
func TestApproach_EmailRefusedWhenDomainNotVerified(t *testing.T) {
	uc, _, sender := emailApprovalFixture(t, mailboxAt(false, nil, 25, 0))

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead-email",
		ApproachSendInput{Text: "Olá, tudo bem?", Subject: "Proposta"})

	assert.ErrorIs(t, err, domain.ErrEmailDomainNotVerified)
	assert.Zero(t, sender.sends, "nothing may leave before DNS is published")
}

// The fail-open this closes: warmup_started_at had NO writer, so it was NULL
// on every mailbox and the warmup guard released everything. NULL means
// "don't know", and not knowing never releases — this test FAILED on the old
// code (the send went out).
func TestApproach_EmailRefusedWhenWarmupNeverStarted(t *testing.T) {
	uc, _, sender := emailApprovalFixture(t, mailboxAt(true, nil, 25, 0))

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead-email",
		ApproachSendInput{Text: "Olá", Subject: "Proposta"})

	assert.ErrorIs(t, err, domain.ErrEmailWarmupNotStarted)
	assert.Zero(t, sender.sends, "an identity of unknown age must not open cold threads")
}

// Inside the ramp the ceiling is small but real: above it, the refusal tells
// the person how much went out and why the limit is what it is.
func TestApproach_EmailRefusedAboveTheWarmupDailyLimit(t *testing.T) {
	start := time.Now().Add(-24 * time.Hour) // day 2 of the ramp
	uc, _, sender := emailApprovalFixture(t,
		mailboxAt(true, &start, 25, domain.WarmupInitialCap+1))

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead-email",
		ApproachSendInput{Text: "Olá", Subject: "Proposta"})

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrEmailDailyCap.Code, appErr.Code)
	assert.Contains(t, appErr.Message, "aquecimento",
		"the refusal explains the ramp, not just the wall")
	assert.Contains(t, appErr.Message, "dia 2",
		"the person learns where in the ramp they are")
	assert.Zero(t, sender.sends)
}

func TestApproach_EmailRefusedWhenDailyCapReached(t *testing.T) {
	start := time.Now().Add(-60 * 24 * time.Hour) // long past the ramp
	uc, _, sender := emailApprovalFixture(t, mailboxAt(true, &start, 3, 3))

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead-email",
		ApproachSendInput{Text: "Olá", Subject: "Proposta"})

	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, domain.ErrEmailDailyCap.Code, appErr.Code)
	assert.Contains(t, appErr.Message, "3 de 3", "used and cap travel in the message")
	assert.Zero(t, sender.sends)
}

// ── Hybrid bulk sending (2026-08-25) ────────────────────────────────────────

func bulkReadyMailbox(m *domain.EmailMailbox) *domain.EmailMailbox {
	id := "dom_1"
	at := time.Now()
	m.BulkDomainID = &id
	m.BulkVerifiedAt = &at
	return m
}

// A batch with no provider wired must refuse — never fall back to the
// customer's mailbox, which is exactly the burn the hybrid model prevents.
func TestApproach_BulkRefusedWhenProviderUnwired(t *testing.T) {
	uc, _, sender := emailApprovalFixture(t, bulkReadyMailbox(mailboxAt(true, nil, 25, 0)))

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead-email",
		ApproachSendInput{Text: "Olá", Subject: "Proposta", ViaBulk: true})

	assert.ErrorIs(t, err, domain.ErrBulkSendingUnavailable)
	assert.Zero(t, sender.sends)
}

// A batch on a domain not registered+verified with the provider refuses with
// the code the app turns into a "configure volume sending" CTA.
func TestApproach_BulkRefusedWhenDomainNotBulkVerified(t *testing.T) {
	uc, _, sender := emailApprovalFixture(t, mailboxAt(true, nil, 25, 0))
	bulk := &countingSender{}
	uc.SetBulkSender(bulk)

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead-email",
		ApproachSendInput{Text: "Olá", Subject: "Proposta", ViaBulk: true})

	assert.ErrorIs(t, err, domain.ErrBulkSendingNotReady)
	assert.Zero(t, bulk.sends)
	assert.Zero(t, sender.sends, "the mailbox path must not be used as a fallback")
}

// A bulk-verified batch goes through the BULK sender and skips the mailbox's
// warmup/daily-cap machinery entirely — the provider's reputation carries the
// volume. Broadcast budget and lead transition stay identical.
func TestApproach_BulkSendsThroughProviderSkippingMailboxCaps(t *testing.T) {
	// DNS unverified + warmup NULL: every mailbox gate would refuse — the
	// bulk path must not consult them.
	uc, repo, sender := emailApprovalFixture(t, bulkReadyMailbox(mailboxAt(false, nil, 25, 0)))
	bulk := &countingSender{}
	bulk.On("SendMessageAndGetID", mock.Anything, mock.Anything, "compras@hospital.com.br",
		mock.Anything).Return("email_1", nil)
	uc.SetBulkSender(bulk)

	lead, err := uc.ApproveSend(context.Background(), "acc1", "lead-email",
		ApproachSendInput{Text: "Olá, proposta em anexo", Subject: "Proposta", ViaBulk: true})

	require.NoError(t, err)
	assert.Equal(t, 1, bulk.sends)
	assert.Zero(t, sender.sends, "the customer's mailbox must not carry a batch")
	assert.Zero(t, repo.recorded, "a bulk send must not spend the mailbox's daily budget")
	assert.Equal(t, domain.LeadStatusContacted, lead.Status)
}

// The happy path: sent through the EMAIL sender (not the WhatsApp one), with
// the subject carried, and counted against the cap only after delivery.
func TestApproach_EmailSendsAndCountsAgainstTheCap(t *testing.T) {
	warmed := time.Now().Add(-60 * 24 * time.Hour)
	uc, repo, sender := emailApprovalFixture(t, mailboxAt(true, &warmed, 25, 0))
	sender.On("SendMessageAndGetID", mock.Anything, mock.Anything, "compras@hospital.com.br",
		mock.MatchedBy(func(m *domain.Message) bool {
			meta, _ := m.Metadata.(map[string]any)
			return m.Type == "text" && m.Content != nil && meta["subject"] == "Proposta de equipamentos"
		})).Return("<mid@clinica.com.br>", nil)

	lead, err := uc.ApproveSend(context.Background(), "acc1", "lead-email",
		ApproachSendInput{Text: "Olá, vi que a clínica atende convênios.", Subject: "Proposta de equipamentos"})

	require.NoError(t, err)
	assert.Equal(t, domain.LeadStatusContacted, lead.Status)
	assert.Equal(t, 1, sender.sends)
	assert.Equal(t, 1, repo.recorded, "the cap counts DELIVERIES, not attempts")
}

// A provider failure must not consume the customer's daily budget — a cap
// that counted attempts would shrink every time SMTP hiccuped.
func TestApproach_EmailFailureDoesNotConsumeBudget(t *testing.T) {
	warmed := time.Now().Add(-60 * 24 * time.Hour)
	uc, repo, sender := emailApprovalFixture(t, mailboxAt(true, &warmed, 25, 0))
	sender.On("SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", errors.New("smtp: connection refused"))

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead-email",
		ApproachSendInput{Text: "Olá", Subject: "Proposta"})

	require.Error(t, err)
	assert.Zero(t, repo.recorded)
}

// Unwired guards mean no protection at all. Refusing is the only safe
// default: sending unguarded cold mail from a customer's domain is exactly
// the harm US-M007 exists to prevent.
func TestApproach_EmailRefusedWhenGuardsUnwired(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, _, _, _, _, _, _, _, _ := newApproachUC()
	// deliberately no SetMailboxRepo / SetEmailSender

	channelID := "ch-email"
	leadRepo.On("GetByID", mock.Anything, "lead-email").Return(&domain.Lead{
		ID: "lead-email", AccountID: "acc1", FunnelID: "f1",
		ContactID: "compras@hospital.com.br", Status: domain.LeadStatusPendingApproval,
	}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").Return(&domain.SalesFunnel{
		ID: "f1", AccountID: "acc1", ChannelID: &channelID,
	}, nil)
	channelRepo.On("GetByID", mock.Anything, channelID).Return(&domain.Channel{
		ID: channelID, AccountID: "acc1", Type: "email",
		Config: map[string]any{"from_address": "comercial@clinica.com.br"},
	}, nil)

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead-email", ApproachSendInput{Text: "Olá"})
	require.Error(t, err, "unguarded cold mail from a customer domain is the harm US-M007 prevents")
}

// --- Case instrumentation at the send point (US-M011) -----------------------

// fakeSalesEvents captures what the pipeline recorded, and can fail on
// demand so the "instrumentation never breaks the send" rule is provable.
type fakeSalesEvents struct {
	events []*domain.SalesEvent
	err    error
}

func (f *fakeSalesEvents) Record(_ context.Context, e *domain.SalesEvent) error {
	if f.err != nil {
		return f.err
	}
	f.events = append(f.events, e)
	return nil
}

func (f *fakeSalesEvents) Metrics(context.Context, string, time.Time, time.Time) (*domain.CaseMetrics, error) {
	return &domain.CaseMetrics{}, nil
}

func (f *fakeSalesEvents) CountSince(context.Context, string, domain.SalesEventKind, time.Time) (int64, error) {
	return 0, nil
}

func (f *fakeSalesEvents) LastApproachAt(context.Context, string) (time.Time, error) {
	return time.Time{}, nil
}

// The send point records the approach with the channel it actually went
// through — the case report's per-channel breakdown is only worth reading
// if it reflects where the message left from.
func TestApproach_ApproveSend_RecordsTheCaseEvent(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, contactRepo, accountRepo, planRepo, _, usageRepo, _, _, sender := newApproachUC()
	events := &fakeSalesEvents{}
	uc.SetCaseRecorder(NewCaseRecorder(events))

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "+5511999999999", Status: domain.LeadStatusPendingApproval}, nil)
	ch := "ch1"
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", ChannelID: &ch}, nil)
	channelRepo.On("GetByID", mock.Anything, "ch1").
		Return(&domain.Channel{ID: "ch1", AccountID: "acc1", Type: "whatsapp"}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", "+5511999999999").Return(false, nil)
	planID := "plan1"
	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1", PlanID: &planID}, nil)
	planRepo.On("GetByID", mock.Anything, "plan1").Return(&domain.Plan{ID: "plan1", MaxBroadcastMessagesPerMonth: 100}, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc1").Return(&domain.AccountUsage{}, nil)
	sender.On("SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("wamid-1", nil)
	usageRepo.On("AddBroadcastMessages", mock.Anything, "acc1", int32(1)).Return(nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead1", ApproachSendInput{
		Text:         "Olá",
		TemplateName: "approach_v1",
		Buttons:      []ApproachTemplateButton{{SubType: "quick_reply", Text: "Parar de receber", Payload: "STOP"}},
	})
	require.NoError(t, err)

	require.Len(t, events.events, 1)
	assert.Equal(t, domain.SalesEventApproachSent, events.events[0].Kind)
	assert.Equal(t, "whatsapp", events.events[0].ChannelType)
	require.NotNil(t, events.events[0].LeadID)
	assert.Equal(t, "lead1", *events.events[0].LeadID)
}

// A send that FAILED must not appear in the case report — a report that
// counts attempts as sends is worse than no report.
func TestApproach_ApproveSend_FailedSendIsNotRecorded(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, contactRepo, accountRepo, planRepo, _, usageRepo, _, _, sender := newApproachUC()
	events := &fakeSalesEvents{}
	uc.SetCaseRecorder(NewCaseRecorder(events))

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "+5511999999999", Status: domain.LeadStatusPendingApproval}, nil)
	ch := "ch1"
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", ChannelID: &ch}, nil)
	channelRepo.On("GetByID", mock.Anything, "ch1").
		Return(&domain.Channel{ID: "ch1", AccountID: "acc1", Type: "whatsapp"}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", "+5511999999999").Return(false, nil)
	planID := "plan1"
	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1", PlanID: &planID}, nil)
	planRepo.On("GetByID", mock.Anything, "plan1").Return(&domain.Plan{ID: "plan1", MaxBroadcastMessagesPerMonth: 100}, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc1").Return(&domain.AccountUsage{}, nil)
	sender.On("SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", errors.New("meta rejected"))

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead1", ApproachSendInput{
		Text:         "Olá",
		TemplateName: "approach_v1",
		Buttons:      []ApproachTemplateButton{{SubType: "quick_reply", Text: "Parar", Payload: "STOP"}},
	})
	require.Error(t, err)
	assert.Empty(t, events.events)
}

// Instrumentation must never be able to break the thing it measures: a
// recorder that errors leaves the send successful.
func TestApproach_ApproveSend_RecorderFailureDoesNotBreakTheSend(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, contactRepo, accountRepo, planRepo, _, usageRepo, _, _, sender := newApproachUC()
	uc.SetCaseRecorder(NewCaseRecorder(&fakeSalesEvents{err: errors.New("db down")}))

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "+5511999999999", Status: domain.LeadStatusPendingApproval}, nil)
	ch := "ch1"
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", ChannelID: &ch}, nil)
	channelRepo.On("GetByID", mock.Anything, "ch1").
		Return(&domain.Channel{ID: "ch1", AccountID: "acc1", Type: "whatsapp"}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", "+5511999999999").Return(false, nil)
	planID := "plan1"
	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1", PlanID: &planID}, nil)
	planRepo.On("GetByID", mock.Anything, "plan1").Return(&domain.Plan{ID: "plan1", MaxBroadcastMessagesPerMonth: 100}, nil)
	usageRepo.On("GetByAccountID", mock.Anything, "acc1").Return(&domain.AccountUsage{}, nil)
	sender.On("SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("wamid-1", nil)
	usageRepo.On("AddBroadcastMessages", mock.Anything, "acc1", int32(1)).Return(nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	lead, err := uc.ApproveSend(context.Background(), "acc1", "lead1", ApproachSendInput{
		Text:         "Olá",
		TemplateName: "approach_v1",
		Buttons:      []ApproachTemplateButton{{SubType: "quick_reply", Text: "Parar", Payload: "STOP"}},
	})
	require.NoError(t, err)
	assert.Equal(t, domain.LeadStatusContacted, lead.Status)
}

// The one outbound check that BLOCKS. Everything else this system finds is a
// judgement the approver may legitimately make; "Olá {{nome}}" is not a
// judgement, it is a broken message.
func TestApproveSend_RefusesAnUnfilledPlaceholder(t *testing.T) {
	// Every dependency nil on purpose: the placeholder check runs BEFORE any
	// repository is touched, and this proves it — a broken message never
	// reaches the lead lookup, let alone the sender.
	uc := NewApproachUseCase(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	for _, text := range []string{
		"Olá {{nome}}, tudo bem?",
		"Atenciosamente, [Seu Nome]",
		"Ligue para XXXX",
	} {
		_, err := uc.ApproveSend(context.Background(), "acc-1", "lead-1",
			ApproachSendInput{Text: text})

		require.Error(t, err, "input %q", text)
		var appErr *domain.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, domain.ErrApproachHasPlaceholder.Code, appErr.Code, "input %q", text)
		// The offending fragment is named, so the person fixing it does not
		// have to hunt for it in a long message.
		assert.Greater(t, len(appErr.Message),
			len(domain.ErrApproachHasPlaceholder.Message),
			"the error must quote what to fix")
	}
}

// --- SaveDraft: keeps what the person wrote, sends nothing ------------------
//
// The only writers of approach_draft were the AI and the send. Whoever typed
// their own text and closed the panel lost all of it — the words lived in a screen
// field and nowhere else. Losing what somebody wrote is the bug that makes them
// stop writing in the tool.

func TestApproach_SaveDraft_PersistsWithoutSendingOrQueuing(t *testing.T) {
	uc, leadRepo, _, _, _, _, _, _, _, _, _, sender := newApproachUC()

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", Status: domain.LeadStatusEnriched}, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	lead, err := uc.SaveDraft(context.Background(), "acc1", "lead1", "Oi, vi que vocês atendem convênio")
	require.NoError(t, err)
	assert.Equal(t, "Oi, vi que vocês atendem convênio", lead.ApproachDraft)
	// Queuing is a DECISION ("this is ready for review"); saving half a sentence is
	// not that decision.
	assert.Equal(t, domain.LeadStatusEnriched, lead.Status)
	sender.AssertNotCalled(t, "SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	leadRepo.AssertExpectations(t)
}

// A lead already in the queue keeps its place: the text under review is simply
// the current text.
func TestApproach_SaveDraft_KeepsPendingApprovalInPlace(t *testing.T) {
	uc, leadRepo, _, _, _, _, _, _, _, _, _, _ := newApproachUC()

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", Status: domain.LeadStatusPendingApproval, ApproachDraft: "antigo"}, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	lead, err := uc.SaveDraft(context.Background(), "acc1", "lead1", "reescrito")
	require.NoError(t, err)
	assert.Equal(t, domain.LeadStatusPendingApproval, lead.Status)
	assert.Equal(t, "reescrito", lead.ApproachDraft)
}

// Clearing the box is an edit. Refusing to persist the empty value would
// resurrect a draft the person deleted on purpose.
func TestApproach_SaveDraft_EmptyClearsIt(t *testing.T) {
	uc, leadRepo, _, _, _, _, _, _, _, _, _, _ := newApproachUC()

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", Status: domain.LeadStatusEnriched, ApproachDraft: "tinha texto"}, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	lead, err := uc.SaveDraft(context.Background(), "acc1", "lead1", "")
	require.NoError(t, err)
	assert.Equal(t, "", lead.ApproachDraft)
}

// Identical text is not worth a write: the panel saves on close, and closing
// without editing is the common case.
func TestApproach_SaveDraft_UnchangedSkipsWrite(t *testing.T) {
	uc, leadRepo, _, _, _, _, _, _, _, _, _, _ := newApproachUC()

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", Status: domain.LeadStatusEnriched, ApproachDraft: "igual"}, nil)

	_, err := uc.SaveDraft(context.Background(), "acc1", "lead1", "igual")
	require.NoError(t, err)
	leadRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

// Rewriting the draft of an already-contacted lead would rewrite what WAS sent,
// as far as every screen showing it is concerned.
func TestApproach_SaveDraft_RefusesAfterContact(t *testing.T) {
	uc, leadRepo, _, _, _, _, _, _, _, _, _, _ := newApproachUC()

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", Status: domain.LeadStatusContacted, ApproachDraft: "enviado"}, nil)

	_, err := uc.SaveDraft(context.Background(), "acc1", "lead1", "novo")
	assert.ErrorIs(t, err, domain.ErrLeadAlreadyContacted)
	leadRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

// Missing and belonging-to-another-account answer the same: a guessed id must not
// probe another tenant's funnel.
func TestApproach_SaveDraft_CrossAccountLooksMissing(t *testing.T) {
	uc, leadRepo, _, _, _, _, _, _, _, _, _, _ := newApproachUC()

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "outra", Status: domain.LeadStatusEnriched}, nil)

	_, err := uc.SaveDraft(context.Background(), "acc1", "lead1", "x")
	assert.ErrorIs(t, err, domain.ErrLeadNotFound)
}

// --- The dossier reaching the draft ----------------------------------------
//
// The generator wrote from the funnel's persona and one product while the same
// account already held its own co-purchase patterns, this customer's purchase
// history and the signals somebody recorded. None of it reached the message.

func TestApproach_DraftPayloadCarriesTheDossier(t *testing.T) {
	uc, leadRepo, funnelRepo, _, contactRepo, accountRepo, _, _, _, aiFactory, provider, _ := newApproachUC()

	uc.SetDossierBuilder(NewLeadDossierBuilder().SetCrossSell(stubCrossSell{
		bought: []string{"luva"},
		pairs: []domain.CoPurchasePair{
			{Product: "luva", Partner: "mascara", Customers: 8, ProductCustomers: 11},
		},
	}))

	lead := &domain.Lead{
		ID: "lead1", AccountID: "acc1", FunnelID: "f1",
		ContactID: "contact-1", Status: domain.LeadStatusEnriched,
	}
	leadRepo.On("GetByID", mock.Anything, "lead1").Return(lead, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", Name: "Vendas"}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", "contact-1").Return(false, nil)
	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1"}, nil)
	aiFactory.On("GetProvider", mock.Anything, mock.Anything).Return(provider, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	var seen string
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			msgs := args.Get(2).([]domain.ChatMessage)
			seen = msgs[0].Content
		}).
		Return(&domain.AICallbackResponse{Content: "Oi"}, nil)

	_, err := uc.Draft(context.Background(), "acc1", "lead1")
	require.NoError(t, err)

	// The pattern AND its evidence: the numerator and denominator are what turn
	// a guess in a confident voice into a sentence the salesperson can defend.
	assert.Contains(t, seen, "mascara")
	assert.Contains(t, seen, "8 clientes que compram luva também levam")
	// And the gaps, named, so the model asks instead of inventing.
	assert.Contains(t, seen, "Ainda não sabemos")
}

// Unwired, the payload is exactly what it was before the dossier existed — an
// account that has not been migrated drafts as it always did.
func TestApproach_WithoutADossierThePayloadIsUnchanged(t *testing.T) {
	uc, leadRepo, funnelRepo, _, contactRepo, accountRepo, _, _, _, aiFactory, provider, _ := newApproachUC()

	lead := &domain.Lead{
		ID: "lead1", AccountID: "acc1", FunnelID: "f1",
		ContactID: "contact-1", Status: domain.LeadStatusEnriched,
		Enrichment: []byte(`{"decision_maker_name":"Ana"}`),
	}
	leadRepo.On("GetByID", mock.Anything, "lead1").Return(lead, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", Name: "Vendas"}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", "contact-1").Return(false, nil)
	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1"}, nil)
	aiFactory.On("GetProvider", mock.Anything, mock.Anything).Return(provider, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	var seen string
	provider.On("GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			seen = args.Get(2).([]domain.ChatMessage)[0].Content
		}).
		Return(&domain.AICallbackResponse{Content: "Oi"}, nil)

	_, err := uc.Draft(context.Background(), "acc1", "lead1")
	require.NoError(t, err)

	assert.Contains(t, seen, "Enriquecimento (dados do decisor/empresa)")
	assert.NotContains(t, seen, "O que já sabemos deste lead")
}

// An empty catalogue refuses to draft AT ALL. With nothing true to offer,
// the model pads the message with generic filler — forty of those reached a
// real approval queue before this gate existed.
func TestApproach_Draft_RefusesWithoutProducts(t *testing.T) {
	uc, leadRepo, funnelRepo, _, contactRepo, accountRepo, _, _, _, aiFactory, provider, sender := newApproachUC()

	// Override the harness catalogue with a confirmed-empty one. Registered
	// later but with the SAME generic args — so make it authoritative by
	// unsetting the harness default first.
	uc.productRepo = emptyProductRepo{}

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "+5511999999999", Status: domain.LeadStatusEnriched}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", Name: "Vendas"}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", "+5511999999999").Return(false, nil)
	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1"}, nil)

	_, err := uc.Draft(context.Background(), "acc1", "lead1")
	assert.ErrorIs(t, err, domain.ErrApproachNeedsProducts)
	// Nothing was generated and nothing was sent.
	aiFactory.AssertNotCalled(t, "GetProvider", mock.Anything, mock.Anything)
	provider.AssertNotCalled(t, "GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	sender.AssertNotCalled(t, "SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// emptyProductRepo is a ProductRepository whose catalogue is confirmed empty.
type emptyProductRepo struct{}

func (emptyProductRepo) Create(context.Context, *domain.Product) error { return nil }
func (emptyProductRepo) GetByID(context.Context, string) (*domain.Product, error) {
	return nil, nil
}

// An account with nowhere to send refuses to draft AT ALL, for the same
// reason an empty catalogue does. Production had 89 finished offers and zero
// channels, growing by forty a night: an offer with no door does not wait,
// it ages.
func TestApproach_Draft_RefusesWithoutAChannel(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, contactRepo, accountRepo, _, _, _, aiFactory, provider, sender := newApproachUC()

	// The gate reads through its own narrow port, wired here only.
	channelRepo.On("ListByAccountID", mock.Anything, "acc1").
		Return([]*domain.Channel{}, nil)
	uc.SetSendChannels(channelRepo)

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "+5511999999999", Status: domain.LeadStatusEnriched}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", Name: "Vendas"}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", "+5511999999999").Return(false, nil)
	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1"}, nil)

	_, err := uc.Draft(context.Background(), "acc1", "lead1")
	assert.ErrorIs(t, err, domain.ErrApproachNeedsChannel)
	// No model call, no spend, nothing sent.
	aiFactory.AssertNotCalled(t, "GetProvider", mock.Anything, mock.Anything)
	provider.AssertNotCalled(t, "GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	sender.AssertNotCalled(t, "SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The internal CS thread with the Replider platform is not a way to reach a
// hospital, so an account whose only channel is that one still cannot draft.
func TestApproach_Draft_TheInternalThreadIsNotADoor(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, contactRepo, accountRepo, _, _, _, _, _, _ := newApproachUC()

	channelRepo.On("ListByAccountID", mock.Anything, "acc1").
		Return([]*domain.Channel{
			{ID: "ch-internal", AccountID: "acc1", Type: domain.ChannelTypeInternal, IsActive: true},
			{ID: "ch-paused", AccountID: "acc1", Type: "email", IsActive: false},
		}, nil)
	uc.SetSendChannels(channelRepo)

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "+5511999999999", Status: domain.LeadStatusEnriched}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", Name: "Vendas"}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", "+5511999999999").Return(false, nil)
	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1"}, nil)

	_, err := uc.Draft(context.Background(), "acc1", "lead1")
	assert.ErrorIs(t, err, domain.ErrApproachNeedsChannel)
}

// A channel-lookup FAILURE must not stop a working account from drafting —
// fail-open, same posture as the catalogue gate. A database blip is not a
// reason to refuse work.
//
// The assertion is deliberately narrow: the draft is allowed to fail for some
// OTHER reason (there is no AI provider wired here). What matters is that it
// gets past the gate rather than being refused by it.
func TestApproach_Draft_AChannelReadErrorFailsOpen(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, contactRepo, accountRepo, _, _, _, aiFactory, _, _ := newApproachUC()

	channelRepo.On("ListByAccountID", mock.Anything, "acc1").
		Return([]*domain.Channel(nil), errors.New("db blip"))
	uc.SetSendChannels(channelRepo)

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "+5511999999999", Status: domain.LeadStatusEnriched}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", Name: "Vendas"}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", "+5511999999999").Return(false, nil)
	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1"}, nil)
	aiFactory.On("GetProvider", mock.Anything, mock.Anything).
		Return(nil, errors.New("no provider in this harness")).Maybe()

	_, err := uc.Draft(context.Background(), "acc1", "lead1")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrApproachNeedsChannel,
		"a failed channel read must not be read as an empty channel list")
}

func (emptyProductRepo) ListByAccountID(context.Context, string) ([]*domain.Product, error) {
	return []*domain.Product{}, nil
}
func (emptyProductRepo) Update(context.Context, *domain.Product) error { return nil }
func (emptyProductRepo) Delete(context.Context, string) error          { return nil }

// The opt-out answer comes from the RESOLVED contact's own flag. Re-asking the
// repository by external_id missed a person keyed by their other address —
// opted out by e-mail, keyed by phone — and let the draft through.
func TestApproach_Draft_OptedOutIsReadFromTheResolvedContact(t *testing.T) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	contactRepo := new(MockContactRepo)
	sender := new(mockApproachSender)
	uc := NewApproachUseCase(leadRepo, funnelRepo, new(MockChannelRepo), contactRepo,
		new(MockAccountRepo), new(MockPlanRepo), new(MockProductRepo), new(MockUsageRepo),
		new(MockAIFactory), sender)

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "ct-1", Status: domain.LeadStatusEnriched}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1"}, nil)
	optedOut := time.Now()
	contactRepo.On("GetByID", mock.Anything, "ct-1", "acc1").
		Return(&domain.Contact{ID: "ct-1", AccountID: "acc1", ExternalID: "556536188000", OptedOutAt: &optedOut}, nil)

	_, err := uc.Draft(context.Background(), "acc1", "lead1")
	assert.ErrorIs(t, err, domain.ErrContactOptedOut)
	contactRepo.AssertNotCalled(t, "IsOptedOut", mock.Anything, mock.Anything, mock.Anything)
	sender.AssertNotCalled(t, "SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A door is not an address. An account with a connected mailbox and a contact
// carrying only a phone can send — just not to this person — and drafting for
// them produces a finished letter with nowhere to post it.
//
// Production, 2026-09-10: 70 contacts, 52 with a phone, ZERO with an e-mail,
// 50 finished offers in the queue and not one that could leave. The
// account-level gate passed the whole time, because the account really did
// have a mailbox.
func TestApproach_Draft_RefusesAContactNoChannelCanReach(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, contactRepo, accountRepo, aiFactory, provider, sender := approachUCWithContacts()

	channelRepo.On("ListByAccountID", mock.Anything, "acc1").
		Return([]*domain.Channel{
			{ID: "ch-mail", AccountID: "acc1", Type: "email", IsActive: true},
		}, nil)
	uc.SetSendChannels(channelRepo)

	phoneOnly := "+5565988887777"
	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "ct1", Status: domain.LeadStatusEnriched}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", Name: "Vendas"}, nil)
	contactRepo.On("GetByID", mock.Anything, "ct1", "acc1").
		Return(&domain.Contact{ID: "ct1", AccountID: "acc1", Phone: &phoneOnly}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", mock.Anything).Return(false, nil).Maybe()
	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1"}, nil)

	_, err := uc.Draft(context.Background(), "acc1", "lead1")
	assert.ErrorIs(t, err, domain.ErrApproachNeedsAddress)
	// The point of refusing here is that nothing is spent writing it.
	aiFactory.AssertNotCalled(t, "GetProvider", mock.Anything, mock.Anything)
	provider.AssertNotCalled(t, "GenerateResponse", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	sender.AssertNotCalled(t, "SendMessageAndGetID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The same contact, with a WhatsApp channel connected, is perfectly
// reachable. The gate is the CROSSING of address and channel, never either
// one alone — otherwise it would block the account this product is for.
func TestApproach_Draft_APhoneIsAnAddressWhenWhatsAppIsConnected(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, contactRepo, accountRepo, aiFactory, _, _ := approachUCWithContacts()

	channelRepo.On("ListByAccountID", mock.Anything, "acc1").
		Return([]*domain.Channel{
			{ID: "ch-wa", AccountID: "acc1", Type: "whatsapp", IsActive: true},
		}, nil)
	uc.SetSendChannels(channelRepo)

	phoneOnly := "+5565988887777"
	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "ct1", Status: domain.LeadStatusEnriched}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", Name: "Vendas"}, nil)
	contactRepo.On("GetByID", mock.Anything, "ct1", "acc1").
		Return(&domain.Contact{ID: "ct1", AccountID: "acc1", Phone: &phoneOnly}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", mock.Anything).Return(false, nil).Maybe()
	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1"}, nil)
	aiFactory.On("GetProvider", mock.Anything, mock.Anything).
		Return(nil, errors.New("no provider in this harness")).Maybe()

	_, err := uc.Draft(context.Background(), "acc1", "lead1")
	assert.NotErrorIs(t, err, domain.ErrApproachNeedsAddress,
		"a phone plus a WhatsApp channel is a deliverable envelope")
}

// A contact that does not resolve is a lookup problem, not a proven gap. The
// other gates here all fail open on a read error and so must this one, or a
// database blip stops a working account from drafting anything.
func TestApproach_Draft_AnUnresolvedContactFailsOpen(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, contactRepo, accountRepo, aiFactory, _, _ := approachUCWithContacts()

	channelRepo.On("ListByAccountID", mock.Anything, "acc1").
		Return([]*domain.Channel{
			{ID: "ch-mail", AccountID: "acc1", Type: "email", IsActive: true},
		}, nil)
	uc.SetSendChannels(channelRepo)

	leadRepo.On("GetByID", mock.Anything, "lead1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1", ContactID: "ct1", Status: domain.LeadStatusEnriched}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{ID: "f1", AccountID: "acc1", Name: "Vendas"}, nil)
	contactRepo.On("GetByID", mock.Anything, "ct1", "acc1").
		Return((*domain.Contact)(nil), errors.New("db blip"))
	contactRepo.On("GetByExternalID", mock.Anything, "acc1", "ct1").
		Return((*domain.Contact)(nil), errors.New("db blip"))
	contactRepo.On("IsOptedOut", mock.Anything, "acc1", mock.Anything).Return(false, nil).Maybe()
	accountRepo.On("GetByID", mock.Anything, "acc1").Return(&domain.Account{ID: "acc1"}, nil)
	aiFactory.On("GetProvider", mock.Anything, mock.Anything).
		Return(nil, errors.New("no provider in this harness")).Maybe()

	_, err := uc.Draft(context.Background(), "acc1", "lead1")
	assert.NotErrorIs(t, err, domain.ErrApproachNeedsAddress,
		"an unreadable contact is not a contact proven unreachable")
}

// approachUCWithContacts is newApproachUC without its blanket "every contact
// lookup returns nothing" stub.
//
// That stub exists because most fixtures here use the legacy shape where the
// lead's ContactID IS the phone, so the id lookup legitimately misses. testify
// matches the FIRST registered expectation, so a test that needs a real
// contact cannot register one on that repo — it would be shadowed. The
// reachability gate is entirely about what is on the contact, so it needs one.
func approachUCWithContacts() (
	*ApproachUseCase,
	*mockLeadRepo, *MockSalesFunnelRepo, *MockChannelRepo, *MockContactRepo,
	*MockAccountRepo, *MockAIFactory, *MockAIProvider, *mockApproachSender,
) {
	leadRepo := new(mockLeadRepo)
	funnelRepo := new(MockSalesFunnelRepo)
	channelRepo := new(MockChannelRepo)
	contactRepo := new(MockContactRepo)
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	productRepo := new(MockProductRepo)
	productRepo.On("ListByAccountID", mock.Anything, mock.Anything).
		Return([]*domain.Product{{ID: "prod1", AccountID: "acc1", Name: "Tomógrafo"}}, nil).Maybe()
	usageRepo := new(MockUsageRepo)
	aiFactory := new(MockAIFactory)
	provider := new(MockAIProvider)
	sender := new(mockApproachSender)

	uc := NewApproachUseCase(leadRepo, funnelRepo, channelRepo, contactRepo,
		accountRepo, planRepo, productRepo, usageRepo, aiFactory, sender)
	return uc, leadRepo, funnelRepo, channelRepo, contactRepo, accountRepo,
		aiFactory, provider, sender
}
