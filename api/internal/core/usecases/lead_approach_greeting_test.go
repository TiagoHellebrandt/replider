package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// The one exception to human-in-the-loop (§6.5, F4): a greeting on an account
// running autopilot. Everything that narrows it is tested here, because the
// blast radius of getting it wrong is unattended mail sent in a customer's
// name.

type fakeAutopilotRepo struct {
	settings  *domain.Autopilot
	sentToday int
	recorded  []*domain.AutopilotSend
}

func (f *fakeAutopilotRepo) Get(context.Context, string) (*domain.Autopilot, error) {
	return f.settings, nil
}
func (f *fakeAutopilotRepo) Save(context.Context, *domain.Autopilot) error        { return nil }
func (f *fakeAutopilotRepo) RecordApproval(context.Context, string, bool) error   { return nil }
func (f *fakeAutopilotRepo) MarkOffered(context.Context, string, time.Time) error { return nil }
func (f *fakeAutopilotRepo) CountSentToday(context.Context, string, time.Time) (int, error) {
	return f.sentToday, nil
}
func (f *fakeAutopilotRepo) RecordSend(_ context.Context, e *domain.AutopilotSend) error {
	f.recorded = append(f.recorded, e)
	return nil
}
func (f *fakeAutopilotRepo) ListSends(context.Context, string, int, int) ([]*domain.AutopilotSend, error) {
	return nil, nil
}

// greetingFixture wires an account with ONE active funnel on a channel of the
// given type, and no existing lead for the contact.
func greetingFixture(t *testing.T, channelType string, pilot *fakeAutopilotRepo) (
	*ApproachUseCase, *mockLeadRepo, *mockApproachSender,
) {
	t.Helper()
	uc, leadRepo, funnelRepo, channelRepo, contactRepo, accountRepo,
		_, _, _, _, _, sender := newApproachUC()

	channelID := "ch1"
	funnelRepo.On("ListByAccountID", mock.Anything, "acc1").
		Return([]*domain.SalesFunnel{{
			ID: "f1", AccountID: "acc1", Active: true, ChannelID: &channelID,
		}}, nil).Maybe()
	funnelRepo.On("GetByID", mock.Anything, "f1").
		Return(&domain.SalesFunnel{
			ID: "f1", AccountID: "acc1", Active: true, ChannelID: &channelID,
		}, nil).Maybe()
	channelRepo.On("GetByID", mock.Anything, channelID).
		Return(&domain.Channel{ID: channelID, AccountID: "acc1", Type: channelType}, nil).Maybe()
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "f1", "c1").Return(nil, nil).Maybe()
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil).Maybe()
	contactRepo.On("IsOptedOut", mock.Anything, mock.Anything, mock.Anything).
		Return(false, nil).Maybe()
	accountRepo.On("GetByID", mock.Anything, "acc1").
		Return(&domain.Account{ID: "acc1", Timezone: "America/Sao_Paulo"}, nil).Maybe()

	if pilot != nil {
		uc.SetAutopilotRepo(pilot)
	}
	return uc, leadRepo, sender
}

// The queue is the default. Without autopilot, a birthday greeting waits for a
// person like everything else.
func TestGreeting_QueuesWhenAutopilotIsOff(t *testing.T) {
	pilot := &fakeAutopilotRepo{settings: &domain.Autopilot{Enabled: false}}
	uc, leadRepo, sender := greetingFixture(t, "email", pilot)

	err := uc.QueueRelationshipApproach(context.Background(), "acc1", "c1",
		"relationship", "Feliz aniversário!", "Parabéns!", true)

	require.NoError(t, err)
	leadRepo.AssertCalled(t, "Upsert", mock.Anything, mock.Anything)
	sender.AssertNotCalled(t, "SendMessageAndGetID",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	assert.Empty(t, pilot.recorded)
}

// The rule that protects a WhatsApp number from Meta does not become
// negotiable because the message is friendly.
func TestGreeting_NeverAutoSendsOnWhatsApp(t *testing.T) {
	pilot := &fakeAutopilotRepo{settings: &domain.Autopilot{Enabled: true}}
	uc, _, sender := greetingFixture(t, "whatsapp", pilot)

	err := uc.QueueRelationshipApproach(context.Background(), "acc1", "c1",
		"relationship", "Feliz aniversário!", "Parabéns!", true)

	require.NoError(t, err)
	sender.AssertNotCalled(t, "SendMessageAndGetID",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	assert.Empty(t, pilot.recorded)
}

// A maintenance reminder implies a service the company may not be able to
// schedule; a check-in is a sales message wearing a friendly hat.
func TestGreeting_NotLowRiskNeverAutoSends(t *testing.T) {
	pilot := &fakeAutopilotRepo{settings: &domain.Autopilot{Enabled: true}}
	uc, _, sender := greetingFixture(t, "email", pilot)

	err := uc.QueueRelationshipApproach(context.Background(), "acc1", "c1",
		"check_in", "Tudo bem?", "Faz um tempo...", false)

	require.NoError(t, err)
	sender.AssertNotCalled(t, "SendMessageAndGetID",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	assert.Empty(t, pilot.recorded)
}

// The cap is a promise to the operator: no more than N while I am away. It is
// shared with cold outbound on purpose — a per-category cap adds up to no cap.
func TestGreeting_RespectsTheDailyCap(t *testing.T) {
	pilot := &fakeAutopilotRepo{
		settings:  &domain.Autopilot{Enabled: true, DailyCap: 5},
		sentToday: 5,
	}
	uc, _, sender := greetingFixture(t, "email", pilot)

	err := uc.QueueRelationshipApproach(context.Background(), "acc1", "c1",
		"relationship", "Feliz aniversário!", "Parabéns!", true)

	require.NoError(t, err)
	sender.AssertNotCalled(t, "SendMessageAndGetID",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// An unwired autopilot repository means the feature does not exist for this
// deployment — the draft simply queues.
func TestGreeting_UnwiredAutopilotJustQueues(t *testing.T) {
	uc, leadRepo, sender := greetingFixture(t, "email", nil)

	err := uc.QueueRelationshipApproach(context.Background(), "acc1", "c1",
		"relationship", "Feliz aniversário!", "Parabéns!", true)

	require.NoError(t, err)
	leadRepo.AssertCalled(t, "Upsert", mock.Anything, mock.Anything)
	sender.AssertNotCalled(t, "SendMessageAndGetID",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Adding a draft on top of a live negotiation puts two conversations about the
// same customer in front of the same salesperson.
func TestGreeting_LeavesALeadThatIsAlreadyBeingWorked(t *testing.T) {
	pilot := &fakeAutopilotRepo{settings: &domain.Autopilot{Enabled: true}}
	uc, leadRepo, sender := greetingFixture(t, "email", pilot)
	// Replace the "no existing lead" expectation with one mid-conversation.
	leadRepo.ExpectedCalls = nil
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "f1", "c1").
		Return(&domain.Lead{ID: "lead1", AccountID: "acc1", FunnelID: "f1",
			ContactID: "c1", Status: domain.LeadStatusResponded}, nil)

	err := uc.QueueRelationshipApproach(context.Background(), "acc1", "c1",
		"relationship", "Feliz aniversário!", "Parabéns!", true)

	require.NoError(t, err)
	leadRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
	sender.AssertNotCalled(t, "SendMessageAndGetID",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// An account with sales and no funnel is a setup problem somebody can fix —
// reported, not silently dropped.
func TestGreeting_NoFunnelIsReported(t *testing.T) {
	uc, _, funnelRepo, _, _, _, _, _, _, _, _, _ := newApproachUC()
	funnelRepo.On("ListByAccountID", mock.Anything, "acc1").
		Return([]*domain.SalesFunnel{}, nil)

	err := uc.QueueRelationshipApproach(context.Background(), "acc1", "c1",
		"relationship", "Feliz aniversário!", "Parabéns!", true)

	assert.ErrorIs(t, err, domain.ErrSalesFunnelNotFound)
}

// The positive case, and the one that makes every refusal above meaningful:
// Even with autopilot ON, an email channel and a low-risk kind, the greeting
// does NOT go out by itself. This was the one path in the product that wrote
// to a customer without a person approving it (removed 2026-09-01): the
// suggestion lands in the queue and waits like everything else.
func TestGreeting_NeverAutoSendsEvenWithAutopilotOn(t *testing.T) {
	warmed := time.Now().Add(-60 * 24 * time.Hour)
	uc, repo, sender := emailApprovalFixture(t, mailboxAt(true, &warmed, 25, 0))

	channelID := "ch-email"
	funnelRepo := new(MockSalesFunnelRepo)
	funnelRepo.On("ListByAccountID", mock.Anything, "acc1").
		Return([]*domain.SalesFunnel{{
			ID: "f1", AccountID: "acc1", Active: true, ChannelID: &channelID,
		}}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").Return(&domain.SalesFunnel{
		ID: "f1", AccountID: "acc1", Active: true, ChannelID: &channelID,
	}, nil)
	uc.funnelRepo = funnelRepo

	leadRepo := new(mockLeadRepo)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "f1", "compras@hospital.com.br").
		Return(nil, nil)
	leadRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)
	uc.leadRepo = leadRepo

	pilot := &fakeAutopilotRepo{settings: &domain.Autopilot{Enabled: true, DailyCap: 20}}
	uc.SetAutopilotRepo(pilot)

	err := uc.QueueRelationshipApproach(context.Background(), "acc1",
		"compras@hospital.com.br", "relationship",
		"Feliz aniversário!", "Parabéns! Um ótimo dia.", true)

	require.NoError(t, err)
	leadRepo.AssertCalled(t, "Upsert", mock.Anything, mock.Anything)
	assert.Equal(t, 0, sender.sends, "nothing leaves without a person approving it")
	assert.Equal(t, 0, repo.recorded)
	assert.Empty(t, pilot.recorded)
}
