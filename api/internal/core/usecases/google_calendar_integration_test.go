package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/core/domain"
)

type fakeGCalRepo struct {
	stored  *domain.GoogleCalendarIntegration
	deleted bool
}

func (f *fakeGCalRepo) Get(_ context.Context, accountID string) (*domain.GoogleCalendarIntegration, error) {
	if f.stored != nil && f.stored.AccountID == accountID {
		return f.stored, nil
	}
	return nil, nil
}
func (f *fakeGCalRepo) Save(_ context.Context, integ *domain.GoogleCalendarIntegration) error {
	f.stored = integ
	return nil
}
func (f *fakeGCalRepo) Delete(_ context.Context, _ string) error {
	f.deleted = true
	f.stored = nil
	return nil
}

type fakeGCalOAuth struct {
	lastState string
	calendars []domain.GoogleUserCalendar
}

func (f *fakeGCalOAuth) AuthCodeURL(state string) string {
	f.lastState = state
	return "https://accounts.google.com/o/oauth2/auth?state=" + state
}
func (f *fakeGCalOAuth) ExchangeCode(_ context.Context, code string) (string, string, error) {
	return "refresh-" + code, "user@gmail.com", nil
}
func (f *fakeGCalOAuth) ListCalendars(_ context.Context, _ string) ([]domain.GoogleUserCalendar, error) {
	return f.calendars, nil
}

func TestGCalIntegration_ConnectRoundTrip(t *testing.T) {
	repo := &fakeGCalRepo{}
	oauth := &fakeGCalOAuth{}
	uc := NewGoogleCalendarIntegrationUseCase(repo, oauth, "state-secret")

	_, err := uc.AuthURL("acc_123")
	require.NoError(t, err)
	require.NotEmpty(t, oauth.lastState)

	// The signed state round-trips through the callback and recovers
	// the initiating account without any client-supplied identity.
	accountID, err := uc.HandleCallback(context.Background(), oauth.lastState, "code-xyz")
	require.NoError(t, err)
	assert.Equal(t, "acc_123", accountID)
	require.NotNil(t, repo.stored)
	assert.Equal(t, "refresh-code-xyz", repo.stored.RefreshToken)
	assert.Equal(t, "user@gmail.com", repo.stored.GoogleEmail)

	status, err := uc.Status(context.Background(), "acc_123")
	require.NoError(t, err)
	assert.True(t, status.Configured)
	assert.True(t, status.Connected)
	assert.Equal(t, "user@gmail.com", status.GoogleEmail)

	require.NoError(t, uc.Disconnect(context.Background(), "acc_123"))
	assert.True(t, repo.deleted)
}

func TestGCalIntegration_CallbackRejectsTamperedState(t *testing.T) {
	repo := &fakeGCalRepo{}
	oauth := &fakeGCalOAuth{}
	uc := NewGoogleCalendarIntegrationUseCase(repo, oauth, "state-secret")

	_, err := uc.AuthURL("acc_123")
	require.NoError(t, err)

	// A state signed with a different secret must be refused.
	other := NewGoogleCalendarIntegrationUseCase(repo, oauth, "wrong-secret")
	_, err = other.HandleCallback(context.Background(), oauth.lastState, "code")
	assert.ErrorIs(t, err, domain.ErrGCalInvalidState)

	_, err = uc.HandleCallback(context.Background(), "garbage", "code")
	assert.ErrorIs(t, err, domain.ErrGCalInvalidState)
	assert.Nil(t, repo.stored)
}

func TestGCalIntegration_CallbackRejectsExpiredState(t *testing.T) {
	repo := &fakeGCalRepo{}
	oauth := &fakeGCalOAuth{}
	uc := NewGoogleCalendarIntegrationUseCase(repo, oauth, "state-secret")

	_, err := uc.AuthURL("acc_123")
	require.NoError(t, err)

	uc.now = func() time.Time { return time.Now().Add(stateTTL + time.Minute) }
	_, err = uc.HandleCallback(context.Background(), oauth.lastState, "code")
	assert.ErrorIs(t, err, domain.ErrGCalInvalidState)
}

func TestGCalIntegration_NotConfiguredGatesEverything(t *testing.T) {
	repo := &fakeGCalRepo{}
	uc := NewGoogleCalendarIntegrationUseCase(repo, nil, "state-secret")

	_, err := uc.AuthURL("acc_123")
	assert.ErrorIs(t, err, domain.ErrGCalNotConfigured)

	_, err = uc.HandleCallback(context.Background(), "any", "code")
	assert.ErrorIs(t, err, domain.ErrGCalNotConfigured)

	_, err = uc.ListCalendars(context.Background(), "acc_123")
	assert.ErrorIs(t, err, domain.ErrGCalNotConfigured)

	// Status still answers (the dashboard needs it to hide the button).
	status, err := uc.Status(context.Background(), "acc_123")
	require.NoError(t, err)
	assert.False(t, status.Configured)
	assert.False(t, status.Connected)
}

func TestGCalIntegration_ListCalendarsRequiresConnection(t *testing.T) {
	repo := &fakeGCalRepo{}
	oauth := &fakeGCalOAuth{calendars: []domain.GoogleUserCalendar{{ID: "primary", Summary: "Pessoal", Primary: true}}}
	uc := NewGoogleCalendarIntegrationUseCase(repo, oauth, "state-secret")

	_, err := uc.ListCalendars(context.Background(), "acc_123")
	assert.ErrorIs(t, err, domain.ErrGCalNotConnected)

	repo.stored = &domain.GoogleCalendarIntegration{AccountID: "acc_123", RefreshToken: "rt"}
	cals, err := uc.ListCalendars(context.Background(), "acc_123")
	require.NoError(t, err)
	require.Len(t, cals, 1)
	assert.Equal(t, "primary", cals[0].ID)
}
