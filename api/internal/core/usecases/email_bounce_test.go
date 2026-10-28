package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// The job in one line: a permanently dead address must stop receiving mail,
// and a bad day must not be mistaken for one.

type fakeSuppressions struct {
	suppressed []*domain.EmailSuppression
	err        error
	is         bool
	isErr      error
}

func (f *fakeSuppressions) Suppress(_ context.Context, s *domain.EmailSuppression) error {
	if f.err != nil {
		return f.err
	}
	f.suppressed = append(f.suppressed, s)
	return nil
}

func (f *fakeSuppressions) IsSuppressed(_ context.Context, _, _ string) (bool, error) {
	return f.is, f.isErr
}

func (f *fakeSuppressions) List(_ context.Context, _ string, _, _ int) ([]*domain.EmailSuppression, error) {
	return f.suppressed, nil
}

func (f *fakeSuppressions) Release(_ context.Context, _, _ string) error { return nil }

type fakeLastOutbound struct {
	msg *domain.Message
	err error
}

func (f *fakeLastOutbound) LastOutboundToContact(_ context.Context, _, _ string) (*domain.Message, error) {
	return f.msg, f.err
}

type fakeBounceStats struct {
	domain.EmailTemplateRepository
	bounces []string
	version int
}

func (f *fakeBounceStats) RecordBounce(_ context.Context, id string, version int) error {
	f.bounces = append(f.bounces, id)
	f.version = version
	return nil
}

func bounceChannel() *domain.Channel {
	return &domain.Channel{ID: "chan-email", AccountID: "acc-1", Type: "email"}
}

func sentMessage() *domain.Message {
	return &domain.Message{
		ID:        "msg-sent",
		Direction: "outbound",
		Metadata: json.RawMessage(
			`{"subject":"Proposta","template_id":"tpl-frio","template_version":2}`),
	}
}

func bounceUC(t *testing.T) (*EmailBounceUseCase, *fakeSuppressions, *MockMessageRepo, *fakeBounceStats) {
	t.Helper()
	sup := &fakeSuppressions{}
	msgRepo := new(MockMessageRepo)
	msgRepo.On("MarkSendFailed", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	stats := &fakeBounceStats{}

	uc := NewEmailBounceUseCase(sup, msgRepo, &fakeLastOutbound{msg: sentMessage()})
	uc.SetTemplateStats(stats)
	return uc, sup, msgRepo, stats
}

func TestBounce_HardSuppressesTheAddress(t *testing.T) {
	uc, sup, _, _ := bounceUC(t)

	uc.Record(context.Background(), bounceChannel(), domain.EmailBounce{
		Kind:       domain.BounceHard,
		Recipient:  "dono@clinicafechada.com.br",
		Diagnostic: "550 5.1.1 User unknown",
	})

	require.Len(t, sup.suppressed, 1)
	assert.Equal(t, "dono@clinicafechada.com.br", sup.suppressed[0].Address)
	assert.Equal(t, domain.SuppressionHardBounce, sup.suppressed[0].Reason)
	// The remote server's own words, kept verbatim: it is the answer to
	// "why can't I email them?".
	assert.Equal(t, "550 5.1.1 User unknown", sup.suppressed[0].Diagnostic)
}

// The one that protects real customers: a full mailbox is emptied, a
// greylisted server accepts the retry. Suppressing here would silently
// delete a working address from the list.
func TestBounce_SoftNeverSuppresses(t *testing.T) {
	uc, sup, _, stats := bounceUC(t)

	uc.Record(context.Background(), bounceChannel(), domain.EmailBounce{
		Kind:       domain.BounceSoft,
		Recipient:  "dono@clinica.com.br",
		Diagnostic: "452 mailbox full",
	})

	assert.Empty(t, sup.suppressed)
	// Still counted: a template whose recipients keep deferring is still a
	// signal about the list.
	assert.Equal(t, []string{"tpl-frio"}, stats.bounces)
}

func TestBounce_ComplaintIsSuppressedUnderItsOwnReason(t *testing.T) {
	uc, sup, _, _ := bounceUC(t)

	uc.Record(context.Background(), bounceChannel(), domain.EmailBounce{
		Kind:      domain.BounceComplaint,
		Recipient: "dono@clinica.com.br",
	})

	require.Len(t, sup.suppressed, 1)
	// Not folded into hard_bounce: the address WORKS, and the person told
	// their provider we are abusive. Different problem, different answer.
	assert.Equal(t, domain.SuppressionComplaint, sup.suppressed[0].Reason)
}

func TestBounce_AddressIsStoredLowercased(t *testing.T) {
	uc, sup, _, _ := bounceUC(t)

	uc.Record(context.Background(), bounceChannel(), domain.EmailBounce{
		Kind:      domain.BounceHard,
		Recipient: "  Dono@Clinica.COM.br  ",
	})

	require.Len(t, sup.suppressed, 1)
	assert.Equal(t, "dono@clinica.com.br", sup.suppressed[0].Address)
}

func TestBounce_CountsAgainstTheTemplateVersion(t *testing.T) {
	uc, _, _, stats := bounceUC(t)

	uc.Record(context.Background(), bounceChannel(), domain.EmailBounce{
		Kind: domain.BounceHard, Recipient: "dono@clinica.com.br",
	})

	assert.Equal(t, []string{"tpl-frio"}, stats.bounces)
	assert.Equal(t, 2, stats.version)
}

// The salesperson is looking at the thread waiting for a reply. The bubble
// they are looking at has to say it never arrived.
func TestBounce_MarksTheSentMessageUndelivered(t *testing.T) {
	uc, _, msgRepo, _ := bounceUC(t)

	uc.Record(context.Background(), bounceChannel(), domain.EmailBounce{
		Kind: domain.BounceHard, Recipient: "dono@clinica.com.br",
		Diagnostic: "550 5.1.1 User unknown",
	})

	msgRepo.AssertCalled(t, "MarkSendFailed", mock.Anything, "msg-sent",
		"550 5.1.1 User unknown")
}

// An empty tooltip on a red marker reads as a UI bug, not as an answer.
func TestBounce_UndeliveredReasonIsNeverBlank(t *testing.T) {
	uc, _, msgRepo, _ := bounceUC(t)

	uc.Record(context.Background(), bounceChannel(), domain.EmailBounce{
		Kind: domain.BounceHard, Recipient: "dono@clinica.com.br",
	})

	msgRepo.AssertCalled(t, "MarkSendFailed", mock.Anything, "msg-sent", "hard bounce")
}

// A bounce for something sent by hand has no template to charge, and must
// still suppress the address.
func TestBounce_NoTemplateStillSuppresses(t *testing.T) {
	sup := &fakeSuppressions{}
	msgRepo := new(MockMessageRepo)
	msgRepo.On("MarkSendFailed", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	stats := &fakeBounceStats{}
	uc := NewEmailBounceUseCase(sup, msgRepo, &fakeLastOutbound{
		msg: &domain.Message{ID: "msg-sent", Direction: "outbound",
			Metadata: json.RawMessage(`{"subject":"Oi"}`)},
	})
	uc.SetTemplateStats(stats)

	uc.Record(context.Background(), bounceChannel(), domain.EmailBounce{
		Kind: domain.BounceHard, Recipient: "dono@clinica.com.br",
	})

	assert.Len(t, sup.suppressed, 1)
	assert.Empty(t, stats.bounces)
}

// Nothing found to charge is an ordinary case (a message sent from the
// salesperson's own mailbox), not a reason to skip the suppression.
func TestBounce_UnknownOriginalStillSuppresses(t *testing.T) {
	sup := &fakeSuppressions{}
	msgRepo := new(MockMessageRepo)
	uc := NewEmailBounceUseCase(sup, msgRepo, &fakeLastOutbound{msg: nil})
	uc.SetTemplateStats(&fakeBounceStats{})

	uc.Record(context.Background(), bounceChannel(), domain.EmailBounce{
		Kind: domain.BounceHard, Recipient: "dono@clinica.com.br",
	})

	assert.Len(t, sup.suppressed, 1)
	msgRepo.AssertNotCalled(t, "MarkSendFailed", mock.Anything, mock.Anything, mock.Anything)
}

// A failed suppression is the one failure that lets the send keep happening,
// so it must not take the rest of the handling down with it.
func TestBounce_SuppressionFailureDoesNotStopTheRest(t *testing.T) {
	sup := &fakeSuppressions{err: errors.New("db down")}
	msgRepo := new(MockMessageRepo)
	msgRepo.On("MarkSendFailed", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	stats := &fakeBounceStats{}
	uc := NewEmailBounceUseCase(sup, msgRepo, &fakeLastOutbound{msg: sentMessage()})
	uc.SetTemplateStats(stats)

	uc.Record(context.Background(), bounceChannel(), domain.EmailBounce{
		Kind: domain.BounceHard, Recipient: "dono@clinica.com.br",
	})

	assert.Equal(t, []string{"tpl-frio"}, stats.bounces)
}

// A bounce naming nobody cannot suppress anything.
func TestBounce_WithoutARecipientDoesNothing(t *testing.T) {
	uc, sup, msgRepo, stats := bounceUC(t)

	uc.Record(context.Background(), bounceChannel(), domain.EmailBounce{
		Kind: domain.BounceHard,
	})

	assert.Empty(t, sup.suppressed)
	assert.Empty(t, stats.bounces)
	msgRepo.AssertNotCalled(t, "MarkSendFailed", mock.Anything, mock.Anything, mock.Anything)
}

// ── The send-side gate ────────────────────────────────────────────────────

// Sending to a suppressed address is the thing that costs the customer their
// sending reputation, so the refusal happens before any provider call.
func TestApproveSend_RefusesASuppressedAddress(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, contactRepo, _, _, _, _, _, _, sender := newApproachUC()
	uc.SetEmailSender(sender)
	uc.SetSuppressionRepo(&fakeSuppressions{is: true})

	const addr = "dono@clinicafechada.com.br"
	leadRepo.On("GetByID", mock.Anything, "lead1").Return(&domain.Lead{
		ID: "lead1", AccountID: "acc1", FunnelID: "f1",
		ContactID: addr, Status: domain.LeadStatusPendingApproval,
	}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").Return(&domain.SalesFunnel{
		ID: "f1", AccountID: "acc1", ChannelID: strPtr("ch1"),
	}, nil)
	channelRepo.On("GetByID", mock.Anything, "ch1").Return(&domain.Channel{
		ID: "ch1", AccountID: "acc1", Type: "email",
		Config: map[string]any{"from_address": "comercial@clinicasaolucas.com.br"},
	}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Maybe()

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead1",
		ApproachSendInput{Text: "Bom dia!", Subject: "Proposta"})

	require.Error(t, err)
	assert.Equal(t, domain.ErrEmailAddressSuppressed.Code, err.(*domain.AppError).Code)
	sender.AssertNotCalled(t, "SendMessageAndGetID",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Fails CLOSED, and this is the opposite of the opt-out guard on purpose:
// there the inbound keyword is the durable enforcement and the send-side
// check is belt-and-braces, so it fails open. A suppression has no second
// enforcement point, so a lookup we cannot perform is not permission.
func TestApproveSend_SuppressionLookupFailureRefusesTheSend(t *testing.T) {
	uc, leadRepo, funnelRepo, channelRepo, contactRepo, _, _, _, _, _, _, sender := newApproachUC()
	uc.SetEmailSender(sender)
	uc.SetSuppressionRepo(&fakeSuppressions{isErr: errors.New("db down")})

	const addr = "dono@clinica.com.br"
	leadRepo.On("GetByID", mock.Anything, "lead1").Return(&domain.Lead{
		ID: "lead1", AccountID: "acc1", FunnelID: "f1",
		ContactID: addr, Status: domain.LeadStatusPendingApproval,
	}, nil)
	funnelRepo.On("GetByID", mock.Anything, "f1").Return(&domain.SalesFunnel{
		ID: "f1", AccountID: "acc1", ChannelID: strPtr("ch1"),
	}, nil)
	channelRepo.On("GetByID", mock.Anything, "ch1").Return(&domain.Channel{
		ID: "ch1", AccountID: "acc1", Type: "email",
		Config: map[string]any{"from_address": "comercial@clinicasaolucas.com.br"},
	}, nil)
	contactRepo.On("IsOptedOut", mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Maybe()

	_, err := uc.ApproveSend(context.Background(), "acc1", "lead1",
		ApproachSendInput{Text: "Bom dia!", Subject: "Proposta"})

	require.Error(t, err)
	sender.AssertNotCalled(t, "SendMessageAndGetID",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// ── Taking the lead out of the pipeline ───────────────────────────────────

// leads.contact_id is a contact ULID, and the bounce carries an ADDRESS. The
// lookup used to run on the address directly, so it never matched — and the
// funnel was keyed on the channel, which the default funnel does not have.
// Two silent misses; a dead address stayed "aguardando resposta" forever.
func TestBounce_HardMarksTheLeadLostByContactOnTheDefaultFunnel(t *testing.T) {
	uc, _, _, _ := bounceUC(t)

	funnelRepo := new(MockSalesFunnelRepo)
	funnelRepo.On("GetByChannelID", mock.Anything, "chan-email").Return(nil, nil)
	funnelRepo.On("EnsureDefault", mock.Anything, "acc-1").Return(&domain.SalesFunnel{ID: "fun-default"}, nil)
	leadRepo := new(mockLeadRepo)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fun-default", "contact-dono").
		Return(&domain.Lead{ID: "lead-dono", Status: domain.LeadStatusContacted}, nil)
	leadRepo.On("UpdateStatus", mock.Anything, "lead-dono", domain.LeadStatusLost).Return(nil)
	contactRepo := new(MockContactRepo)
	contactRepo.On("GetByExternalID", mock.Anything, "acc-1", "dono@clinicafechada.com.br").
		Return(&domain.Contact{ID: "contact-dono"}, nil)
	uc.SetSalesLead(funnelRepo, leadRepo)
	uc.SetContactRepo(contactRepo)

	uc.Record(context.Background(), bounceChannel(), domain.EmailBounce{
		Kind: domain.BounceHard, Recipient: "Dono@ClinicaFechada.com.br",
	})

	leadRepo.AssertCalled(t, "UpdateStatus", mock.Anything, "lead-dono", domain.LeadStatusLost)
	// Resolved through the contact: the raw address was never used as a key.
	leadRepo.AssertNotCalled(t, "GetByFunnelAndContact", mock.Anything, "fun-default", "dono@clinicafechada.com.br")
}

// A row written before the identity migration carries the address itself in
// contact_id. Still found, through the legacy second try.
func TestBounce_LegacyLeadKeyedByAddressStillGoesLost(t *testing.T) {
	uc, _, _, _ := bounceUC(t)

	funnelRepo := new(MockSalesFunnelRepo)
	funnelRepo.On("GetByChannelID", mock.Anything, "chan-email").Return(&domain.SalesFunnel{ID: "fun-1"}, nil)
	leadRepo := new(mockLeadRepo)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fun-1", "dono@clinica.com.br").
		Return(&domain.Lead{ID: "lead-legacy", Status: domain.LeadStatusContacted}, nil)
	leadRepo.On("UpdateStatus", mock.Anything, "lead-legacy", domain.LeadStatusLost).Return(nil)
	contactRepo := new(MockContactRepo)
	contactRepo.On("GetByExternalID", mock.Anything, "acc-1", "dono@clinica.com.br").
		Return(nil, errors.New("no rows"))
	uc.SetSalesLead(funnelRepo, leadRepo)
	uc.SetContactRepo(contactRepo)

	uc.Record(context.Background(), bounceChannel(), domain.EmailBounce{
		Kind: domain.BounceHard, Recipient: "dono@clinica.com.br",
	})

	leadRepo.AssertCalled(t, "UpdateStatus", mock.Anything, "lead-legacy", domain.LeadStatusLost)
}

// A customer who bought and whose address later died is still won.
func TestBounce_AWonLeadIsNeverMarkedLost(t *testing.T) {
	uc, _, _, _ := bounceUC(t)

	funnelRepo := new(MockSalesFunnelRepo)
	funnelRepo.On("GetByChannelID", mock.Anything, "chan-email").Return(nil, nil)
	funnelRepo.On("EnsureDefault", mock.Anything, "acc-1").Return(&domain.SalesFunnel{ID: "fun-default"}, nil)
	leadRepo := new(mockLeadRepo)
	leadRepo.On("GetByFunnelAndContact", mock.Anything, "fun-default", "contact-won").
		Return(&domain.Lead{ID: "lead-won", Status: domain.LeadStatusWon}, nil)
	contactRepo := new(MockContactRepo)
	contactRepo.On("GetByExternalID", mock.Anything, "acc-1", "cliente@hospital.com.br").
		Return(&domain.Contact{ID: "contact-won"}, nil)
	uc.SetSalesLead(funnelRepo, leadRepo)
	uc.SetContactRepo(contactRepo)

	uc.Record(context.Background(), bounceChannel(), domain.EmailBounce{
		Kind: domain.BounceHard, Recipient: "cliente@hospital.com.br",
	})

	leadRepo.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
}
