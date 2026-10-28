package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

type MockSalesLeadRepo struct {
	createErr      error
	created        *domain.SalesLead
	existsRecent   bool
	existsRecented bool
	existsErr      error
}

func (m *MockSalesLeadRepo) Create(ctx context.Context, l *domain.SalesLead) error {
	m.created = l
	return m.createErr
}

func (m *MockSalesLeadRepo) ExistsRecent(ctx context.Context, email string, since time.Time) (bool, error) {
	m.existsRecented = true
	return m.existsRecent, m.existsErr
}

type MockSalesNotifier struct {
	to     string
	lead   domain.SalesLead
	called bool
	err    error
}

func (m *MockSalesNotifier) SendSalesLeadEmail(to string, lead domain.SalesLead) error {
	m.called = true
	m.to = to
	m.lead = lead
	return m.err
}

func validSalesInput() SubmitSalesLeadInput {
	return SubmitSalesLeadInput{
		Company:       "Acme Ltda",
		Email:         "buyer@acme.com",
		CompanySize:   "201-500",
		Channels:      "WhatsApp, Telegram",
		MonthlyVolume: "10k-50k",
		Message:       "Quero migrar do concorrente.",
	}
}

func TestSubmitSalesLead_Success_PersistsAndNotifies(t *testing.T) {
	repo := &MockSalesLeadRepo{}
	notifier := &MockSalesNotifier{}
	uc := NewSubmitSalesLeadUseCase(repo, notifier, "sales@replider.com")

	err := uc.Execute(context.Background(), validSalesInput())
	assert.NoError(t, err)
	assert.NotNil(t, repo.created)
	assert.Equal(t, "Acme Ltda", repo.created.Company)
	assert.NotEmpty(t, repo.created.ID)
	assert.True(t, notifier.called)
	assert.Equal(t, "sales@replider.com", notifier.to)
	assert.Equal(t, "buyer@acme.com", notifier.lead.Email)
}

func TestSubmitSalesLead_NormalizesEmail(t *testing.T) {
	repo := &MockSalesLeadRepo{}
	uc := NewSubmitSalesLeadUseCase(repo, &MockSalesNotifier{}, "sales@replider.com")

	in := validSalesInput()
	in.Email = "  BUYER@ACME.COM  "
	err := uc.Execute(context.Background(), in)
	assert.NoError(t, err)
	assert.Equal(t, "buyer@acme.com", repo.created.Email)
}

func TestSubmitSalesLead_Honeypot_BlocksBot(t *testing.T) {
	repo := &MockSalesLeadRepo{}
	notifier := &MockSalesNotifier{}
	uc := NewSubmitSalesLeadUseCase(repo, notifier, "sales@replider.com")

	in := validSalesInput()
	in.Honeypot = "http://spam.example"
	err := uc.Execute(context.Background(), in)
	assert.Equal(t, domain.ErrSpamDetected, err)
	assert.Nil(t, repo.created, "honeypot submission must not be persisted")
	assert.False(t, repo.existsRecented, "honeypot must short-circuit before rate-limit check")
	assert.False(t, notifier.called)
}

func TestSubmitSalesLead_InvalidEmail(t *testing.T) {
	repo := &MockSalesLeadRepo{}
	uc := NewSubmitSalesLeadUseCase(repo, &MockSalesNotifier{}, "sales@replider.com")

	in := validSalesInput()
	in.Email = "notanemail"
	err := uc.Execute(context.Background(), in)
	assert.Equal(t, domain.ErrInvalidEmail, err)
	assert.Nil(t, repo.created)
}

func TestSubmitSalesLead_MissingFields(t *testing.T) {
	uc := NewSubmitSalesLeadUseCase(&MockSalesLeadRepo{}, &MockSalesNotifier{}, "sales@replider.com")

	cases := []func(in *SubmitSalesLeadInput){
		func(in *SubmitSalesLeadInput) { in.Company = "  " },
		func(in *SubmitSalesLeadInput) { in.CompanySize = "" },
		func(in *SubmitSalesLeadInput) { in.Channels = "" },
		func(in *SubmitSalesLeadInput) { in.MonthlyVolume = "" },
	}
	for _, mut := range cases {
		in := validSalesInput()
		mut(&in)
		err := uc.Execute(context.Background(), in)
		assert.Equal(t, domain.ErrMissingSalesFields, err)
	}
}

func TestSubmitSalesLead_RateLimit_BlocksRepeat(t *testing.T) {
	repo := &MockSalesLeadRepo{existsRecent: true}
	notifier := &MockSalesNotifier{}
	uc := NewSubmitSalesLeadUseCase(repo, notifier, "sales@replider.com")

	err := uc.Execute(context.Background(), validSalesInput())
	assert.Equal(t, domain.ErrTooManyRequests, err)
	assert.True(t, repo.existsRecented)
	assert.Nil(t, repo.created)
	assert.False(t, notifier.called)
}

func TestSubmitSalesLead_EmailFailure_DoesNotFailRequest(t *testing.T) {
	repo := &MockSalesLeadRepo{}
	notifier := &MockSalesNotifier{err: errors.New("smtp down")}
	uc := NewSubmitSalesLeadUseCase(repo, notifier, "sales@replider.com")

	// A failed notification is logged but must not surface — the persisted
	// lead is the durable record.
	err := uc.Execute(context.Background(), validSalesInput())
	assert.NoError(t, err)
	assert.NotNil(t, repo.created)
	assert.True(t, notifier.called)
}

func TestSubmitSalesLead_NilNotifier_StillPersists(t *testing.T) {
	repo := &MockSalesLeadRepo{}
	uc := NewSubmitSalesLeadUseCase(repo, nil, "")

	err := uc.Execute(context.Background(), validSalesInput())
	assert.NoError(t, err)
	assert.NotNil(t, repo.created)
}
