package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

type MockROILeadRepo struct {
	createErr      error
	created        bool
	createdLead    *domain.ROILead
	existsRecent   bool
	existsRecented bool // whether ExistsRecent was consulted
	existsErr      error
}

func (m *MockROILeadRepo) Create(ctx context.Context, l *domain.ROILead) error {
	m.created = true
	m.createdLead = l
	return m.createErr
}

func (m *MockROILeadRepo) ExistsRecent(ctx context.Context, email string, since time.Time) (bool, error) {
	m.existsRecented = true
	return m.existsRecent, m.existsErr
}

func validROIInput() SubmitROILeadInput {
	return SubmitROILeadInput{
		Email:            "owner@example.com",
		MessagesPerMonth: 1000,
		CurrentCost:      4000,
		HourlyRate:       50,
	}
}

func TestSubmitROILead_Success_ReturnsSavingsAndPersists(t *testing.T) {
	repo := &MockROILeadRepo{}
	uc := NewSubmitROILeadUseCase(repo)

	savings, err := uc.Execute(context.Background(), validROIInput())
	assert.NoError(t, err)
	assert.True(t, repo.created)
	// 1000 msg * 0.7 * (3/60) h * 50/h = 1750
	assert.InDelta(t, 1750.0, savings, 0.001)
	assert.NotNil(t, repo.createdLead)
	assert.InDelta(t, 1750.0, repo.createdLead.EstimatedSavings, 0.001)
	assert.NotEmpty(t, repo.createdLead.ID)
}

func TestSubmitROILead_NormalizesEmail(t *testing.T) {
	repo := &MockROILeadRepo{}
	uc := NewSubmitROILeadUseCase(repo)

	in := validROIInput()
	in.Email = "  OWNER@EXAMPLE.COM  "
	_, err := uc.Execute(context.Background(), in)
	assert.NoError(t, err)
	assert.Equal(t, "owner@example.com", repo.createdLead.Email)
}

func TestSubmitROILead_Honeypot_BlocksBot(t *testing.T) {
	repo := &MockROILeadRepo{}
	uc := NewSubmitROILeadUseCase(repo)

	in := validROIInput()
	in.Honeypot = "http://spam.example"
	_, err := uc.Execute(context.Background(), in)
	assert.Equal(t, domain.ErrSpamDetected, err)
	assert.False(t, repo.created, "honeypot submission must not be persisted")
	assert.False(t, repo.existsRecented, "honeypot submission must short-circuit before rate-limit check")
}

func TestSubmitROILead_InvalidEmail(t *testing.T) {
	repo := &MockROILeadRepo{}
	uc := NewSubmitROILeadUseCase(repo)

	in := validROIInput()
	in.Email = "notanemail"
	_, err := uc.Execute(context.Background(), in)
	assert.Equal(t, domain.ErrInvalidEmail, err)
	assert.False(t, repo.created)
}

func TestSubmitROILead_MissingNumericFields(t *testing.T) {
	repo := &MockROILeadRepo{}
	uc := NewSubmitROILeadUseCase(repo)

	cases := []SubmitROILeadInput{
		{Email: "owner@example.com", MessagesPerMonth: 0, CurrentCost: 4000, HourlyRate: 50},
		{Email: "owner@example.com", MessagesPerMonth: 1000, CurrentCost: 0, HourlyRate: 50},
		{Email: "owner@example.com", MessagesPerMonth: 1000, CurrentCost: 4000, HourlyRate: 0},
	}
	for _, in := range cases {
		_, err := uc.Execute(context.Background(), in)
		assert.Equal(t, domain.ErrMissingROIFields, err)
	}
	assert.False(t, repo.created)
}

func TestSubmitROILead_RateLimit_BlocksRepeatWithin24h(t *testing.T) {
	repo := &MockROILeadRepo{existsRecent: true}
	uc := NewSubmitROILeadUseCase(repo)

	_, err := uc.Execute(context.Background(), validROIInput())
	assert.Equal(t, domain.ErrTooManyRequests, err)
	assert.True(t, repo.existsRecented)
	assert.False(t, repo.created, "rate-limited email must not be re-inserted")
}

func TestEstimateROISavings_ZeroOnNonPositiveInputs(t *testing.T) {
	assert.Equal(t, 0.0, EstimateROISavings(0, 50))
	assert.Equal(t, 0.0, EstimateROISavings(1000, 0))
}
