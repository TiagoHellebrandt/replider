package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuthStateNonces_SingleUseAndExpiry(t *testing.T) {
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	n := newOAuthStateNonces()
	n.now = func() time.Time { return now }

	a := n.issue(now.Add(10 * time.Minute))
	assert.True(t, n.consume(a), "first use")
	assert.False(t, n.consume(a), "second use")
	assert.False(t, n.consume("never-issued"))

	b := n.issue(now.Add(10 * time.Minute))
	now = now.Add(11 * time.Minute)
	assert.False(t, n.consume(b), "expired")
}

// A Google mailbox consent state copied out of a redirect cannot be
// presented twice: the first callback consumes it.
func TestConnectGoogleMailbox_AStateIsAcceptedOnlyOnce(t *testing.T) {
	conn := &fakeMailConnector{refreshToken: "r", email: "vendas@clinicaaurora.com"}
	store := newFakeChannelStore()
	uc := newConnectUC(conn, store)

	_, err := uc.AuthURL("acc-1")
	require.NoError(t, err)
	state := conn.lastState

	_, err = uc.HandleCallback(context.Background(), state, "code")
	require.NoError(t, err)

	_, err = uc.HandleCallback(context.Background(), state, "code")
	assert.ErrorIs(t, err, ErrEmailOAuthInvalidState)
	assert.Len(t, store.created, 1)
}

func TestGoogleCalendarIntegration_AStateIsAcceptedOnlyOnce(t *testing.T) {
	repo := &fakeGCalRepo{}
	oauth := &fakeGCalOAuth{}
	uc := NewGoogleCalendarIntegrationUseCase(repo, oauth, "state-secret")

	_, err := uc.AuthURL("acc_123")
	require.NoError(t, err)
	state := oauth.lastState

	_, err = uc.HandleCallback(context.Background(), state, "code-xyz")
	require.NoError(t, err)

	_, err = uc.HandleCallback(context.Background(), state, "code-xyz")
	assert.ErrorIs(t, err, domain.ErrGCalInvalidState)
}
